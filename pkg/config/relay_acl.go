/*
Copyright © 2021-2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package config

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
)

// NetworkOnlyACL is a relayv2.ACLFilter that gates incoming circuit-v2
// relay reservations on cluster membership. A peer is a "cluster
// member" when its libp2p peer ID is present in the local ledger's
// alive bucket (i.e. it has successfully gossiped a healthcheck
// timestamp against our network token).
//
// During a short bootstrap window — before the ACL has been told
// "you can start strict-mode now" — every reservation is allowed.
// Without this window a new peer joining a network where we are
// reachable would be unable to reserve a slot on us before having
// proved itself, even though it can't prove itself until it has
// joined gossipsub, which on many NAT'd setups requires going
// through a relay first.
//
// The zero value is usable: AllowReserve returns true (open) until
// the first call to Members(...) flips the gate via Start.
type NetworkOnlyACL struct {
	members atomic.Pointer[map[peer.ID]struct{}]
	started atomic.Bool
}

// AllowReserve implements relayv2.ACLFilter. Returns true if the
// peer is a recognised cluster member, or if strict-mode hasn't been
// entered yet (bootstrap window).
func (a *NetworkOnlyACL) AllowReserve(p peer.ID, _ ma.Multiaddr) bool {
	if !a.started.Load() {
		return true
	}
	m := a.members.Load()
	if m == nil {
		return true
	}
	_, ok := (*m)[p]
	return ok
}

// AllowConnect implements relayv2.ACLFilter. We only gate the
// reservation step — once a peer has a reservation it is by
// definition a cluster member, so any connect through it stays
// permitted. This keeps in-flight relayed sessions stable even if
// the alive bucket flickers.
func (a *NetworkOnlyACL) AllowConnect(_ peer.ID, _ ma.Multiaddr, _ peer.ID) bool {
	return true
}

// Members replaces the authorised peer set. Callers can drop or add
// peers atomically; readers see a consistent snapshot. Calling
// Members for the first time also flips the gate out of bootstrap
// mode: subsequent AllowReserve calls enforce the set strictly.
func (a *NetworkOnlyACL) Members(set map[peer.ID]struct{}) {
	// Defensive copy so callers can keep mutating their map without
	// racing with the goroutines reading via AllowReserve.
	cp := make(map[peer.ID]struct{}, len(set))
	for k, v := range set {
		cp[k] = v
	}
	a.members.Store(&cp)
	a.started.Store(true)
}

// NetworkOnlyACLService is a node.NetworkService that periodically
// snapshots the ledger's alive bucket into the supplied ACL.
//
// The first non-empty snapshot ends the bootstrap window. If the
// bucket is empty (e.g. the alive service is disabled), the ACL
// stays in open-mode and a debug log line is emitted on each tick
// so operators can spot the misconfiguration.
//
// The refresh cadence should be ≤ the alive-service announce
// interval; once per 30 s is a reasonable default.
func NetworkOnlyACLService(acl *NetworkOnlyACL, refresh time.Duration) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
		if acl == nil {
			return nil
		}
		// Refresh once immediately so the very first reservation after
		// the ledger is up does not have to wait a full tick.
		refreshACL(c.Logger, acl, b)

		go func() {
			t := time.NewTicker(refresh)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					refreshACL(c.Logger, acl, b)
				}
			}
		}()
		return nil
	}
}

func refreshACL(logger log.StandardLogger, acl *NetworkOnlyACL, b *blockchain.Ledger) {
	bucket := b.LastBlock().Storage[protocol.HealthCheckKey]
	if len(bucket) == 0 {
		// Ledger has no alive entries yet — could mean alive-service is
		// disabled, or we are very early in startup. Stay in open-mode
		// until something appears.
		if logger != nil {
			logger.Debugf("relay-service NetworkOnly ACL: alive bucket empty, keeping ACL open until next refresh")
		}
		return
	}
	set := make(map[peer.ID]struct{}, len(bucket))
	for uuid := range bucket {
		pid, err := peer.Decode(uuid)
		if err != nil {
			continue
		}
		set[pid] = struct{}{}
	}
	acl.Members(set)
}
