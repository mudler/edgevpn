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

package trustzone

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
)

// PeerGuardian provides auth for peers from blockchain data
type PeerGuardian struct {
	authProviders []AuthProvider
	logger        log.StandardLogger
}

func NewPeerGuardian(logger log.StandardLogger, authProviders ...AuthProvider) *PeerGuardian {
	return &PeerGuardian{
		authProviders: authProviders,
		logger:        logger,
	}
}

// ReceiveMessage is a GenericHandler for public channel to provide authentication.
// We receive messages here and we select them based on 2 criterias:
// - messages that are supposed to generate challenges for auth mechanisms.
//   Auth mechanisms should get user auth data from a special TZ dedicated to hashes that are manually added
// - messages that are answers to such challenges and then means that the sender.ID should be added to the trust zone
func (pg *PeerGuardian) ReceiveMessage(l *blockchain.Ledger, m *hub.Message, c chan *hub.Message) error {
	pg.logger.Debug("Peerguardian received message from", m.SenderID)

	for _, a := range pg.authProviders {

		_, exists := l.GetKey(protocol.TrustZoneKey, m.SenderID)
		trustAuth := l.CurrentData()[protocol.TrustZoneAuthKey]
		if !exists && a.Authenticate(m, c, trustAuth) {
			// try to authenticate it
			// Note we can also not be in a TZ here as we are not able to check (we miss node information at hand)
			// In any way nodes would ignore the messages, and that we hit Authenticate is useful for two (or more)
			// steps authenticators.
			l.Persist(context.Background(), 5*time.Second, 120*time.Second, protocol.TrustZoneKey, m.SenderID, "")
			return nil
		}
	}

	return nil
}

// Challenger is a NetworkService that should send challenges with all enabled authenticators until we are in TZ
// note that might never happen as node might not have a satisfying authentication mechanism
func (pg *PeerGuardian) Challenger(duration time.Duration, autocleanup bool) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
		b.Announce(ctx, duration, func() {
			trustAuth := b.CurrentData()[protocol.TrustZoneAuthKey]
			_, exists := b.GetKey(protocol.TrustZoneKey, n.Host().ID().String())
			for _, a := range pg.authProviders {
				a.Challenger(exists, c, n, b, trustAuth)
			}

			// Automatically cleanup TZ from peers not anymore in the hub
			if autocleanup {
				peers, err := n.MessageHub.ListPeers()
				if err != nil {
					return
				}
				tz := b.CurrentData()[protocol.TrustZoneKey]

				for k, _ := range tz {
				PEER:
					for _, p := range peers {
						if p.String() == k {
							break PEER
						}
					}
					b.Delete(protocol.TrustZoneKey, k)
				}
			}
		})
		return nil
	}
}

// AuthProvider is a generic Blockchain authentity provider
type AuthProvider interface {
	// Authenticate either generates challanges to pick up later or authenticates a node
	// from a message with the available auth data in the blockchain
	Authenticate(*hub.Message, chan *hub.Message, map[string]blockchain.Data) bool
	Challenger(inTrustZone bool, c node.Config, n *node.Node, b *blockchain.Ledger, trustData map[string]blockchain.Data)
}
