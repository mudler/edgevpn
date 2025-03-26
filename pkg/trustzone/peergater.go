/*
Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trustzone

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
)

type PeerGater struct {
	sync.Mutex
	trustDB          []peer.ID
	enabled, relaxed bool
}

// NewPeerGater returns a new peergater
// In relaxed mode won't gate until the trustDB contains some auth data.
func NewPeerGater(relaxed bool) *PeerGater {
	return &PeerGater{enabled: true, relaxed: relaxed}
}

// Enabled returns true if the PeerGater is enabled
func (pg *PeerGater) Enabled() bool {
	pg.Lock()
	defer pg.Unlock()
	return pg.enabled
}

// Disable turn off the peer gating mechanism
func (pg *PeerGater) Disable() {
	pg.Lock()
	defer pg.Unlock()
	pg.enabled = false
}

// Enable turns on peer gating mechanism
func (pg *PeerGater) Enable() {
	pg.Lock()
	defer pg.Unlock()
	pg.enabled = true
}

// Implements peergating interface
// resolves to peers in the trustDB. if peer is absent will return true
func (pg *PeerGater) Gate(n *node.Node, p peer.ID) bool {
	pg.Lock()
	defer pg.Unlock()
	if !pg.enabled {
		return false
	}

	if pg.relaxed && len(pg.trustDB) == 0 {
		return false
	}

	for _, pp := range pg.trustDB {
		if pp == p {
			return false
		}
	}

	return true
}

// UpdaterService is a service responsible to sync back trustDB from the ledger state.
// It is a network service which retrieves the senders ID listed in the Trusted Zone
// and fills it in the trustDB used to gate blockchain messages
func (pg *PeerGater) UpdaterService(duration time.Duration) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
		b.Announce(ctx, duration, func() {
			db := []peer.ID{}
			tz, found := b.CurrentData()[protocol.TrustZoneKey]
			if found {
				for k, _ := range tz {
					db = append(db, peer.ID(k))
				}
			}
			pg.Lock()
			pg.trustDB = db
			pg.Unlock()
		})

		return nil
	}
}
