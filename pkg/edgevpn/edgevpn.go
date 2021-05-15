package edgevpn

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	hub "github.com/mudler/edgevpn/pkg/hub"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"go.uber.org/zap"
)

type EdgeVPN struct {
	config  Config
	HubRoom *hub.Room
	doneCh  chan struct{}
	inputCh chan *hub.Message
	seed    int64
	nick    string
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

func (e *EdgeVPN) Start() error {
	ifce, err := e.createInterface()
	if err != nil {
		return err
	}
	defer ifce.Close()

	// Set the handler when we receive packages
	e.config.Handlers = append(e.config.Handlers, IfaceWriter(ifce))

	e.config.Logger.Sugar().Info("starting edgevpn background daemon")

	// Startup libp2p network
	err = e.network()
	if err != nil {
		return err
	}

	// Write packets from interface
	return e.writePackets(ifce)
}

func (e *EdgeVPN) writePackets(ifce *water.Interface) error {

	if err := e.prepareInterface(); err != nil {
		return err
	}

	mw, err := e.MessageWriter()
	if err != nil {
		return err
	}
	var frame ethernet.Frame

	for {
		frame.Resize(e.config.MTU)
		n, err := ifce.Read([]byte(frame))
		if err != nil {
			return err
		}
		frame = frame[:n]
		mw.Write(frame)
		e.config.Logger.Debug("packet",
			zap.String("dst", frame.Destination().String()),
			zap.String("Src", frame.Source().String()),
			zap.String("Ethertype", fmt.Sprint(frame.Ethertype())),
			zap.String("Payload", fmt.Sprint(frame.Payload())),
			zap.String("dst", frame.Destination().String()),
		)
	}
}

func (e *EdgeVPN) MessageWriter(opts ...hub.MessageOption) (*MessageWriter, error) {
	mess := &hub.Message{}
	mess.Apply(opts...)

	return &MessageWriter{
		c:     e.config,
		input: e.inputCh,
		mess:  mess,
	}, nil
}

func (e *EdgeVPN) network() error {

	ctx := context.Background()
	e.config.Logger.Sugar().Info("generating host data")

	host, err := e.genHost(ctx)
	if err != nil {
		e.config.Logger.Sugar().Error(err.Error())

		return err
	}
	e.host = host

	e.config.Logger.Sugar().Info("Host created. We are:", host.ID())
	e.config.Logger.Sugar().Info(host.Addrs())

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	//host.SetStreamHandler(protocol.ID(e.config.ProtocolID), w.handleStream)

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

// end signals the event loop to exit gracefully
func (e *EdgeVPN) Stop() {
	e.doneCh <- struct{}{}
}
