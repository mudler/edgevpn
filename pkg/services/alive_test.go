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

package services_test

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	. "github.com/mudler/edgevpn/pkg/services"
)

var _ = Describe("Alive service", func() {
	token := node.GenerateNewConnectionData(25).Base64()

	logg := logger.New(log.LevelError)
	l := node.Logger(logg)

	opts := append(
		Alive(5*time.Second, 100*time.Second, 15*time.Minute),
		node.WithDiscoveryInterval(10*time.Second),
		node.FromBase64(true, true, token),
		l)

	Context("Aliveness check", func() {
		It("detect both nodes alive after a while", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e2, _ := node.New(append(opts, node.WithStore(&blockchain.MemoryStore{}))...)
			e1, _ := node.New(append(opts, node.WithStore(&blockchain.MemoryStore{}))...)

			e1.Start(ctx)
			e2.Start(ctx)

			ll, _ := e1.Ledger()

			ll.Persist(ctx, 5*time.Second, 100*time.Second, "t", "t", "test")

			matches := And(ContainElement(e2.Host().ID().String()),
				ContainElement(e1.Host().ID().String()))

			index := ll.LastBlock().Index
			Eventually(func() []string {
				ll, err := e1.Ledger()
				if err != nil {
					return []string{}
				}
				return AvailableNodes(ll, 15*time.Minute)
			}, 100*time.Second, 1*time.Second).Should(matches)

			Expect(ll.LastBlock().Index).ToNot(Equal(index))
		})
	})

	Context("Aliveness Scrub", func() {
		BeforeEach(func() {
			opts = append(
				Alive(5*time.Second, 20*time.Second, 15*time.Minute),
				node.WithDiscoveryInterval(10*time.Second),
				node.FromBase64(true, true, token),
				l)
		})

		It("cleans up after a while", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e2, _ := node.New(append(opts, node.WithStore(&blockchain.MemoryStore{}))...)
			e1, _ := node.New(append(opts, node.WithStore(&blockchain.MemoryStore{}))...)

			e1.Start(ctx)
			e2.Start(ctx)

			ll, _ := e1.Ledger()

			ll.Persist(ctx, 5*time.Second, 100*time.Second, "t", "t", "test")

			matches := And(ContainElement(e2.Host().ID().String()),
				ContainElement(e1.Host().ID().String()))

			index := ll.LastBlock().Index
			Eventually(func() []string {
				ll, err := e1.Ledger()
				if err != nil {
					return []string{}
				}
				return AvailableNodes(ll, 15*time.Minute)
			}, 100*time.Second, 1*time.Second).Should(matches)

			Expect(ll.LastBlock().Index).ToNot(Equal(index))
			index = ll.LastBlock().Index

			Eventually(func() []string {
				ll, err := e1.Ledger()
				if err != nil {
					return []string{}
				}
				return AvailableNodes(ll, 15*time.Minute)
			}, 50*time.Second, 1*time.Second).Should(BeEmpty())

			Expect(ll.LastBlock().Index).ToNot(Equal(index))
			index = ll.LastBlock().Index

			Eventually(func() []string {
				ll, err := e1.Ledger()
				if err != nil {
					return []string{}
				}
				return AvailableNodes(ll, 15*time.Minute)
			}, 30*time.Second, 1*time.Second).Should(matches)
			Expect(ll.LastBlock().Index).ToNot(Equal(index))

		})
	})
})
