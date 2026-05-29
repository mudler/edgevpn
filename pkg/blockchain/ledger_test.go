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

	"github.com/mudler/edgevpn/pkg/hub"
)

// msgFor wraps a ledger's last block into the wire form Update expects
// (gzip-compressed JSON), mirroring what writeData broadcasts.
func msgFor(l *Ledger) *hub.Message {
	bb, _ := json.Marshal(l.LastBlock())
	return hub.NewMessage(compress(bb).String())
}

// TestUpdateEqualIndexTieBreak guards the fix for the equal-index split-brain:
// two ledgers that independently climb to the same index with different data
// (e.g. two peers that start simultaneously and advertise in lockstep) must
// converge on exchange instead of rejecting each other forever.
func TestUpdateEqualIndexTieBreak(t *testing.T) {
	a := New(io.Discard, &MemoryStore{})
	b := New(io.Discard, &MemoryStore{})

	a.Add("nodes", map[string]interface{}{"a": "1"})
	b.Add("nodes", map[string]interface{}{"b": "1"})

	// Precondition: same height, different blocks — the deadlock scenario.
	if a.LastBlock().Index != b.LastBlock().Index {
		t.Fatalf("precondition: expected equal index, got %d and %d",
			a.LastBlock().Index, b.LastBlock().Index)
	}
	if a.LastBlock().Hash == b.LastBlock().Hash {
		t.Fatal("precondition: expected the two blocks to differ")
	}

	// Each node receives the other's equal-index block.
	if err := a.Update(a, msgFor(b), nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Update(b, msgFor(a), nil); err != nil {
		t.Fatal(err)
	}

	// Deterministic tie-break: both must now hold the same (higher-hash) block,
	// not their own divergent ones.
	if a.LastBlock().Hash != b.LastBlock().Hash {
		t.Fatalf("tie-break did not converge: a=%s b=%s",
			a.LastBlock().Hash, b.LastBlock().Hash)
	}
}

// TestUpdateHigherIndexStillWins ensures the height rule (and thus deletion
// propagation, which works by raising the index) is unchanged by the tie-break.
func TestUpdateHigherIndexStillWins(t *testing.T) {
	a := New(io.Discard, &MemoryStore{})
	b := New(io.Discard, &MemoryStore{})

	// b climbs higher than a.
	a.Add("nodes", map[string]interface{}{"a": "1"})
	b.Add("nodes", map[string]interface{}{"b": "1"})
	b.Add("nodes", map[string]interface{}{"b": "2"})

	if b.LastBlock().Index <= a.LastBlock().Index {
		t.Fatalf("precondition: expected b higher than a, got %d and %d",
			b.LastBlock().Index, a.LastBlock().Index)
	}

	// a receives b's higher block and must adopt it regardless of hash order.
	if err := a.Update(a, msgFor(b), nil); err != nil {
		t.Fatal(err)
	}
	if a.LastBlock().Hash != b.LastBlock().Hash {
		t.Fatal("higher-index block was not adopted")
	}
}
