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

package trustzone_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmanager "github.com/libp2p/go-libp2p/p2p/net/connmgr"
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
				node.FromBase64(true, true, token, nil, nil), node.WithStore(permStore), ll)

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
				node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), ll)

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
				node.FromBase64(true, true, token, nil, nil), node.WithStore(permStore), ll)

			Expect(err).ToNot(HaveOccurred())

			l, err = e.Ledger()
			Expect(err).ToNot(HaveOccurred())

			e.Start(ctx)

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
