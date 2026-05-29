// Copyright © 2022 Ettore Di Giacinto <mudler@c3os.io>
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

package hub

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/crypto"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type MessageHub struct {
	sync.Mutex

	blockchain, public *room
	ps                 *pubsub.PubSub
	otpKey             string
	maxsize            int
	keyLength          int
	interval           int
	joinPublic         bool
	directPeers        []peer.AddrInfo

	ctxCancel                context.CancelFunc
	Messages, PublicMessages chan *Message
}

// roomBufSize is the number of incoming messages to buffer for each topic.
const roomBufSize = 128

// Option configures a MessageHub at construction time.
type Option func(*MessageHub)

// WithDirectPeers pins peers as gossipsub direct peers — the router holds a
// persistent connection to each and bypasses mesh negotiation for them, which
// makes pubsub delivery reliable to known/bootstrap peers even when the mesh
// (e.g. on a 2-node cluster) hasn't reached its target size.
func WithDirectPeers(peers []peer.AddrInfo) Option {
	return func(m *MessageHub) { m.directPeers = peers }
}

func NewHub(otp string, maxsize, keyLength, interval int, joinPublic bool, opts ...Option) *MessageHub {
	m := &MessageHub{otpKey: otp, maxsize: maxsize, keyLength: keyLength, interval: interval,
		Messages: make(chan *Message, roomBufSize), PublicMessages: make(chan *Message, roomBufSize), joinPublic: joinPublic}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *MessageHub) topicKey(salts ...string) string {
	totp := crypto.TOTP(sha256.New, m.keyLength, m.interval, m.otpKey)
	if len(salts) > 0 {
		return crypto.MD5(totp + strings.Join(salts, ":"))
	}
	return crypto.MD5(totp)
}

func (m *MessageHub) joinRoom(host host.Host) error {
	m.Lock()
	defer m.Unlock()

	if m.ctxCancel != nil {
		m.ctxCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.ctxCancel = cancel

	// create a new PubSub service using the GossipSub router.
	//
	// FloodPublish makes the publisher flood messages to ALL connected peers
	// rather than only to its mesh — important for small clusters (2-3 nodes)
	// where the gossipsub mesh sits below its low-watermark and standard
	// mesh-only delivery becomes unreliable / asymmetric.
	//
	// PeerExchange lets gossipsub peers gossip about each other, which helps
	// recover from one-way mesh links and from peer churn.
	//
	// DirectPeers (when set, typically from bootstrap peers) pins specific
	// peers as always-connected, mesh-bypass delivery targets — guaranteeing
	// publication reaches them.
	psOpts := []pubsub.Option{
		pubsub.WithMaxMessageSize(m.maxsize),
		pubsub.WithFloodPublish(true),
		pubsub.WithPeerExchange(true),
	}
	if len(m.directPeers) > 0 {
		psOpts = append(psOpts, pubsub.WithDirectPeers(m.directPeers))
	}
	ps, err := pubsub.NewGossipSub(ctx, host, psOpts...)
	if err != nil {
		return err
	}

	// join the "chat" room
	cr, err := connect(ctx, ps, host.ID(), m.topicKey(), m.Messages)
	if err != nil {
		return err
	}

	m.blockchain = cr

	if m.joinPublic {
		cr2, err := connect(ctx, ps, host.ID(), m.topicKey("public"), m.PublicMessages)
		if err != nil {
			return err
		}
		m.public = cr2
	}

	m.ps = ps

	return nil
}

func (m *MessageHub) Start(ctx context.Context, host host.Host) error {
	c := make(chan interface{})
	go func(c context.Context, cc chan interface{}) {
		k := ""
		for {
			select {
			default:
				currentKey := m.topicKey()
				if currentKey != k {
					k = currentKey
					cc <- nil
				}
				time.Sleep(1 * time.Second)
			case <-ctx.Done():
				close(cc)
				return
			}
		}
	}(ctx, c)

	for range c {
		m.joinRoom(host)
	}

	// Close eventual open contexts
	if m.ctxCancel != nil {
		m.ctxCancel()
	}
	return nil
}

func (m *MessageHub) PublishMessage(mess *Message) error {
	m.Lock()
	defer m.Unlock()
	if m.blockchain != nil {
		return m.blockchain.publishMessage(mess)
	}
	return errors.New("no message room available")
}

func (m *MessageHub) PublishPublicMessage(mess *Message) error {
	m.Lock()
	defer m.Unlock()
	if m.public != nil {
		return m.public.publishMessage(mess)
	}
	return errors.New("no message room available")
}

func (m *MessageHub) ListPeers() ([]peer.ID, error) {
	m.Lock()
	defer m.Unlock()
	if m.blockchain != nil {
		return m.blockchain.Topic.ListPeers(), nil
	}
	return nil, errors.New("no message room available")
}
