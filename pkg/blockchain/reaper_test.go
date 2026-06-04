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
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/protocol"
)

var _ = Describe("Reaper", func() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	It("tombstones entries of inactive owners and keeps live ones", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		b := newTestSigner()

		// A is live and owns ip1; B has no heartbeat and owns ip2.
		feed(l, heartbeat(a, now))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {
				"10.1.0.1": mkSignedEntry(a, protocol.MachinesLedgerKey, "10.1.0.1", machine(a.ID(), "10.1.0.1"), 1, now),
				"10.1.0.2": mkSignedEntry(b, protocol.MachinesLedgerKey, "10.1.0.2", machine(b.ID(), "10.1.0.2"), 1, now),
			},
		})

		// Reaping happens by an authorized signer.
		l.SetSigner(a)
		l.Reap(time.Hour)

		_, foundB := l.GetKey(protocol.MachinesLedgerKey, "10.1.0.2")
		Expect(foundB).To(BeFalse(), "entry of inactive owner B should be tombstoned")
		_, foundA := l.GetKey(protocol.MachinesLedgerKey, "10.1.0.1")
		Expect(foundA).To(BeTrue(), "entry of live owner A must be kept")
	})

	It("prunes tombstones older than the tombstone TTL", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		l.SetSigner(a)

		old := SignedData{Owner: a.ID(), Version: 5, UpdatedAt: now.Add(-2 * time.Hour).Unix(), Deleted: true}
		old.Sig, _ = a.Sign(canonical(protocol.MachinesLedgerKey, "10.1.0.9", old))
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {"10.1.0.9": old}})

		l.Reap(time.Hour) // tombstone-ttl = 1h, this one is 2h old

		_, ok := l.CurrentStorage()[protocol.MachinesLedgerKey]["10.1.0.9"]
		Expect(ok).To(BeFalse(), "tombstone older than the tombstone-ttl must be pruned")
	})

	Describe("IsOwnerLive", func() {
		It("treats every owner as live when ownership is disabled", func() {
			off := New(io.Discard, &MemoryStore{})
			Expect(off.IsOwnerLive("anyone")).To(BeTrue())
		})

		It("reflects heartbeat freshness under enforcement", func() {
			l := enforcedLedger(time.Minute, now)
			a := newTestSigner()
			feed(l, heartbeat(a, now))

			Expect(l.IsOwnerLive(a.ID())).To(BeTrue())
			Expect(l.IsOwnerLive("ghost")).To(BeFalse())
		})
	})
})
