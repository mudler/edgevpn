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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SignedData wire format", func() {
	// An unsigned entry must serialise to the exact pre-authentication wire
	// format (a bare JSON value), so existing nodes that decode Storage values
	// as plain strings keep working.
	It("keeps unsigned entries on the legacy bare-value format", func() {
		d := SignedData{Value: Data(`{"PeerID":"x"}`)}

		got, err := json.Marshal(d)
		Expect(err).NotTo(HaveOccurred())
		want, _ := json.Marshal(`{"PeerID":"x"}`) // legacy: Data is a string -> JSON string
		Expect(string(got)).To(Equal(string(want)))

		var back SignedData
		Expect(json.Unmarshal(got, &back)).To(Succeed())
		Expect(back.Value).To(Equal(d.Value))
		Expect(back.Owner).To(BeEmpty())
		Expect(back.Version).To(BeZero())
	})

	It("round-trips a signed entry as the full object", func() {
		d := SignedData{Value: Data(`"v"`), Owner: "peerX", Version: 3, UpdatedAt: 100, Sig: []byte{1, 2, 3}}

		b, err := json.Marshal(d)
		Expect(err).NotTo(HaveOccurred())

		var back SignedData
		Expect(json.Unmarshal(b, &back)).To(Succeed())
		Expect(back.Owner).To(Equal("peerX"))
		Expect(back.Version).To(Equal(uint64(3)))
		Expect(back.UpdatedAt).To(Equal(int64(100)))
		Expect(back.Sig).To(Equal([]byte{1, 2, 3}))
		Expect(back.Value).To(Equal(d.Value))
	})
})
