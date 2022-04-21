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
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/crypto"
	"github.com/xlzd/gotp"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
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

	ctxCancel                context.CancelFunc
	Messages, PublicMessages chan *Message
}

// roomBufSize is the number of incoming messages to buffer for each topic.
const roomBufSize = 128

func NewHub(otp string, maxsize, keyLength, interval int, joinPublic bool) *MessageHub {
	return &MessageHub{otpKey: otp, maxsize: maxsize, keyLength: keyLength, interval: interval,
		Messages: make(chan *Message, roomBufSize), PublicMessages: make(chan *Message, roomBufSize), joinPublic: joinPublic}
}

func (m *MessageHub) topicKey(salts ...string) string {
	totp := gotp.NewTOTP(strings.ToUpper(m.otpKey), m.keyLength, m.interval, nil)
	if len(salts) > 0 {
		return crypto.MD5(totp.Now() + strings.Join(salts, ":"))
	}
	return crypto.MD5(totp.Now())
}

func (m *MessageHub) joinRoom(host host.Host) error {
	m.Lock()
	defer m.Unlock()

	if m.ctxCancel != nil {
		m.ctxCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.ctxCancel = cancel

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, host, pubsub.WithMaxMessageSize(m.maxsize))
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
