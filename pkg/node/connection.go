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
	"crypto/rand"
	"io"
	mrand "math/rand"
	"net"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	conngater "github.com/libp2p/go-libp2p/p2p/net/conngater"
	hub "github.com/mudler/edgevpn/pkg/hub"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/xlzd/gotp"
)

// Host returns the libp2p peer host
func (e *Node) Host() host.Host {
	return e.host
}

func (e *Node) genHost(ctx context.Context) (host.Host, error) {
	var r io.Reader
	if e.seed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(e.seed))
	}

	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 4096, r)
	if err != nil {
		return nil, err
	}

	opts := e.config.Options

	if e.config.InterfaceAddress != "" {
		// Avoid to loopback traffic by trying to connect to nodes in via VPN
		_, vpnNetwork, err := net.ParseCIDR(e.config.InterfaceAddress)
		if err != nil {
			return nil, err
		}
		cg, err := conngater.NewBasicConnectionGater(nil)
		if err != nil {
			return nil, err
		}
		if err := cg.BlockSubnet(vpnNetwork); err != nil {
			return nil, err
		}
		opts = append(opts, libp2p.ConnectionGater(cg))
	}

	opts = append(opts, libp2p.Identity(prvKey))

	addrs := []multiaddr.Multiaddr{}
	for _, l := range e.config.ListenAddresses {
		addrs = append(addrs, []multiaddr.Multiaddr(l)...)
	}
	opts = append(opts, libp2p.ListenAddrs(addrs...))

	for _, d := range e.config.ServiceDiscovery {
		opts = append(opts, d.Option(ctx))
	}

	opts = append(opts, e.config.AdditionalOptions...)

	if e.config.Insecure {
		e.config.Logger.Info("Disabling Security transport layer")
		opts = append(opts, libp2p.NoSecurity)
	}

	opts = append(opts, libp2p.FallbackDefaults)

	return libp2p.New(opts...)
}

func (e *Node) sealkey() string {
	return gotp.NewTOTP(e.config.ExchangeKey, e.config.SealKeyLength, e.config.SealKeyInterval, nil).Now()
}

func (e *Node) handleEvents(ctx context.Context) {
	for {
		select {
		case m := <-e.inputCh:
			c := m.Copy()
			if err := c.Seal(e.sealkey()); err != nil {
				e.config.Logger.Warn(err.Error())
			}
			e.handleOutgoingMessage(c)
		case m := <-e.HubRoom.Messages:
			c := m.Copy()
			if err := c.Unseal(e.sealkey()); err != nil {
				e.config.Logger.Warn(err.Error())
			}
			e.handleReceivedMessage(c)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Node) handleReceivedMessage(m *hub.Message) {
	for _, h := range e.config.Handlers {
		if err := h(m); err != nil {
			e.config.Logger.Warnf("handler error: %s", err)
		}
	}
}

func (e *Node) handleOutgoingMessage(m *hub.Message) {
	err := e.HubRoom.PublishMessage(m)
	if err != nil {
		e.config.Logger.Warnf("publish error: %s", err)
	}
}
