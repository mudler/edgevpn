package edgevpn

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/mudler/edgevpn/pkg/blockchain"
	hub "github.com/mudler/edgevpn/pkg/hub"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"golang.org/x/net/ipv4"
)

type EdgeVPN struct {
	config  Config
	HubRoom *hub.Room
	doneCh  chan struct{}
	inputCh chan *hub.Message
	seed    int64
	host    host.Host
}

func New(p ...Option) *EdgeVPN {
	c := Config{}
	c.Apply(p...)

	return &EdgeVPN{
		config:  c,
		doneCh:  make(chan struct{}, 1),
		inputCh: make(chan *hub.Message, 3000),
		seed:    0,
	}
}

// keeps syncronized the blockchain with the node IP
func (e *EdgeVPN) adverizer(ip net.IP, ledger *blockchain.Ledger) {
	for {
		time.Sleep(5 * time.Second)

		nodeID := e.host.ID().String()
		// Retrieve current ID for ip in the blockchain
		existingPeerID, found := ledger.GetKey(ip.String())
		// If mismatch, update the blockchain
		if !found || existingPeerID != nodeID {
			updatedMap := map[string]string{}
			updatedMap[ip.String()] = nodeID
			ledger.Add(updatedMap)
		}
	}
}

// Start the vpn. Returns an error in case of failure
func (e *EdgeVPN) Start() error {
	ifce, err := e.createInterface()
	if err != nil {
		return err
	}
	defer ifce.Close()

	mw, err := e.MessageWriter()
	if err != nil {
		return err
	}

	ledger := blockchain.New(mw, e.config.MaxBlockChainLength)

	// Set the handler when we receive messages
	// The ledger needs to read them and update the internal blockchain
	e.config.Handlers = append(e.config.Handlers, ledger.Update)

	e.config.Logger.Sugar().Info("starting edgevpn background daemon")

	// Startup libp2p network
	err = e.network(ledger, ifce)
	if err != nil {
		return err
	}

	// Avoid to loopback traffic by trying to connect to nodes in via VPN
	ip, _, err := net.ParseCIDR(e.config.InterfaceAddress)
	if err != nil {
		return err
	}

	// Updates the blockchain
	ledger.Syncronizer(context.Background(), 5*time.Second)
	ledger.Persist(
		context.Background(),
		5*time.Second,
		ip.String(),
		func() string { return e.host.ID().String() },
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
	e.doneCh <- struct{}{}
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

func (e *EdgeVPN) streamHandler(ledger *blockchain.Ledger, ifce *water.Interface) func(stream network.Stream) {
	return func(stream network.Stream) {
		if !ledger.ExistsValue(stream.Conn().RemotePeer().String()) {
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
			e.config.Logger.Sugar().Debug("could not read from interface")
			return err
		}
		frame = frame[:n]

		header, err := ipv4.ParseHeader(frame)
		if err != nil {
			e.config.Logger.Sugar().Infof("could not parase ipv4 header from frame")
			continue
		}

		dst := header.Dst.String()

		// Query the routing table
		value, found := ledger.GetKey(dst)
		if !found {
			e.config.Logger.Sugar().Infof("'%s' not found in the routing table", dst)
			continue
		}

		// Decode the Peer
		d, err := peer.Decode(value)
		if err != nil {
			e.config.Logger.Sugar().Infof("could not decode peer '%s'", value)
			continue
		}

		// Open a stream
		stream, err := e.host.NewStream(ctx, d, Protocol)
		if err != nil {
			e.config.Logger.Sugar().Infof("could not open stream '%s'", err.Error())
			continue
		}
		stream.Write(frame)
		stream.Close()
	}
}

func (e *EdgeVPN) network(ledger *blockchain.Ledger, ifce *water.Interface) error {
	ctx := context.Background()
	e.config.Logger.Sugar().Info("generating host data")

	host, err := e.genHost(ctx)
	if err != nil {
		e.config.Logger.Sugar().Error(err.Error())
		return err
	}
	e.host = host

	host.SetStreamHandler(protocol.ID(Protocol), e.streamHandler(ledger, ifce))

	e.config.Logger.Sugar().Info("Host created. We are:", host.ID())
	e.config.Logger.Sugar().Info(host.Addrs())

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, host, pubsub.WithMaxMessageSize(e.config.MaxMessageSize))
	if err != nil {
		return err
	}

	// join the chat room
	cr, err := hub.JoinRoom(ctx, ps, host.ID(), e.config.RoomName)
	if err != nil {
		return err
	}

	e.HubRoom = cr

	for _, sd := range e.config.ServiceDiscovery {
		if err := sd.Run(e.config.Logger, ctx, host); err != nil {
			e.config.Logger.Sugar().Fatal(err)
		}
	}

	e.config.Logger.Sugar().Info("starting event handler")
	go e.handleEvents(ctx)
	e.config.Logger.Sugar().Info("started event handler successfully")

	return nil
}
