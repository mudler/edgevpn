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
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/protocol"
)

// --- helpers -------------------------------------------------------------

func mkSignedEntry(t *testing.T, s Signer, bucket, key string, value interface{}, version uint64, now time.Time) SignedData {
	t.Helper()
	jb, _ := json.Marshal(value)
	d := SignedData{Value: Data(jb), Owner: s.ID(), Version: version, UpdatedAt: now.Unix()}
	sig, err := s.Sign(canonical(bucket, key, d))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
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
func heartbeat(t *testing.T, s Signer, now time.Time) map[string]map[string]SignedData {
	return map[string]map[string]SignedData{
		protocol.HealthCheckKey: {
			s.ID(): mkSignedEntry(t, s, protocol.HealthCheckKey, s.ID(), now.UTC().Format(time.RFC3339), 1, now),
		},
	}
}

func machine(peerID, addr string) map[string]string {
	return map[string]string{"PeerID": peerID, "Address": addr}
}

// --- tests ---------------------------------------------------------------

func TestMergeAcceptsOwnerWrite(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	ip := "10.1.0.1"

	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
	})

	v, found := l.GetKey(protocol.MachinesLedgerKey, ip)
	if !found {
		t.Fatal("owner write should be accepted")
	}
	var m map[string]string
	v.Unmarshal(&m)
	if m["PeerID"] != a.ID() {
		t.Fatalf("stored owner = %q, want %q", m["PeerID"], a.ID())
	}
}

func TestMergeRejectsHijackOfLiveOwner(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	b := newTestSigner(t)
	ip := "10.1.0.1"

	// A is alive and owns the IP.
	feed(l, heartbeat(t, a, now))
	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
	})

	// B tries to steal the IP with a higher version, validly signed by B.
	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, b, protocol.MachinesLedgerKey, ip, machine(b.ID(), ip), 2, now)},
	})

	v, _ := l.GetKey(protocol.MachinesLedgerKey, ip)
	var m map[string]string
	v.Unmarshal(&m)
	if m["PeerID"] != a.ID() {
		t.Fatalf("hijack succeeded: owner = %q, want %q (A, who is live)", m["PeerID"], a.ID())
	}
}

func TestMergeRejectsRollback(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	ip := "10.1.0.1"

	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), "10.9.9.9"), 2, now)},
	})
	// Replay an older version of A's own entry.
	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), "10.0.0.0"), 1, now)},
	})

	v, _ := l.GetKey(protocol.MachinesLedgerKey, ip)
	var m map[string]string
	v.Unmarshal(&m)
	if m["Address"] != "10.9.9.9" {
		t.Fatalf("rollback accepted: address = %q, want 10.9.9.9", m["Address"])
	}
}

func TestMergeRejectsForgedSignature(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	ip := "10.1.0.1"

	e := mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)
	e.Sig = []byte("garbage")
	feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: e}})

	if _, found := l.GetKey(protocol.MachinesLedgerKey, ip); found {
		t.Fatal("entry with forged signature must be rejected")
	}
}

func TestMergeAllowsReclaimAfterOwnerExpired(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	b := newTestSigner(t)
	ip := "10.1.0.1"

	// A owns the IP but has NO heartbeat -> not live -> expired.
	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
	})
	// B reclaims with a higher version.
	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, b, protocol.MachinesLedgerKey, ip, machine(b.ID(), ip), 2, now)},
	})

	v, _ := l.GetKey(protocol.MachinesLedgerKey, ip)
	var m map[string]string
	v.Unmarshal(&m)
	if m["PeerID"] != b.ID() {
		t.Fatalf("reclaim of expired owner failed: owner = %q, want %q", m["PeerID"], b.ID())
	}
}

// DNS records carry no owner field, so the dns bucket is "self-owned": the first
// signer to claim a name owns it, and (while it is live) no other peer may take
// it over. This blocks hijacking an existing DNS name and enables reaping.
func TestMergeDNSFirstClaimBlocksHijack(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	b := newTestSigner(t)
	name := "foo.internal"

	feed(l, heartbeat(t, a, now)) // A is live
	feed(l, map[string]map[string]SignedData{
		protocol.DNSKey: {name: mkSignedEntry(t, a, protocol.DNSKey, name, map[string]string{"1": "10.0.0.5"}, 1, now)},
	})
	if _, found := l.GetKey(protocol.DNSKey, name); !found {
		t.Fatal("first claim of a DNS name should be accepted")
	}

	// B tries to repoint the name with a higher version while A is live.
	feed(l, map[string]map[string]SignedData{
		protocol.DNSKey: {name: mkSignedEntry(t, b, protocol.DNSKey, name, map[string]string{"1": "6.6.6.6"}, 2, now)},
	})

	v, _ := l.GetKey(protocol.DNSKey, name)
	var got map[string]string
	v.Unmarshal(&got)
	if got["1"] != "10.0.0.5" {
		t.Fatalf("DNS hijack of a live owner succeeded: %v", got)
	}
}

func TestMergeTombstoneHidesEntry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	ip := "10.1.0.1"

	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {ip: mkSignedEntry(t, a, protocol.MachinesLedgerKey, ip, machine(a.ID(), ip), 1, now)},
	})

	// Signed tombstone by the owner.
	tomb := SignedData{Owner: a.ID(), Version: 2, UpdatedAt: now.Unix(), Deleted: true}
	tomb.Sig, _ = a.Sign(canonical(protocol.MachinesLedgerKey, ip, tomb))
	feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {ip: tomb}})

	if _, found := l.GetKey(protocol.MachinesLedgerKey, ip); found {
		t.Fatal("tombstoned entry must read as absent")
	}
}
