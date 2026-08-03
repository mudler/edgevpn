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
	"encoding/json"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/protocol"
)

// legacyEntry builds an entry in the pre-ownership wire format: a bare value
// with no owner, no version and no signature.
func legacyEntry(value interface{}) SignedData {
	jb, _ := json.Marshal(value)
	return SignedData{Value: Data(jb)}
}

// restartedWithPersistedLedger models a node that ran with --ledger-state under
// off/observe, so its on-disk ledger holds unsigned entries, and has now been
// restarted into enforce: the entries are already in storage before the first
// merge runs.
func restartedWithPersistedLedger(persisted map[string]map[string]SignedData, ttl time.Duration, now time.Time) *Ledger {
	s := &MemoryStore{}
	genesis := Block{}
	genesis = Block{0, now.String(), map[string]map[string]SignedData{}, genesis.Checksum(), ""}
	s.Add(genesis)
	s.Add(genesis.NewBlock(persisted))

	return New(io.Discard, s,
		WithEnforcedOwnership(DefaultRegistry(ttl), ttl),
		WithClock(func() time.Time { return now }),
	)
}

func entryOwner(l *Ledger, bucket, key string) string {
	return l.CurrentStorage()[bucket][key].Owner
}

var _ = Describe("Legacy unsigned entries under enforcement", func() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ip := "10.1.0.1"

	It("lets the peer a persisted unsigned entry names re-claim its own key", func() {
		b := newTestSigner()
		l := restartedWithPersistedLedger(map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: legacyEntry(machine(b.ID(), ip))},
		}, time.Minute, now)

		// B is alive and re-announces, now signed.
		feed(l, heartbeat(b, now))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(b, protocol.MachinesLedgerKey, ip, machine(b.ID(), ip), 10, now)},
		})

		Expect(entryOwner(l, protocol.MachinesLedgerKey, ip)).To(Equal(b.ID()),
			"the rightful owner must be able to sign over its own legacy entry")
	})

	It("still refuses another live peer's attempt to take that key", func() {
		b := newTestSigner()
		c := newTestSigner()
		l := restartedWithPersistedLedger(map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: legacyEntry(machine(b.ID(), ip))},
		}, time.Minute, now)

		// B, whom the legacy entry names, is alive.
		feed(l, heartbeat(b, now))

		// C is alive too, and tries to claim the IP for itself.
		feed(l, heartbeat(c, now))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(c, protocol.MachinesLedgerKey, ip, machine(c.ID(), ip), 10, now)},
		})

		Expect(entryOwner(l, protocol.MachinesLedgerKey, ip)).To(BeEmpty(),
			"a legacy entry naming a live peer must not be claimable by anyone else")
	})

	It("lets another peer claim it once the named peer has gone", func() {
		b := newTestSigner()
		c := newTestSigner()
		l := restartedWithPersistedLedger(map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: legacyEntry(machine(b.ID(), ip))},
		}, time.Minute, now)

		// No heartbeat for B: it is not live, so the slot is reclaimable.
		feed(l, heartbeat(c, now))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(c, protocol.MachinesLedgerKey, ip, machine(c.ID(), ip), 10, now)},
		})

		Expect(entryOwner(l, protocol.MachinesLedgerKey, ip)).To(Equal(c.ID()))
	})

	It("lets a peer re-claim a persisted unsigned entry keyed by its own peer ID", func() {
		b := newTestSigner()
		l := restartedWithPersistedLedger(map[string]map[string]SignedData{
			protocol.EgressService: {b.ID(): legacyEntry("true")},
		}, time.Minute, now)

		feed(l, heartbeat(b, now))
		feed(l, map[string]map[string]SignedData{
			protocol.EgressService: {b.ID(): mkSignedEntry(b, protocol.EgressService, b.ID(), "true", 10, now)},
		})

		Expect(entryOwner(l, protocol.EgressService, b.ID())).To(Equal(b.ID()))
	})

	It("does not let a peer sign over a legacy entry keyed by someone else's peer ID", func() {
		b := newTestSigner()
		c := newTestSigner()
		l := restartedWithPersistedLedger(map[string]map[string]SignedData{
			protocol.EgressService: {b.ID(): legacyEntry("true")},
		}, time.Minute, now)

		feed(l, heartbeat(b, now))
		feed(l, heartbeat(c, now))
		// C signs an entry under B's key. OwnerOf is the key, so C is not the owner.
		feed(l, map[string]map[string]SignedData{
			protocol.EgressService: {b.ID(): mkSignedEntry(c, protocol.EgressService, b.ID(), "true", 10, now)},
		})

		Expect(entryOwner(l, protocol.EgressService, b.ID())).To(BeEmpty())
	})
})

var _ = Describe("Deleting legacy unsigned entries under enforcement", func() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ip := "10.1.0.1"

	persisted := func(owner string) map[string]map[string]SignedData {
		return map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: legacyEntry(machine(owner, ip))},
		}
	}

	// The divergence case: Delete() always succeeds locally, so if the merge
	// refuses the tombstone the owner's node and every other node disagree
	// permanently, with nothing logged on the node that issued the delete.
	It("propagates a rightful owner's deletion of its legacy entry", func() {
		b := newTestSigner()

		// The owner's own node, restarted into enforce over a persisted ledger.
		owner := restartedWithPersistedLedger(persisted(b.ID()), time.Minute, now)
		owner.SetSigner(b)
		owner.Delete(protocol.MachinesLedgerKey, ip)

		_, found := owner.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse(), "Delete always takes effect locally")

		tombstone := owner.CurrentStorage()[protocol.MachinesLedgerKey][ip]
		Expect(tombstone.Deleted).To(BeTrue())
		Expect(tombstone.Owner).To(Equal(b.ID()))

		// Any other node holding the same persisted entry, with B alive — so the
		// tombstone cannot be accepted merely because B's lease has lapsed.
		peer := restartedWithPersistedLedger(persisted(b.ID()), time.Minute, now)
		feed(peer, heartbeat(b, now))
		feed(peer, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: tombstone},
		})

		_, found = peer.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse(),
			"the deletion must propagate, or the owner's node and its peers diverge for good")
	})

	It("still refuses a tombstone from another live peer", func() {
		b := newTestSigner()
		c := newTestSigner()

		l := restartedWithPersistedLedger(persisted(b.ID()), time.Minute, now)
		feed(l, heartbeat(b, now))
		feed(l, heartbeat(c, now))

		tombstone := SignedData{Owner: c.ID(), Version: 1 << 40, UpdatedAt: now.Unix(), Deleted: true}
		tombstone.Sig, _ = c.Sign(canonical(protocol.MachinesLedgerKey, ip, tombstone))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: tombstone},
		})

		_, found := l.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeTrue(),
			"only the peer the entry names may delete it while that peer is live")
	})

	// The reaper's cross-owner tombstone must keep working: once the named peer
	// is gone, anyone may tombstone the entry.
	It("still allows a tombstone once the named peer has gone", func() {
		b := newTestSigner()
		c := newTestSigner()

		l := restartedWithPersistedLedger(persisted(b.ID()), time.Minute, now)
		feed(l, heartbeat(c, now)) // no heartbeat for B

		tombstone := SignedData{Owner: c.ID(), Version: 1 << 40, UpdatedAt: now.Unix(), Deleted: true}
		tombstone.Sig, _ = c.Sign(canonical(protocol.MachinesLedgerKey, ip, tombstone))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: tombstone},
		})

		_, found := l.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse())
	})
})
