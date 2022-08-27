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

package node_test

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/peer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	. "github.com/mudler/edgevpn/pkg/node"
)

var _ = Describe("Node", func() {
	// Trigger key rotation on a low frequency to test everything works in between
	token := GenerateNewConnectionData(25).Base64()

	l := Logger(logger.New(log.LevelFatal))

	Context("Configuration", func() {
		It("fails if is not valid", func() {
			_, err := New(FromBase64(true, true, "  ", nil, nil), WithStore(&blockchain.MemoryStore{}), l)
			Expect(err).To(HaveOccurred())
			_, err = New(FromBase64(true, true, token, nil, nil), WithStore(&blockchain.MemoryStore{}), l)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Connection", func() {
		It("see each other node ID", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			e, _ := New(FromBase64(true, true, token, nil, nil), WithStore(&blockchain.MemoryStore{}), l)
			e2, _ := New(FromBase64(true, true, token, nil, nil), WithStore(&blockchain.MemoryStore{}), l)

			e.Start(ctx)
			e2.Start(ctx)

			Eventually(func() []peer.ID {
				return e.Host().Network().Peers()
			}, 240*time.Second, 1*time.Second).Should(ContainElement(e2.Host().ID()))
		})

		It("nodes can write to the ledger", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			e, _ := New(FromBase64(true, true, token, nil, nil), WithStore(&blockchain.MemoryStore{}), WithDiscoveryInterval(10*time.Second), l)
			e2, _ := New(FromBase64(true, true, token, nil, nil), WithStore(&blockchain.MemoryStore{}), WithDiscoveryInterval(10*time.Second), l)

			e.Start(ctx)
			e2.Start(ctx)

			l, err := e.Ledger()
			Expect(err).ToNot(HaveOccurred())
			l2, err := e2.Ledger()
			Expect(err).ToNot(HaveOccurred())

			l.Announce(ctx, 2*time.Second, func() { l.Add("foo", map[string]interface{}{"bar": "baz"}) })

			Eventually(func() string {
				var s string
				v, exists := l2.GetKey("foo", "bar")
				if exists {
					v.Unmarshal(&s)
				}
				return s
			}, 240*time.Second, 1*time.Second).Should(Equal("baz"))
		})
	})

	Context("connection gater", func() {
		It("blacklists", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e, _ := New(
				WithBlacklist("1.1.1.1/32", "1.1.1.0/24"),
				FromBase64(true, true, token, nil, nil),
				WithStore(&blockchain.MemoryStore{}),
				l,
			)

			e.Start(ctx)
			addrs := e.ConnectionGater().ListBlockedAddrs()
			peers := e.ConnectionGater().ListBlockedPeers()
			subs := e.ConnectionGater().ListBlockedSubnets()
			Expect(len(addrs)).To(Equal(0))
			Expect(len(peers)).To(Equal(0))
			Expect(len(subs)).To(Equal(2))

			ips := []string{}
			for _, s := range subs {
				ips = append(ips, s.String())
			}
			Expect(ips).To(ContainElements("1.1.1.1/32", "1.1.1.0/24"))
		})
	})
})
