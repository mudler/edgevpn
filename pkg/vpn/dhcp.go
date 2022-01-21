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

package vpn

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-log/v2"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/types"
	"github.com/mudler/edgevpn/pkg/utils"

	"github.com/mudler/edgevpn/pkg/blockchain"
)

func DHCP(l log.StandardLogger, announcetime time.Duration) ([]node.Option, []Option) {
	ip := make(chan string, 1)
	return []node.Option{
			node.WithNetworkService(
				func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
					//  whoever wants an IP:
					//  1. Get available nodes. Filter from Machine those that do not have an IP.
					//  2. Get the leader among them. If we are not, we wait
					//  3. If we are the leader, pick an IP and start the VPN with that IP
					var wantedIP string
					for wantedIP == "" {
						time.Sleep(5 * time.Second)

						// This network service is blocking and calls in before VPN, hence it needs to registered before VPN
						nodes := services.AvailableNodes(b)

						currentIPs := map[string]string{}
						ips := []string{}

						for _, t := range b.LastBlock().Storage[protocol.MachinesLedgerKey] {
							var m types.Machine
							t.Unmarshal(&m)
							currentIPs[m.PeerID] = m.Address

							l.Debugf("%s uses %s", m.PeerID, m.Address)
							ips = append(ips, m.Address)
						}

						nodesWithNoIP := []string{}
						for _, nn := range nodes {
							if _, exists := currentIPs[nn]; !exists {
								nodesWithNoIP = append(nodesWithNoIP, nn)
							}
						}

						if len(nodes) <= 1 {
							l.Debug("not enough nodes to determine an IP, sleeping")
							continue
						}

						lead := utils.Leader(nodesWithNoIP)
						l.Debug("Nodes with no ip", nodesWithNoIP)

						if n.Host().ID().String() != lead {
							l.Debug("Not leader, sleeping")
							time.Sleep(announcetime)
							continue
						}

						// We are lead
						l.Debug("picking up between", ips)

						wantedIP = utils.NextIP("10.1.0.1", ips)
					}

					ip <- wantedIP
					return nil
				},
			),
		}, []Option{
			func(cfg *Config) error {
				cfg.InterfaceAddress = fmt.Sprintf("%s/24", <-ip)
				close(ip)
				l.Debug("IP Received", cfg.InterfaceAddress)
				return nil
			},
		}
}
