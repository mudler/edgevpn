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

	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/protocol"
)

// --- helpers -------------------------------------------------------------

func mkSignedEntry(s Signer, bucket, key string, value interface{}, version uint64, now time.Time) SignedData {
	jb, _ := json.Marshal(value)
	d := SignedData{Value: Data(jb), Owner: s.ID(), Version: version, UpdatedAt: now.Unix()}
	sig, err := s.Sign(canonical(bucket, key, d))
	Expect(err).NotTo(HaveOccurred())
	d.Sig = sig
	return d
}

// feed pushes a block carrying the given entries into the ledger via the same
// wire path Update consumes from gossip.
func feed(l *Ledger, entries map[string]map[string]SignedData) {
	b := l.LastBlock().NewBlock(entries)
	bb, _ := json.Marshal(b)
	l.Update(nil, hub.NewMessage(compress(bb).String()), nil)
}

func enforcedLedger(ttl time.Duration, now time.Time) *Ledger {
	return New(io.Discard, &MemoryStore{},
		WithEnforcedOwnership(DefaultRegistry(ttl), ttl),
		WithClock(func() time.Time { return now }),
	)
}

// heartbeat returns a signed healthcheck entry for s as of now.
func heartbeat(s Signer, now time.Time) map[string]map[string]SignedData {
	return map[string]map[string]SignedData{
		protocol.HealthCheckKey: {
			s.ID(): mkSignedEntry(s, protocol.HealthCheckKey, s.ID(), now.UTC().Format(time.RFC3339), 1, now),
		},
	}
}

func machine(peerID, addr string) map[string]string {
	return map[string]string{"PeerID": peerID, "Address": addr}
}

func storedOwner(l *Ledger, bucket, key string) string {
	v, _ := l.GetKey(bucket, key)
	var m map[string]string
	v.Unmarshal(&m)
	return m["PeerID"]
}

// --- specs ---------------------------------------------------------------

