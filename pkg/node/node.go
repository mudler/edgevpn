// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	"github.com/mudler/edgevpn/pkg/crypto"
	protocol "github.com/mudler/edgevpn/pkg/protocol"

	"github.com/mudler/edgevpn/pkg/blockchain"
	hub "github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/logger"
)

type Node struct {
	config     Config
	MessageHub *hub.MessageHub

	//HubRoom *hub.Room
	inputCh      chan *hub.Message
	genericHubCh chan *hub.Message

	seed   int64
	host   host.Host
	cg     *conngater.BasicConnectionGater
	ledger *blockchain.Ledger
	sync.Mutex
}

const defaultChanSize = 3000

var defaultLibp2pOptions = []libp2p.Option{
	libp2p.EnableNATService(),
	libp2p.NATPortMap(),
	libp2p.EnableAutoRelay(),
}

func New(p ...Option) (*Node, error) {
	c := &Config{
		DiscoveryInterval:        5 * time.Minute,
		StreamHandlers:           make(map[protocol.Protocol]StreamHandler),
		LedgerAnnounceTime:       5 * time.Second,
		LedgerSyncronizationTime: 5 * time.Second,
		SealKeyLength:            12,
		Options:                  defaultLibp2pOptions,
		Logger:                   logger.New(log.LevelDebug),
		Sealer:                   &crypto.AESSealer{},
		Store:                    &blockchain.MemoryStore{},
	}

	if err := c.Apply(p...); err != nil {
		return nil, err
	}

	return &Node{
		config:       *c,
		inputCh:      make(chan *hub.Message, defaultChanSize),
		genericHubCh: make(chan *hub.Message, defaultChanSize),
		seed:         0,
	}, nil
}

// Ledger return the ledger which uses the node
// connection to broadcast messages
func (e *Node) Ledger() (*blockchain.Ledger, error) {
	e.Lock()
	defer e.Unlock()
	if e.ledger != nil {
		return e.ledger, nil
	}

	mw, err := e.messageWriter()
	if err != nil {
		return nil, err
	}

	e.ledger = blockchain.New(mw, e.config.Store)
	return e.ledger, nil
}

// PeerGater returns the node peergater
func (e *Node) PeerGater() Gater {
	return e.config.PeerGater
}

// Start joins the node over the p2p network
func (e *Node) Start(ctx context.Context) error {

	ledger, err := e.Ledger()
	if err != nil {
		return err
	}

	// Set the handler when we receive messages
	// The ledger needs to read them and update the internal blockchain
	e.config.Handlers = append(e.config.Handlers, ledger.Update)

	e.config.Logger.Info("Starting EdgeVPN network")

	// Startup libp2p network
	err = e.startNetwork(ctx)
	if err != nil {
		return err
	}

	// Send periodically messages to the channel with our blockchain content
	ledger.Syncronizer(ctx, e.config.LedgerSyncronizationTime)

	// Start eventual declared NetworkServices
	for _, s := range e.config.NetworkServices {
		err := s(ctx, e.config, e, ledger)
		if err != nil {
			return err
		}
	}

	return nil
}

// messageWriter returns a new MessageWriter bound to the edgevpn instance
// with the given options
func (e *Node) messageWriter(opts ...hub.MessageOption) (*messageWriter, error) {
	mess := &hub.Message{}
	mess.Apply(opts...)

	return &messageWriter{
		c:     e.config,
		input: e.inputCh,
		mess:  mess,
	}, nil
}

func (e *Node) startNetwork(ctx context.Context) error {
	e.config.Logger.Debug("Generating host data")

	host, err := e.genHost(ctx)
	if err != nil {
		e.config.Logger.Error(err.Error())
		return err
	}
	e.host = host

	ledger, err := e.Ledger()
	if err != nil {
		return err
	}

	for pid, strh := range e.config.StreamHandlers {
		host.SetStreamHandler(pid.ID(), network.StreamHandler(strh(e, ledger)))
	}

	e.config.Logger.Info("Node ID:", host.ID())
	e.config.Logger.Info("Node Addresses:", host.Addrs())

	// Hub rotates within sealkey interval.
	// this time length should be enough to make room for few block exchanges. This is ideally on minutes (10, 20, etc. )
	// it makes sure that if a bruteforce is attempted over the encrypted messages, the real key is not exposed.
	e.MessageHub = hub.NewHub(e.config.RoomName, e.config.MaxMessageSize, e.config.SealKeyLength, e.config.SealKeyInterval, e.config.GenericHub)

	for _, sd := range e.config.ServiceDiscovery {
		if err := sd.Run(e.config.Logger, ctx, host); err != nil {
			e.config.Logger.Fatal(err)
		}
	}

	go e.handleEvents(ctx, e.inputCh, e.MessageHub.Messages, e.MessageHub.PublishMessage, e.config.Handlers, true)
	go e.MessageHub.Start(ctx, host)

	// If generic hub is enabled one is created separately with a set of generic channel handlers associated with.
	// note peergating is disabled in order to freely exchange messages that can be used for authentication or for other public means.
	if e.config.GenericHub {
		go e.handleEvents(ctx, e.genericHubCh, e.MessageHub.PublicMessages, e.MessageHub.PublishPublicMessage, e.config.GenericChannelHandler, false)
	}

	e.config.Logger.Debug("Network started")
	return nil
}

// PublishMessage publishes a message to the generic channel (if enabled)
// See GenericChannelHandlers(..) to attach handlers to receive messages from this channel.
func (e *Node) PublishMessage(m *hub.Message) error {
	if !e.config.GenericHub {
		return fmt.Errorf("generic hub disabled")
	}

	e.genericHubCh <- m

	return nil
}
