// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package node_test

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	. "github.com/mudler/edgevpn/pkg/node"
)

var _ = Describe("Node", func() {
	token := GenerateNewConnectionData().Base64()

	l := Logger(logger.New(log.LevelFatal))

	Context("Configuration", func() {
		It("fails if is not valid", func() {
			_, err := New(FromBase64(true, true, "  "), WithStore(&blockchain.MemoryStore{}), l)
			Expect(err).To(HaveOccurred())
			_, err = New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Connection", func() {
		It("see each other node ID", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			e, _ := New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)
			e2, _ := New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)

			e.Start(ctx)
			e2.Start(ctx)

			Eventually(func() []peer.ID {
				return e.Host().Network().Peers()
			}, 100*time.Second, 1*time.Second).Should(ContainElement(e2.Host().ID()))
		})

		It("nodes can write to the ledger", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			e, _ := New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)
			e2, _ := New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)

			e.Start(ctx)
			e2.Start(ctx)

			l, err := e.Ledger()
			Expect(err).ToNot(HaveOccurred())
			l2, err := e2.Ledger()
			Expect(err).ToNot(HaveOccurred())

			l.Announce(ctx, 1*time.Second, func() { l.Add("foo", map[string]interface{}{"bar": "baz"}) })

			Eventually(func() string {
				var s string
				v, exists := l2.GetKey("foo", "bar")
				if exists {
					v.Unmarshal(&s)
				}
				return s
			}, 100*time.Second, 1*time.Second).Should(Equal("baz"))
		})
	})

	Context("connection gater", func() {
		It("blacklists", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e, _ := New(
				WithBlacklist("1.1.1.1/32", "1.1.1.0/24"),
				FromBase64(true, true, token),
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
