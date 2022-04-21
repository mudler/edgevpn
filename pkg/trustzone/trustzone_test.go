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

package trustzone_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmanager "github.com/libp2p/go-libp2p-connmgr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/trustzone"
	. "github.com/mudler/edgevpn/pkg/trustzone"
	. "github.com/mudler/edgevpn/pkg/trustzone/authprovider/ecdsa"
)

var _ = Describe("trustzone", func() {
	token := node.GenerateNewConnectionData().Base64()

	logg := logger.New(log.LevelDebug)
	ll := node.Logger(logg)

	Context("ECDSA auth", func() {
		It("authorize nodes", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ctx2, cancel2 := context.WithCancel(context.Background())
			defer cancel2()

			privKey, pubKey, err := GenerateKeys()
			Expect(err).ToNot(HaveOccurred())

			pg := NewPeerGater(false)
			dur := 5 * time.Second
			provider, err := ECDSA521Provider(logg, string(privKey))
			aps := []trustzone.AuthProvider{provider}

			pguardian := trustzone.NewPeerGuardian(logg, aps...)

			cm, err := connmanager.NewConnManager(
				1,
				5,
				connmanager.WithGracePeriod(80*time.Second),
			)

			permStore := &blockchain.MemoryStore{}

			e, _ := node.New(
				node.WithLibp2pAdditionalOptions(libp2p.ConnectionManager(cm)),
				node.WithNetworkService(
					pg.UpdaterService(dur),
					pguardian.Challenger(dur, false),
				),
				node.EnableGenericHub,
				node.GenericChannelHandlers(pguardian.ReceiveMessage),
				//	node.WithPeerGater(pg),
				node.WithDiscoveryInterval(10*time.Second),
				node.FromBase64(true, true, token), node.WithStore(permStore), ll)

			pguardian2 := trustzone.NewPeerGuardian(logg, aps...)

			e2, _ := node.New(
				node.WithLibp2pAdditionalOptions(libp2p.ConnectionManager(cm)),

				node.WithNetworkService(
					pg.UpdaterService(dur),
					pguardian2.Challenger(dur, false),
				),
				node.EnableGenericHub,
				node.GenericChannelHandlers(pguardian2.ReceiveMessage),
				//	node.WithPeerGater(pg),
				node.WithDiscoveryInterval(10*time.Second),
				node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), ll)

			l, err := e.Ledger()
			Expect(err).ToNot(HaveOccurred())

			l2, err := e2.Ledger()
			Expect(err).ToNot(HaveOccurred())

			go e.Start(ctx2)

			time.Sleep(10 * time.Second)
			go e2.Start(ctx)

			l.Persist(ctx, 2*time.Second, 20*time.Second, protocol.TrustZoneAuthKey, "ecdsa", string(pubKey))

			Eventually(func() bool {
				_, exists := l2.GetKey(protocol.TrustZoneAuthKey, "ecdsa")
				fmt.Println("Ledger2", l2.CurrentData())
				fmt.Println("Ledger1", l.CurrentData())
				return exists
			}, 60*time.Second, 1*time.Second).Should(BeTrue())

			Eventually(func() bool {
				_, exists := l2.GetKey(protocol.TrustZoneKey, e.Host().ID().String())
				fmt.Println("Ledger2", l2.CurrentData())
				fmt.Println("Ledger1", l.CurrentData())
				return exists
			}, 60*time.Second, 1*time.Second).Should(BeTrue())

			Eventually(func() bool {
				_, exists := l.GetKey(protocol.TrustZoneKey, e2.Host().ID().String())
				fmt.Println("Ledger2", l2.CurrentData())
				fmt.Println("Ledger1", l.CurrentData())
				return exists
			}, 60*time.Second, 1*time.Second).Should(BeTrue())

			cancel2()

			e, err = node.New(
				node.WithLibp2pAdditionalOptions(libp2p.ConnectionManager(cm)),
				node.WithNetworkService(
					pg.UpdaterService(dur),
					pguardian.Challenger(dur, false),
				),
				node.EnableGenericHub,
				node.GenericChannelHandlers(pguardian.ReceiveMessage),
				node.WithPeerGater(pg),
				node.WithDiscoveryInterval(10*time.Second),
				node.FromBase64(true, true, token), node.WithStore(permStore), ll)

			Expect(err).ToNot(HaveOccurred())

			l, err = e.Ledger()
			Expect(err).ToNot(HaveOccurred())

			go e.Start(ctx)

			Eventually(func() bool {
				if e.Host() == nil {
					return false
				}
				_, exists := l2.GetKey(protocol.TrustZoneKey, e.Host().ID().String())
				fmt.Println("Ledger2", l2.CurrentData())
				fmt.Println("Ledger1", l.CurrentData())
				return exists
			}, 60*time.Second, 1*time.Second).Should(BeTrue())

			Eventually(func() bool {
				_, exists := l.GetKey(protocol.TrustZoneKey, e.Host().ID().String())
				fmt.Println("Ledger2", l2.CurrentData())
				fmt.Println("Ledger1", l.CurrentData())
				return exists
			}, 60*time.Second, 1*time.Second).Should(BeTrue())
		})
	})
})
