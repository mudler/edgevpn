package edgevpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/mudler/edgevpn/internal"
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
	c := Config{
		StreamHandlers:           make(map[protocol.ID]StreamHandler),
		LedgerAnnounceTime:       5 * time.Second,
		LedgerSyncronizationTime: 5 * time.Second,
		SealKeyLength:            12,
	}
	c.Apply(p...)

	return &EdgeVPN{
		config:  c,
		doneCh:  make(chan struct{}, 1),
		inputCh: make(chan *hub.Message, 3000),
		seed:    0,
	}
}

func (e *EdgeVPN) ExposeService(ledger *blockchain.Ledger, serviceID, dstaddress string) {
	// 1) Register the ServiceID <-> PeerID Association
	// By announcing periodically our service to the blockchain
	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			key := fmt.Sprintf("service-%s", serviceID)
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(key)
			// If mismatch, update the blockchain
			if !found || existingValue.PeerID != e.host.ID().String() {
				updatedMap := map[string]blockchain.Data{}
				updatedMap[key] = blockchain.Data{PeerID: e.host.ID().String()}
				ledger.Add(updatedMap)
			}
		},
	)

	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	e.config.StreamHandlers[protocol.ID(ServiceProtocol)] = func(stream network.Stream) {
		go func() {
			e.config.Logger.Sugar().Info("Received connection from", stream.Conn().RemotePeer().String())
			// TODO: Gate connection by PeerID: stream.Conn().RemotePeer().String()
			// we need a list of known peers
			e.config.Logger.Sugar().Info("Dialing", dstaddress)

			c, err := net.Dial("tcp", dstaddress)
			if err != nil {
				e.config.Logger.Sugar().Info("Reset", stream.Conn().RemotePeer().String(), err.Error())
				stream.Reset()
				return
			}
			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, c)
			go copyStream(closer, c, stream)
			<-closer

			stream.Close()
			c.Close()

			e.config.Logger.Sugar().Info("Done", stream.Conn().RemotePeer().String())

		}()
	}

}

func copyStream(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}

func (e *EdgeVPN) ConnectToService(ledger *blockchain.Ledger, serviceID string, srcaddr string) error {
	// Open local port for listening
	l, err := net.Listen("tcp", srcaddr)
	if err != nil {
		return err
	}
	fmt.Println("Listening on ", srcaddr)

	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		e.config.Logger.Sugar().Info("New connection from", l.Addr().String())
		// Handle connections in a new goroutine, forwarding to the p2p service
		go func() {
			key := fmt.Sprintf("service-%s", serviceID)
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(key)
			// If mismatch, update the blockchain
			if !found {
				e.config.Logger.Sugar().Info("service not found on blockchain")
				return
			}
			// Decode the Peer
			d, err := peer.Decode(existingValue.PeerID)
			if err != nil {
				e.config.Logger.Sugar().Infof("could not decode peer '%s'", existingValue.PeerID)
				return
			}

			// Open a stream
			stream, err := e.host.NewStream(context.Background(), d, ServiceProtocol)
			if err != nil {
				e.config.Logger.Sugar().Infof("could not open stream '%s'", err.Error())
				return
			}
			e.config.Logger.Sugar().Info("Redirecting", l.Addr().String(), "to", serviceID)

			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, conn)
			go copyStream(closer, conn, stream)
			<-closer

			stream.Close()
			conn.Close()
			e.config.Logger.Sugar().Info("Done handling", l.Addr().String(), "to", serviceID)
		}()
	}
}

// Join the network with the ledger.
// It does the minimal action required to be connected
// without any active packet routing
func (e *EdgeVPN) Join(ledger *blockchain.Ledger) error {
	// Set the handler when we receive messages
	// The ledger needs to read them and update the internal blockchain
	e.config.Handlers = append(e.config.Handlers, ledger.Update)

	e.config.Logger.Sugar().Info("starting edgevpn")

	// Startup libp2p network
	err := e.startNetwork()
	if err != nil {
		return err
	}

	// Send periodically messages to the channel with our blockchain content
	ledger.Syncronizer(context.Background(), e.config.LedgerSyncronizationTime)

	return nil
}

func newBlockChainData(e *EdgeVPN, address string) blockchain.Data {
	hostname, _ := os.Hostname()

	return blockchain.Data{
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

	mw, err := e.MessageWriter()
	if err != nil {
		return err
	}

	ledger := blockchain.New(mw, e.config.MaxBlockChainLength)

	// Set the stream handler to get back the packets from the stream to the interface
	e.config.StreamHandlers[protocol.ID(Protocol)] = streamHandler(ledger, ifce)

	// Join the node to the network, using our ledger
	// it also starts up a goroutine that periodically sends
	// messages to the network with our blockchain content
	if err := e.Join(ledger); err != nil {
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
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(ip.String())
			// If mismatch, update the blockchain
			if !found || existingValue.PeerID != e.host.ID().String() {
				updatedMap := map[string]blockchain.Data{}
				updatedMap[ip.String()] = newBlockChainData(e, ip.String())
				ledger.Add(updatedMap)
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

func streamHandler(ledger *blockchain.Ledger, ifce *water.Interface) func(stream network.Stream) {
	return func(stream network.Stream) {
		if !ledger.Exists(
			func(d blockchain.Data) bool {
				return d.PeerID == stream.Conn().RemotePeer().String()
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
		d, err := peer.Decode(value.PeerID)
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

func (e *EdgeVPN) startNetwork() error {
	ctx := context.Background()
	e.config.Logger.Sugar().Info("generating host data")

	host, err := e.genHost(ctx)
	if err != nil {
		e.config.Logger.Sugar().Error(err.Error())
		return err
	}
	e.host = host

	for pid, strh := range e.config.StreamHandlers {
		host.SetStreamHandler(pid, network.StreamHandler(strh))
	}

	e.config.Logger.Sugar().Info("Host created. We are:", host.ID())
	e.config.Logger.Sugar().Info(host.Addrs())

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
			e.config.Logger.Sugar().Fatal(err)
		}
	}

	e.config.Logger.Sugar().Info("starting event handler")
	go e.handleEvents(ctx)
	e.config.Logger.Sugar().Info("started event handler successfully")

	return nil
}
