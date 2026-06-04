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

package blockchain

import (
	"time"

	"github.com/mudler/edgevpn/pkg/protocol"
)

// Reap bounds ledger growth from inactive nodes. For every Liveness-managed
// bucket it signs a tombstone over each entry whose owner is no longer live,
// and it physically prunes tombstones older than tombstoneTTL. It is a no-op
// unless ownership is enabled and a signer is set; callers should run it only
// on the elected leader to avoid redundant writes.
//
// See docs/design/authenticated-ledger.md (section 9).
func (l *Ledger) Reap(tombstoneTTL time.Duration) {
	l.Lock()
	if l.mode == OwnershipOff || l.signer == nil {
		l.Unlock()
		return
	}
	cur := copyStorage(l.blockchain.Last().Storage)
	registry := l.registry
	now := l.clock()
	l.Unlock()

	health := projectValues(cur[protocol.HealthCheckKey])
	changed := false

	for bucket, kv := range cur {
		pol := registry.Policy(bucket)
		for key, e := range kv {
			// Prune tombstones that everyone has had time to observe.
			if e.Deleted {
				if time.Unix(e.UpdatedAt, 0).Add(tombstoneTTL).Before(now) {
					delete(cur[bucket], key)
					changed = true
				}
				continue
			}

			if !pol.Owned || pol.Expiry == NoExpiry {
				continue
			}
			// Tombstone any entry past its lease (inactive Liveness owner, or
			// Absolute-expired such as a stale heartbeat).
			if l.expired(bucket, key, e, pol, health, now) {
				cur[bucket][key] = l.makeTombstone(bucket, key, e, now)
				changed = true
			}
		}
	}

	if changed {
		l.writeData(cur)
	}
}

// LivenessBuckets returns the buckets whose entries expire with owner liveness.
func (l *Ledger) LivenessBuckets() []string {
	l.Lock()
	defer l.Unlock()
	var out []string
	for name, pol := range l.registry {
		if pol.Expiry == Liveness {
			out = append(out, name)
		}
	}
	return out
}

// OwnershipEnabled reports whether the ledger is running the authenticated
// merge (observe or enforce).
func (l *Ledger) OwnershipEnabled() bool {
	l.Lock()
	defer l.Unlock()
	return l.mode != OwnershipOff
}

// IsOwnerLive reports whether owner currently has a fresh heartbeat. When
// ownership is disabled it always returns true, so existing readers behave
// exactly as before; under observe/enforce it lets readers skip entries whose
// owner has gone inactive without waiting for the reaper to tombstone them.
func (l *Ledger) IsOwnerLive(owner string) bool {
	l.Lock()
	if l.mode == OwnershipOff {
		l.Unlock()
		return true
	}
	health := projectValues(l.blockchain.Last().Storage[protocol.HealthCheckKey])
	ttl := l.ttl
	now := l.clock()
	l.Unlock()
	return IsLive(health, owner, ttl, now)
}
