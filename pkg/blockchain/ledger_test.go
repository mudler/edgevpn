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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/hub"
)

// msgFor wraps a ledger's last block into the wire form Update expects
// (gzip-compressed JSON), mirroring what the ledger broadcasts.
func msgFor(l *Ledger) *hub.Message {
	bb, _ := json.Marshal(l.LastBlock())
	return hub.NewMessage(compress(bb).String())
}

// announce mimics one AnnounceUpdate tick: (re)write our own key only if it is
// not already the current value. This is what each node does periodically and
// is what lets the loser of a tie-break re-introduce the data it dropped when
// it adopted the peer's block.
func announce(l *Ledger, key string) {
	if v, ok := l.CurrentData()["nodes"][key]; ok && string(v) == `"1"` {
		return
	}
	l.Add("nodes", map[string]interface{}{key: "1"})
}

func hasKeys(l *Ledger, keys ...string) bool {
	nodes := l.CurrentData()["nodes"]
	for _, k := range keys {
		if _, ok := nodes[k]; !ok {
			return false
		}
	}
	return true
}

var _ = Describe("Legacy merge (ownership off)", func() {
	// Regression guard for the equal-index split-brain. Two ledgers
	// independently reach the same index with different data (e.g. two peers
	// booted simultaneously, each having advertised once). It asserts the full
	// outcome that matters: after a few exchange + re-announce rounds BOTH
	// ledgers hold BOTH advertisements and agree on the same block.
	//
	// Note the intermediate state: a single Update only makes the loser adopt
	// the winner's block (the loser's key is momentarily gone). The union is
	// reached on the next announce — the loser re-adds its key, Add unions it
	// onto the current (winner's) storage at a higher index, and the winner
	// adopts that via height.
	It("converges an equal-index split-brain to the union", func() {
		a := New(io.Discard, &MemoryStore{})
		b := New(io.Discard, &MemoryStore{})

		a.Add("nodes", map[string]interface{}{"a": "1"})
		b.Add("nodes", map[string]interface{}{"b": "1"})

		// Precondition: same height, different blocks — the deadlock scenario
		// that `>`-only rejection could never resolve.
		Expect(a.LastBlock().Index).To(Equal(b.LastBlock().Index), "precondition: equal index")
		Expect(a.LastBlock().Hash).NotTo(Equal(b.LastBlock().Hash), "precondition: differing blocks")

		// Drive a few reconcile rounds (exchange, then each re-announces its own
		// key). Convergence is reached well within this; extra rounds are no-ops.
		for i := 0; i < 5; i++ {
			Expect(a.Update(a, msgFor(b), nil)).To(Succeed())
			Expect(b.Update(b, msgFor(a), nil)).To(Succeed())
			announce(a, "a")
			announce(b, "b")
		}
		// Final exchange so both observe the latest block.
		Expect(a.Update(a, msgFor(b), nil)).To(Succeed())
		Expect(b.Update(b, msgFor(a), nil)).To(Succeed())

		Expect(hasKeys(a, "a", "b")).To(BeTrue(), "node a missing an advertisement")
		Expect(hasKeys(b, "a", "b")).To(BeTrue(), "node b missing an advertisement")
		Expect(a.LastBlock().Hash).To(Equal(b.LastBlock().Hash), "ledgers did not converge on the same block")
	})

	// Ensures the height rule (and thus deletion propagation, which works by
	// raising the index) is unchanged by the tie-break.
	It("adopts a strictly higher-index block regardless of hash order", func() {
		a := New(io.Discard, &MemoryStore{})
		b := New(io.Discard, &MemoryStore{})

		// b climbs higher than a.
		a.Add("nodes", map[string]interface{}{"a": "1"})
		b.Add("nodes", map[string]interface{}{"b": "1"})
		b.Add("nodes", map[string]interface{}{"b": "2"})

		Expect(b.LastBlock().Index).To(BeNumerically(">", a.LastBlock().Index), "precondition: b higher than a")

		Expect(a.Update(a, msgFor(b), nil)).To(Succeed())
		Expect(a.LastBlock().Hash).To(Equal(b.LastBlock().Hash), "higher-index block was not adopted")
	})
})
