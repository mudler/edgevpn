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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/protocol"
)

func tsData(t time.Time) Data {
	b, _ := json.Marshal(t.UTC().Format(time.RFC3339))
	return Data(b)
}

var _ = Describe("Policy & liveness", func() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	Describe("IsLive", func() {
		hd := map[string]Data{
			"peerA": tsData(now.Add(-10 * time.Second)),
			"peerB": tsData(now.Add(-10 * time.Minute)),
		}

		It("reports a peer with a recent heartbeat as live", func() {
			Expect(IsLive(hd, "peerA", time.Minute, now)).To(BeTrue())
		})
		It("reports a peer with a stale heartbeat as not live", func() {
			Expect(IsLive(hd, "peerB", time.Minute, now)).To(BeFalse())
		})
		It("reports a peer with no heartbeat as not live", func() {
			Expect(IsLive(hd, "unknown", time.Minute, now)).To(BeFalse())
		})
	})

	Describe("DefaultRegistry ownership", func() {
		r := DefaultRegistry(time.Minute)

		It("derives the machine owner from the PeerID field", func() {
			machine, _ := json.Marshal(map[string]string{"PeerID": "peerX", "Address": "10.1.0.1"})
			Expect(r.Policy(protocol.MachinesLedgerKey).OwnerOf("10.1.0.1", Data(machine))).To(Equal("peerX"))
		})
		It("treats the key as the owner for the users bucket", func() {
			Expect(r.Policy(protocol.UsersLedgerKey).OwnerOf("peerY", "")).To(Equal("peerY"))
		})
		It("leaves unregistered buckets unowned (open)", func() {
			Expect(r.Policy("some-unregistered-bucket").Owned).To(BeFalse())
		})
	})
})
