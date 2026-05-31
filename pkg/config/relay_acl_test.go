/*
Copyright © 2021-2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package config_test

import (
	"crypto/rand"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/pkg/config"
)

// newRandomPeer generates a fresh libp2p peer.ID for use as a test
// fixture. Tests using two of these to compare a "member" vs a
// "stranger" need them to come from independent key material so the
// resulting peer IDs are distinct.
func newRandomPeer() peer.ID {
	priv, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	Expect(err).ToNot(HaveOccurred())
	pid, err := peer.IDFromPrivateKey(priv)
	Expect(err).ToNot(HaveOccurred())
	return pid
}

var _ = Describe("NetworkOnlyACL", func() {
	Context("bootstrap window", func() {
		It("admits any peer until the first Members call", func() {
			acl := &NetworkOnlyACL{}
			stranger := newRandomPeer()

			Expect(acl.AllowReserve(stranger, nil)).To(BeTrue(),
				"the ACL must be open in bootstrap mode to avoid deadlocking new peers")

			// Sanity: the type satisfies the libp2p ACLFilter contract.
			var _ relayv2.ACLFilter = acl
		})
	})

	Context("strict mode (after Members has been called)", func() {
		It("admits a peer that is in the member set", func() {
			acl := &NetworkOnlyACL{}
			member := newRandomPeer()
			acl.Members(map[peer.ID]struct{}{member: {}})

			Expect(acl.AllowReserve(member, nil)).To(BeTrue())
		})

		It("rejects a peer that is not in the member set", func() {
			acl := &NetworkOnlyACL{}
			member := newRandomPeer()
			stranger := newRandomPeer()
			acl.Members(map[peer.ID]struct{}{member: {}})

			Expect(acl.AllowReserve(stranger, nil)).To(BeFalse(),
				"strict mode must reject peers absent from the alive bucket")
		})

		It("permits AllowConnect regardless of membership", func() {
			// We gate the reservation step only; existing relayed
			// sessions must not get yanked if the alive bucket flickers.
			acl := &NetworkOnlyACL{}
			src := newRandomPeer()
			dst := newRandomPeer()
			acl.Members(map[peer.ID]struct{}{}) // strict, empty set

			Expect(acl.AllowConnect(src, nil, dst)).To(BeTrue())
		})
	})

	Context("Members snapshot ownership", func() {
		It("defensively copies the caller's map", func() {
			acl := &NetworkOnlyACL{}
			member := newRandomPeer()

			set := map[peer.ID]struct{}{member: {}}
			acl.Members(set)

			// Mutate the caller's map after handing it over.
			delete(set, member)

			Expect(acl.AllowReserve(member, nil)).To(BeTrue(),
				"the ACL must not alias the caller's map; readers would race the delete above")
		})
	})
})
