// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/utils"

	"github.com/mudler/edgevpn/pkg/blockchain"
)

func AliveNetworkService(announcetime, scrubTime, maxtime time.Duration) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
		t := time.Now()
		// By announcing periodically our service to the blockchain
		b.Announce(
			ctx,
			announcetime,
			func() {
				// Keep-alive
				b.Add(protocol.HealthCheckKey, map[string]interface{}{
					n.Host().ID().String(): time.Now().Format(time.RFC3339),
				})

				// Keep-alive scrub
				nodes := AvailableNodes(b, maxtime)
				if len(nodes) == 0 {
					return
				}
				lead := utils.Leader(nodes)
				if !t.Add(scrubTime).After(time.Now()) {
					// Update timer so not-leader do not attempt to delete bucket afterwards
					// prevent cycles
					t = time.Now()

					if lead == n.Host().ID().String() {
						// Automatically scrub after some time passed
						b.DeleteBucket(protocol.HealthCheckKey)
					}
				}
			},
		)
		return nil
	}
}

// Alive announce the node every announce time, with a periodic scrub time for healthchecks
// the maxtime is the time used to determine when a node is unreachable (after maxtime, its unreachable)
func Alive(announcetime, scrubTime, maxtime time.Duration) []node.Option {
	return []node.Option{
		node.WithNetworkService(AliveNetworkService(announcetime, scrubTime, maxtime)),
	}
}

// AvailableNodes returns the available nodes which sent a healthcheck in the last maxTime
func AvailableNodes(b *blockchain.Ledger, maxTime time.Duration) (active []string) {
	for u, t := range b.LastBlock().Storage[protocol.HealthCheckKey] {
		var s string
		t.Unmarshal(&s)
		parsed, _ := time.Parse(time.RFC3339, s)
		if parsed.Add(maxTime).After(time.Now()) {
			active = append(active, u)
		}
	}

	return active
}
