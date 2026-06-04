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
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/protocol"
)

func TestIsOwnerLive(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// With ownership disabled, liveness is not tracked: everyone is "live" so
	// readers keep their current behaviour.
	off := New(io.Discard, &MemoryStore{})
	if !off.IsOwnerLive("anyone") {
		t.Fatal("ownership off should treat any owner as live")
	}

	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	feed(l, heartbeat(t, a, now))

	if !l.IsOwnerLive(a.ID()) {
		t.Fatal("owner with a fresh heartbeat should be live")
	}
	if l.IsOwnerLive("ghost") {
		t.Fatal("owner with no heartbeat should not be live")
	}
}

func TestReapTombstonesInactiveOwners(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	b := newTestSigner(t)

	// A is live and owns ip1; B has no heartbeat and owns ip2.
	feed(l, heartbeat(t, a, now))
	feed(l, map[string]map[string]SignedData{
		protocol.MachinesLedgerKey: {
			"10.1.0.1": mkSignedEntry(t, a, protocol.MachinesLedgerKey, "10.1.0.1", machine(a.ID(), "10.1.0.1"), 1, now),
			"10.1.0.2": mkSignedEntry(t, b, protocol.MachinesLedgerKey, "10.1.0.2", machine(b.ID(), "10.1.0.2"), 1, now),
		},
	})

	// Reaping must happen by an authorized signer; give the ledger A's key.
	l.SetSigner(a)
	l.Reap(time.Hour)

	if _, found := l.GetKey(protocol.MachinesLedgerKey, "10.1.0.2"); found {
		t.Fatal("entry of inactive owner B should be tombstoned")
	}
	if _, found := l.GetKey(protocol.MachinesLedgerKey, "10.1.0.1"); !found {
		t.Fatal("entry of live owner A must be kept")
	}
}

func TestReapPrunesOldTombstones(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := enforcedLedger(time.Minute, now)
	a := newTestSigner(t)
	l.SetSigner(a)

	// An old tombstone (UpdatedAt well in the past) for a machines key.
	old := SignedData{Owner: a.ID(), Version: 5, UpdatedAt: now.Add(-2 * time.Hour).Unix(), Deleted: true}
	old.Sig, _ = a.Sign(canonical(protocol.MachinesLedgerKey, "10.1.0.9", old))
	feed(l, map[string]map[string]SignedData{protocol.MachinesLedgerKey: {"10.1.0.9": old}})

	l.Reap(time.Hour) // tombstone-ttl = 1h, this one is 2h old

	if _, ok := l.CurrentStorage()[protocol.MachinesLedgerKey]["10.1.0.9"]; ok {
		t.Fatal("tombstone older than the tombstone-ttl must be physically pruned")
	}
}
