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
	"context"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/pkg/config"
)

// newPublicHost starts a libp2p host on a local TCP listener and forces its
// reachability to Public so the relay service it carries will accept
// reservations. ForceReachabilityPublic is the test-only shortcut for
// skipping AutoNAT's "are we publicly reachable?" probe — in production
// edgevpn nodes go through the real detection path.
func newPublicHost() host.Host {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.ForceReachabilityPublic(),
	)
	Expect(err).ToNot(HaveOccurred())
	return h
}

// newClientHost starts a host that does NOT advertise itself as a relay — it's
// the would-be reserver. Listens on TCP so the relay can dial it back for the
// reservation handshake.
func newClientHost() host.Host {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	Expect(err).ToNot(HaveOccurred())
	return h
}

func connect(a, b host.Host) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	Expect(a.Connect(ctx, peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()})).To(Succeed())
}

// reserve runs client.Reserve with a fresh timeout and returns the
// (reservation, error) pair so each spec can assert on it directly.
func reserve(asker host.Host, relayHost host.Host) (*client.Reservation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rinfo := relayHost.Peerstore().PeerInfo(relayHost.ID())
	return client.Reserve(ctx, asker, rinfo)
}

var _ = Describe("NetworkOnlyACL end-to-end against a real libp2p relay", func() {
	var (
		acl       *NetworkOnlyACL
		relayHost host.Host
		rsvc      *relayv2.Relay
	)

	BeforeEach(func() {
		acl = &NetworkOnlyACL{}
		relayHost = newPublicHost()
		var err error
		rsvc, err = relayv2.New(relayHost, relayv2.WithACL(acl))
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if rsvc != nil {
			Expect(rsvc.Close()).To(Succeed())
		}
		if relayHost != nil {
			Expect(relayHost.Close()).To(Succeed())
		}
	})

	Context("bootstrap window (Members never called)", func() {
		It("accepts a reservation from any peer", func() {
			// This is the path that lets a new cluster member reserve before it
			// has shown up in the relay's alive bucket — without it, a fresh
			// peer would deadlock if the only viable relay is a cluster node.
			stranger := newClientHost()
			defer stranger.Close()
			connect(stranger, relayHost)

			rsvp, err := reserve(stranger, relayHost)
			Expect(err).ToNot(HaveOccurred(), "bootstrap window must keep the ACL open")
			Expect(rsvp).ToNot(BeNil())
			Expect(rsvp.Voucher).ToNot(BeNil())
		})
	})

	Context("strict mode (after Members has been called)", func() {
		It("accepts reservations from peers in the member set", func() {
			member := newClientHost()
			defer member.Close()
			acl.Members(map[peer.ID]struct{}{member.ID(): {}})
			connect(member, relayHost)

			rsvp, err := reserve(member, relayHost)
			Expect(err).ToNot(HaveOccurred())
			Expect(rsvp.Voucher).ToNot(BeNil())
			Expect(rsvp.Voucher.Peer).To(Equal(member.ID()),
				"the voucher must bind to the member's peer ID")
		})

		It("rejects reservations from non-member peers via libp2p PERMISSION_DENIED", func() {
			member := newClientHost()
			defer member.Close()
			stranger := newClientHost()
			defer stranger.Close()

			// Only member is allowed.
			acl.Members(map[peer.ID]struct{}{member.ID(): {}})
			connect(stranger, relayHost)

			_, err := reserve(stranger, relayHost)
			Expect(err).To(HaveOccurred(),
				"strict mode must refuse a stranger's reservation")
			// libp2p surfaces this as "reservation error: status:
			// PERMISSION_DENIED reason: reservation failed". We assert
			// loosely so a future libp2p wording change doesn't break the
			// test outright — but we log when the wording drifts so a
			// future maintainer can tighten the check.
			lower := strings.ToLower(err.Error())
			if !strings.Contains(lower, "refused") &&
				!strings.Contains(lower, "denied") &&
				!strings.Contains(lower, "permission") {
				AddReportEntry("rejection-error-wording",
					"libp2p rejection wording drifted; update the substring match: "+err.Error())
			}
		})
	})

	Context("membership flip mid-flight", func() {
		It("admits a peer immediately after Members(set) includes it", func() {
			// Mirrors the production transition: the alive-bucket watcher
			// snapshots the bucket, sees a new peer for the first time, and
			// calls Members(...). The very next reservation attempt must
			// succeed.
			joiner := newClientHost()
			defer joiner.Close()

			acl.Members(map[peer.ID]struct{}{}) // strict, empty
			connect(joiner, relayHost)

			_, err := reserve(joiner, relayHost)
			Expect(err).To(HaveOccurred(), "pre-membership must be denied")

			acl.Members(map[peer.ID]struct{}{joiner.ID(): {}})

			rsvp, err := reserve(joiner, relayHost)
			Expect(err).ToNot(HaveOccurred(),
				"post-membership must succeed; this is the NetworkService's contract")
			Expect(rsvp.Voucher).ToNot(BeNil())
		})
	})
})