var _ = Describe("Authorized merge (enforced)", func() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ip := "10.1.0.1"

	It("accepts an owner's own write", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()

		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
		})

		_, found := l.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeTrue())
		Expect(storedOwner(l, protocol.MachinesLedgerKey, ip)).To(Equal(a.ID()))
	})

	It("rejects a hijack of a live owner's entry", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		b := newTestSigner()

		// A is alive and owns the IP.
		feed(l, heartbeat(a, now))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
		})

		// B tries to steal the IP with a higher version, validly signed by B.
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(b, protocol.MachinesLedgerKey, ip, machine(b.ID(), ip), 2, now)},
		})

		Expect(storedOwner(l, protocol.MachinesLedgerKey, ip)).To(Equal(a.ID()))
	})

	It("rejects a rollback to an older version", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()

		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), "10.9.9.9"), 2, now)},
		})
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), "10.0.0.0"), 1, now)},
		})

		v, _ := l.GetKey(protocol.MachinesLedgerKey, ip)
		var m map[string]string
		v.Unmarshal(&m)
		Expect(m["Address"]).To(Equal("10.9.9.9"))
	})

	It("rejects a forged signature", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()

		e := mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)
		e.Sig = []byte("garbage")
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: e}})

		_, found := l.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse())
	})

	It("allows reclaim once the owner has expired", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		b := newTestSigner()

		// A owns the IP but has NO heartbeat -> not live -> expired.
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
		})
		// B reclaims with a higher version.
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(b, protocol.MachinesLedgerKey, ip, machine(b.ID(), ip), 2, now)},
		})

		Expect(storedOwner(l, protocol.MachinesLedgerKey, ip)).To(Equal(b.ID()))
	})

	// DNS records carry no owner field, so the dns bucket is "self-owned": the
	// first signer to claim a name owns it, and while it is live no other peer
	// may take it over.
	It("blocks DNS hijack via first-claim", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		b := newTestSigner()
		name := "foo.internal"

		feed(l, heartbeat(a, now)) // A is live
		feed(l, map[string]map[string]SignedData{
			protocol.DNSKey: {name: mkSignedEntry(a, protocol.DNSKey, name, map[string]string{"1": "10.0.0.5"}, 1, now)},
		})
		_, found := l.GetKey(protocol.DNSKey, name)
		Expect(found).To(BeTrue())

		feed(l, map[string]map[string]SignedData{
			protocol.DNSKey: {name: mkSignedEntry(b, protocol.DNSKey, name, map[string]string{"1": "6.6.6.6"}, 2, now)},
		})

		v, _ := l.GetKey(protocol.DNSKey, name)
		var got map[string]string
		v.Unmarshal(&got)
		Expect(got["1"]).To(Equal("10.0.0.5"))
	})

	It("does not churn or warn on byte-identical re-broadcast (owned bucket)", func() {
		warnings := 0
		l := New(io.Discard, &MemoryStore{},
			WithEnforcedOwnership(DefaultRegistry(time.Minute), time.Minute),
			WithClock(func() time.Time { return now }),
			WithViolationLogger(func(string, ...interface{}) { warnings++ }))
		a := newTestSigner()
		entry := mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)

		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: entry}})
		idx := l.LastBlock().Index
		warnings = 0

		// The Syncronizer re-broadcasts the same block every interval; that must
		// neither mint a new block nor log a false violation.
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: entry}})
		Expect(l.LastBlock().Index).To(Equal(idx))
		Expect(warnings).To(BeZero())
	})

	It("does not churn on byte-identical re-broadcast (open bucket)", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		entry := mkSignedEntry(a, "nodes", "k", "v", 1, now)

		feed(l, map[string]map[string]SignedData{"nodes": {"k": entry}})
		idx := l.LastBlock().Index

		feed(l, map[string]map[string]SignedData{"nodes": {"k": entry}})
		Expect(l.LastBlock().Index).To(Equal(idx))
	})

	It("lets a valid owner reclaim a key cleared by a foreign tombstone", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()
		reaper := newTestSigner()

		// reaper is live; A is not (no heartbeat) so its entry is reapable.
		feed(l, heartbeat(reaper, now))
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
		})

		// reaper tombstones A's entry (cross-owner allowed: A expired).
		tomb := SignedData{Owner: reaper.ID(), Version: 2, UpdatedAt: now.Unix(), Deleted: true}
		tomb.Sig, _ = reaper.Sign(canonical(protocol.MachinesLedgerKey, ip, tomb))
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: tomb}})
		_, found := l.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse())

		// A returns and reclaims its own key with a higher version.
		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 3, now)},
		})
		Expect(storedOwner(l, protocol.MachinesLedgerKey, ip)).To(Equal(a.ID()))
	})

	// DeleteBucket tombstones every key; once a bucket has no live keys it must
	// disappear from CurrentData (matching legacy delete semantics), otherwise
	// GetBuckets keeps listing it and AnnounceDeleteBucket never terminates.
	It("drops a fully-tombstoned bucket from CurrentData", func() {
		l := enforcedLedger(time.Minute, now)
		l.SetSigner(newTestSigner())

		l.Add("mybucket", map[string]interface{}{"k": "v"})
		Expect(l.CurrentData()).To(HaveKey("mybucket"))

		l.DeleteBucket("mybucket")
		Expect(l.CurrentData()).NotTo(HaveKey("mybucket"))
	})

	It("hides a tombstoned entry", func() {
		l := enforcedLedger(time.Minute, now)
		a := newTestSigner()

		feed(l, map[string]map[string]SignedData{
			protocol.MachinesLedgerKey: {ip: mkSignedEntry(a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
		})

		tomb := SignedData{Owner: a.ID(), Version: 2, UpdatedAt: now.Unix(), Deleted: true}
		tomb.Sig, _ = a.Sign(canonical(protocol.MachinesLedgerKey, ip, tomb))
		feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: tomb}})

		_, found := l.GetKey(protocol.MachinesLedgerKey, ip)
		Expect(found).To(BeFalse())
	})
})
