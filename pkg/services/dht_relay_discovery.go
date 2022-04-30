// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package services

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/utils"
)

// AutoRelayFeederService is a service responsible to returning periodically peers to
// scan for relays from a DHT discovery service
func AutoRelayFeederService(ll log.StandardLogger, peerChan chan peer.AddrInfo, dht *discovery.DHT, duration time.Duration) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
		ll.Debug("[relay discovery] Service starts")
		ctx, cancel := context.WithCancel(ctx)
		go func() {
			t := utils.NewBackoffTicker(utils.BackoffMaxInterval(duration))
			defer t.Stop()
			defer cancel()
			for {
				select {
				case <-t.C:
				case <-ctx.Done():
					ll.Debug("[relay discovery] stopped")
					return
				}
				ll.Debug("[relay discovery] Finding relays from closest peer")
				closestPeers, err := dht.GetClosestPeers(ctx, n.Host().ID().String())
				if err != nil {
					ll.Error(err)
					continue
				}
				for _, p := range closestPeers {
					addrs := n.Host().Peerstore().Addrs(p)
					if len(addrs) == 0 {
						continue
					}
					ll.Debugf("[relay discovery] Found close peer '%s'", p.Pretty())
					select {
					case peerChan <- peer.AddrInfo{ID: p, Addrs: addrs}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		return nil
	}
}
