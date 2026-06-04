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
	"github.com/libp2p/go-libp2p/core/crypto"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newTestSigner() Signer {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	Expect(err).NotTo(HaveOccurred())
	s, err := NewSigner(priv)
	Expect(err).NotTo(HaveOccurred())
	return s
}

var _ = Describe("Signing", func() {
	It("round-trips a valid signature", func() {
		s := newTestSigner()
		d := SignedData{Value: "hello", Owner: s.ID(), Version: 1, UpdatedAt: 100}

		sig, err := s.Sign(canonical("machines", "10.1.0.1", d))
		Expect(err).NotTo(HaveOccurred())
		d.Sig = sig

		Expect(Verify("machines", "10.1.0.1", d)).To(Succeed())
	})

	It("rejects a tampered value", func() {
		s := newTestSigner()
		d := SignedData{Value: "hello", Owner: s.ID(), Version: 1, UpdatedAt: 100}
		d.Sig, _ = s.Sign(canonical("machines", "10.1.0.1", d))

		d.Value = "tampered"

		Expect(Verify("machines", "10.1.0.1", d)).NotTo(Succeed())
	})

	// A signature is bound to the claimed owner: presenting it under a different
	// Owner peer.ID must not verify (this is what stops impersonation).
	It("rejects a signature presented under a different owner", func() {
		s := newTestSigner()
		other := newTestSigner()

		d := SignedData{Value: "hello", Owner: s.ID(), Version: 1, UpdatedAt: 100}
		d.Sig, _ = s.Sign(canonical("machines", "10.1.0.1", d))

		d.Owner = other.ID()

		Expect(Verify("machines", "10.1.0.1", d)).NotTo(Succeed())
	})
})
