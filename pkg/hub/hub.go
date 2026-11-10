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

	ctxCancel, psCancel      context.CancelFunc
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

	topic := m.topicKey()
	if m.blockchain != nil && m.blockchain.roomName == topic {
		return nil
	}

	// One pubsub instance serves the hub for as long as it runs, and a key
	// change only moves it to new topics. A new instance per key would reset
	// the pubsub streams on every live connection, and two peers never switch
	// at the same instant: a stream opened by the peer that switched first
	// lands on the other peer's old instance, which drops what it reads and
	// later resets it. Gossipsub then reopens the stream but forgets the
	// peer's subscriptions, which the peer announced once and does not repeat,
	// so it stops sending to that peer until the next key change.
	if m.ps == nil {
		// FloodPublish makes the publisher flood messages to ALL connected peers
		// rather than only to its mesh. This matters for small clusters (2-3
		// nodes), where the gossipsub mesh sits below its low-watermark and
		// mesh-only delivery becomes unreliable or asymmetric.
		//
		// PeerExchange lets gossipsub peers gossip about each other, which helps
		// recover from one-way mesh links and from peer churn.
		//
		// DirectPeers (when set, typically from bootstrap peers) pins specific
		// peers as always-connected, mesh-bypass delivery targets, so
		// publication is guaranteed to reach them.
		psOpts := []pubsub.Option{
			pubsub.WithMaxMessageSize(m.maxsize),
			pubsub.WithFloodPublish(true),
			pubsub.WithPeerExchange(true),
		}
		if len(m.directPeers) > 0 {
			psOpts = append(psOpts, pubsub.WithDirectPeers(m.directPeers))
		}
		ctx, cancel := context.WithCancel(context.Background())
		ps, err := pubsub.NewGossipSub(ctx, host, psOpts...)
		if err != nil {
			cancel()
			return err
		}
		m.ps, m.psCancel = ps, cancel
	}

	// The new topics are joined before the old ones are left, so the hub
	// always has a room to publish to.
	ctx, cancel := context.WithCancel(context.Background())
	cr, err := connect(ctx, m.ps, host.ID(), topic, m.Messages)
	if err != nil {
		cancel()
		return err
	}
	var public *room
	if m.joinPublic {
		public, err = connect(ctx, m.ps, host.ID(), m.topicKey("public"), m.PublicMessages)
		if err != nil {
			cr.leave()
			cancel()
			return err
		}
	}

	for _, old := range []*room{m.blockchain, m.public} {
		if old != nil {
			old.leave()
		}
	}
	if m.ctxCancel != nil {
		m.ctxCancel()
	}
	m.blockchain, m.public, m.ctxCancel = cr, public, cancel

	return nil
}

// Start keeps the hub in the room of the current key until ctx is done. It
// checks once a second, so a key change, or a join that failed, is picked up
// within a second.
func (m *MessageHub) Start(ctx context.Context, host host.Host) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		m.joinRoom(host)
		select {
		case <-tick.C:
		case <-ctx.Done():
			m.Lock()
			if m.ctxCancel != nil {
				m.ctxCancel()
			}
			if m.psCancel != nil {
				m.psCancel()
			}
			m.ps, m.psCancel, m.ctxCancel = nil, nil, nil
			m.blockchain, m.public = nil, nil
			m.Unlock()
			return nil
		}
	}
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
