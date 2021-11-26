package edgevpn

import (
	"context"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn/types"
	hub "github.com/mudler/edgevpn/pkg/hub"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"golang.org/x/net/ipv4"
)

const MachinesLedgerKey = "machines"

type EdgeVPN struct {
	config  Config
	HubRoom *hub.Room
	doneCh  chan struct{}
	inputCh chan *hub.Message
	seed    int64
	host    host.Host

	ledger *blockchain.Ledger
}

var defaultLibp2pOptions = []libp2p.Option{
	libp2p.EnableNATService(),
	libp2p.NATPortMap(),
	libp2p.EnableAutoRelay(),
}

func New(p ...Option) *EdgeVPN {
	c := Config{
		DiscoveryInterval:        120 * time.Second,
		StreamHandlers:           make(map[protocol.ID]StreamHandler),
		LedgerAnnounceTime:       5 * time.Second,
		LedgerSyncronizationTime: 5 * time.Second,
		SealKeyLength:            12,
		Options:                  defaultLibp2pOptions,
	}
	c.Apply(p...)

	return &EdgeVPN{
		config:  c,
		doneCh:  make(chan struct{}, 1),
		inputCh: make(chan *hub.Message, 3000),
		seed:    0,
	}
}

func (e *EdgeVPN) Ledger() (*blockchain.Ledger, error) {
	if e.ledger != nil {
		return e.ledger, nil
	}
	mw, err := e.MessageWriter()
	if err != nil {
		return nil, err
	}

	e.ledger = blockchain.New(mw, e.config.Store)
	return e.ledger, nil
}

// Join the network with the ledger.
// It does the minimal action required to be connected
// without any active packet routing
func (e *EdgeVPN) Join() error {

	ledger, err := e.Ledger()
	if err != nil {
		return err
	}

	// Set the handler when we receive messages
	// The ledger needs to read them and update the internal blockchain
	e.config.Handlers = append(e.config.Handlers, ledger.Update)

	e.config.Logger.Info("Starting EdgeVPN network")

	// Startup libp2p network
	err = e.startNetwork()
	if err != nil {
		return err
	}

	// Send periodically messages to the channel with our blockchain content
	ledger.Syncronizer(context.Background(), e.config.LedgerSyncronizationTime)

	return nil
}

func newBlockChainData(e *EdgeVPN, address string) types.Machine {
	hostname, _ := os.Hostname()

	return types.Machine{
		PeerID:   e.host.ID().String(),
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  internal.Version,
		Address:  address,
	}
}

// Start the vpn. Returns an error in case of failure
func (e *EdgeVPN) Start() error {
	ifce, err := e.createInterface()
	if err != nil {
		return err
	}
	defer ifce.Close()

	ledger, err := e.Ledger()
	if err != nil {
		return err
	}

	// Set the stream handler to get back the packets from the stream to the interface
	e.config.StreamHandlers[protocol.ID(Protocol)] = streamHandler(ledger, ifce)

	// Join the node to the network, using our ledger
	// it also starts up a goroutine that periodically sends
	// messages to the network with our blockchain content
	if err := e.Join(); err != nil {
		return err
	}

	// Announce our IP
	ip, _, err := net.ParseCIDR(e.config.InterfaceAddress)
	if err != nil {
		return err
	}

	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			machine := &types.Machine{}
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(MachinesLedgerKey, ip.String())
			existingValue.Unmarshal(machine)

			// If mismatch, update the blockchain
			if !found || machine.PeerID != e.host.ID().String() {
				updatedMap := map[string]interface{}{}
				updatedMap[ip.String()] = newBlockChainData(e, ip.String())
				ledger.Add(MachinesLedgerKey, updatedMap)
			}
		},
	)

	if e.config.NetLinkBootstrap {
		if err := e.prepareInterface(); err != nil {
			return err
		}
	}

	// read packets from the interface
	return e.readPackets(ledger, ifce)
}

// end signals the event loop to exit gracefully
func (e *EdgeVPN) Stop() {
	close(e.doneCh)
}

// MessageWriter returns a new MessageWriter bound to the edgevpn instance
// with the given options
func (e *EdgeVPN) MessageWriter(opts ...hub.MessageOption) (*MessageWriter, error) {
	mess := &hub.Message{}
	mess.Apply(opts...)

	return &MessageWriter{
		c:     e.config,
		input: e.inputCh,
		mess:  mess,
	}, nil
}

func streamHandler(ledger *blockchain.Ledger, ifce *water.Interface) func(stream network.Stream) {
	return func(stream network.Stream) {
		if !ledger.Exists(MachinesLedgerKey,
			func(d blockchain.Data) bool {
				machine := &types.Machine{}
				d.Unmarshal(machine)
				return machine.PeerID == stream.Conn().RemotePeer().String()
			}) {
			stream.Reset()
			return
		}
		io.Copy(ifce.ReadWriteCloser, stream)
		stream.Close()
	}
}

// redirects packets from the interface to the node using the routing table in the blockchain
func (e *EdgeVPN) readPackets(ledger *blockchain.Ledger, ifce *water.Interface) error {
	ctx := context.Background()
	for {
		var frame ethernet.Frame
		frame.Resize(e.config.MTU)
		n, err := ifce.Read([]byte(frame))
		if err != nil {
			e.config.Logger.Debug("could not read from interface")
			return err
		}
		frame = frame[:n]

		header, err := ipv4.ParseHeader(frame)
		if err != nil {
			e.config.Logger.Debugf("could not parase ipv4 header from frame")
			continue
		}

		dst := header.Dst.String()

		// Query the routing table
		value, found := ledger.GetKey(MachinesLedgerKey, dst)
		if !found {
			e.config.Logger.Debugf("'%s' not found in the routing table", dst)
			continue
		}
		machine := &types.Machine{}
		value.Unmarshal(machine)

		// Decode the Peer
		d, err := peer.Decode(machine.PeerID)
		if err != nil {
			e.config.Logger.Debugf("could not decode peer '%s'", value)
			continue
		}

		// Open a stream
		stream, err := e.host.NewStream(ctx, d, Protocol)
		if err != nil {
			e.config.Logger.Debugf("could not open stream '%s'", err.Error())
			continue
		}
		stream.Write(frame)
		stream.Close()
	}
}

func (e *EdgeVPN) Logger() log.StandardLogger {
	return e.config.Logger
}

func (e *EdgeVPN) startNetwork() error {
	ctx := context.Background()
	e.config.Logger.Debug("Generating host data")

	host, err := e.genHost(ctx)
	if err != nil {
		e.config.Logger.Error(err.Error())
		return err
	}
	e.host = host

	for pid, strh := range e.config.StreamHandlers {
		host.SetStreamHandler(pid, network.StreamHandler(strh))
	}

	e.config.Logger.Info("Node ID:", host.ID())
	e.config.Logger.Info("Node Addresses:", host.Addrs())

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, host, pubsub.WithMaxMessageSize(e.config.MaxMessageSize))
	if err != nil {
		return err
	}

	// join the "chat" room
	cr, err := hub.JoinRoom(ctx, ps, host.ID(), e.config.RoomName)
	if err != nil {
		return err
	}

	e.HubRoom = cr

	for _, sd := range e.config.ServiceDiscovery {
		if err := sd.Run(e.config.Logger, ctx, host); err != nil {
			e.config.Logger.Fatal(err)
		}
	}

	go e.handleEvents(ctx)

	e.Logger().Debug("Network started")
	return nil
}
