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
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/ipfs/go-log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	. "github.com/mudler/edgevpn/pkg/services"
)

var _ = Describe("File services", func() {
	token := node.GenerateNewConnectionData(25).Base64()

	logg := logger.New(log.LevelError)
	l := node.Logger(logg)

	e2, _ := node.New(
		node.WithDiscoveryInterval(10*time.Second),
		node.WithNetworkService(AliveNetworkService(2*time.Second, 4*time.Second, 15*time.Minute)),
		node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)

	Context("File sharing", func() {
		It("sends and receive files between two nodes", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fileUUID := "test"

			f, err := ioutil.TempFile("", "test")
			Expect(err).ToNot(HaveOccurred())

			defer os.RemoveAll(f.Name())

			ioutil.WriteFile(f.Name(), []byte("testfile"), os.ModePerm)

			// First node expose a file
			opts, err := ShareFile(logg, 10*time.Second, fileUUID, f.Name())
			Expect(err).ToNot(HaveOccurred())

			opts = append(opts, node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)
			e, _ := node.New(opts...)

			e.Start(ctx)
			e2.Start(ctx)

			Eventually(func() string {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				f, err := ioutil.TempFile("", "test")
				Expect(err).ToNot(HaveOccurred())

				defer os.RemoveAll(f.Name())

				ll, _ := e2.Ledger()
				ll1, _ := e.Ledger()
				By(fmt.Sprint(ll.CurrentData(), ll.LastBlock().Index, ll1.CurrentData()))
				ReceiveFile(ctx, ll, e2, logg, 2*time.Second, fileUUID, f.Name())
				b, _ := ioutil.ReadFile(f.Name())
				return string(b)
			}, 190*time.Second, 1*time.Second).Should(Equal("testfile"))
		})
	})
})
