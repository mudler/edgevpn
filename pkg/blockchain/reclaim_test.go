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
	"fmt"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/protocol"
)

var _ = Describe("An owner whose entries were reaped", func() {
	It("gets its address back once its heartbeats reach the reaper again", func() {
		const ttl = time.Minute
		const ip = "10.1.0.4"
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		clock := func() time.Time { return now }

		var warnings []string
		logWarning := func(format string, args ...interface{}) {
			warnings = append(warnings, fmt.Sprintf(format, args...))
		}
		newLedger := func(s Signer) *Ledger {
			l := New(io.Discard, &MemoryStore{},
				WithEnforcedOwnership(DefaultRegistry(ttl), ttl),
				WithClock(clock),
				WithViolationLogger(logWarning),
			)
			l.SetSigner(s)
			return l
		}
		desktopKey, phoneKey := newTestSigner(), newTestSigner()
		desktop, phone := newLedger(desktopKey), newLedger(phoneKey)

		beat := func(l *Ledger, s Signer) {
			l.Add(protocol.HealthCheckKey, map[string]interface{}{s.ID(): now.UTC().Format(time.RFC3339)})
		}
		// The phone's VPN service claims its address the way pkg/vpn does: it
		// writes the entry only when its own ledger has none for the address, or
		// one owned by another peer.
		announce := func() {
			v, found := phone.GetKey(protocol.MachinesLedgerKey, ip)
			var m map[string]string
			v.Unmarshal(&m)
			if !found || m["PeerID"] != phoneKey.ID() {
				phone.Add(protocol.MachinesLedgerKey, map[string]interface{}{ip: machine(phoneKey.ID(), ip)})
			}
		}
		sync := func() {
			feed(desktop, phone.CurrentStorage())
			feed(phone, desktop.CurrentStorage())
		}

		beat(phone, phoneKey)
		beat(desktop, desktopKey)
		announce()
		sync()
		_, found := desktop.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeTrue(), "the desktop should learn the phone's address")

		// The phone's messages stop reaching the desktop for longer than the
		// ownership TTL. The desktop is the leader and reaps the phone's entries.
		now = now.Add(2 * ttl)
		beat(phone, phoneKey)
		beat(desktop, desktopKey)
		desktop.Reap(time.Hour)
		_, found = desktop.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse(), "the desktop should reap the silent phone's address")

		// The phone is heard from again and keeps announcing.
		for range 3 {
			now = now.Add(15 * time.Second)
			beat(phone, phoneKey)
			beat(desktop, desktopKey)
			announce()
			sync()
		}

		_, found = desktop.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeTrue(), "the desktop never took the phone's address back; ledger warnings: %v", warnings)
	})

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	const ip = "10.1.0.4"
	foreignTombstone := func(reaper Signer, over SignedData) SignedData {
		t := SignedData{Owner: reaper.ID(), Version: over.Version + 1, UpdatedAt: now.Unix(), Deleted: true}
		t.Sig, _ = reaper.Sign(canonical(protocol.MachinesLedgerKey, ip, t))
		return t
	}

	It("re-asserts its own live entry over a tombstone written by another peer", func() {
		l := enforcedLedger(time.Minute, now)
		owner, reaper := newTestSigner(), newTestSigner()
		l.SetSigner(owner)
		l.Add(protocol.HealthCheckKey, map[string]interface{}{owner.ID(): now.UTC().Format(time.RFC3339)})
		l.Add(protocol.MachinesLedgerKey, map[string]interface{}{ip: machine(owner.ID(), ip)})
		mine := l.CurrentStorage()[protocol.MachinesLedgerKey][ip]

		tomb := foreignTombstone(reaper, mine)
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: tomb}})

		got := l.CurrentStorage()[protocol.MachinesLedgerKey][ip]
		Expect(got.Deleted).To(BeFalse())
		Expect(got.Owner).To(Equal(owner.ID()))
		Expect(got.Value).To(Equal(mine.Value))
		Expect(got.Version).To(BeNumerically(">", tomb.Version), "the re-asserted entry must out-version the tombstone")
		Expect(Verify(protocol.MachinesLedgerKey, ip, got)).To(Succeed())
	})

	It("is not re-asserted over a tombstone without a valid signature", func() {
		l := enforcedLedger(time.Minute, now)
		owner, reaper := newTestSigner(), newTestSigner()
		l.SetSigner(owner)
		l.Add(protocol.HealthCheckKey, map[string]interface{}{owner.ID(): now.UTC().Format(time.RFC3339)})
		l.Add(protocol.MachinesLedgerKey, map[string]interface{}{ip: machine(owner.ID(), ip)})
		mine := l.CurrentStorage()[protocol.MachinesLedgerKey][ip]

		forged := SignedData{Owner: reaper.ID(), Version: ^uint64(0) - 1, UpdatedAt: now.Unix(), Deleted: true}
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: forged}})

		Expect(l.CurrentStorage()[protocol.MachinesLedgerKey][ip]).To(Equal(mine))
	})

	It("is not re-asserted over a tombstone whose version lies far in the future", func() {
		l := enforcedLedger(time.Minute, now)
		owner, reaper := newTestSigner(), newTestSigner()
		l.SetSigner(owner)
		l.Add(protocol.HealthCheckKey, map[string]interface{}{owner.ID(): now.UTC().Format(time.RFC3339)})
		l.Add(protocol.MachinesLedgerKey, map[string]interface{}{ip: machine(owner.ID(), ip)})
		mine := l.CurrentStorage()[protocol.MachinesLedgerKey][ip]

		future := SignedData{Owner: reaper.ID(), Version: ^uint64(0) - 1, UpdatedAt: now.Unix(), Deleted: true}
		future.Sig, _ = reaper.Sign(canonical(protocol.MachinesLedgerKey, ip, future))
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: future}})

		Expect(l.CurrentStorage()[protocol.MachinesLedgerKey][ip]).To(Equal(mine))
	})

	It("is left alone by a third peer that receives the same tombstone", func() {
		l := enforcedLedger(time.Minute, now)
		owner, reaper, third := newTestSigner(), newTestSigner(), newTestSigner()
		l.SetSigner(third)
		feed(l, heartbeat(owner, now))
		mine := mkSignedEntry(owner, protocol.MachinesLedgerKey, ip, machine(owner.ID(), ip), 1, now)
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: mine}})

		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: foreignTombstone(reaper, mine)}})

		Expect(l.CurrentStorage()[protocol.MachinesLedgerKey][ip]).To(Equal(mine))
	})
})
