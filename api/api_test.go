// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package api_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/go-log"
	. "github.com/mudler/edgevpn/api"
	client "github.com/mudler/edgevpn/api/client"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("API", func() {

	Context("Binds on socket", func() {
		It("sets data to the API", func() {
			d, _ := ioutil.TempDir("", "xxx")
			defer os.RemoveAll(d)
			os.MkdirAll(d, os.ModePerm)
			socket := filepath.Join(d, "socket")

			c := client.NewClient(client.WithHost("unix://" + socket))

			token := node.GenerateNewConnectionData().Base64()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			l := node.Logger(logger.New(log.LevelFatal))

			e, _ := node.New(node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)
			e.Start(ctx)

			go func() {
				err := API(ctx, fmt.Sprintf("unix://%s", socket), 10*time.Second, 20*time.Second, e)
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() error {
				return c.Put("b", "f", "bar")
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			Eventually(c.GetBuckets, 100*time.Second, 1*time.Second).Should(ContainElement("b"))

			Eventually(func() string {
				d, err := c.GetBucketKey("b", "f")
				if err != nil {
					fmt.Println(err)
				}
				var s string

				d.Unmarshal(&s)
				return s
			}, 10*time.Second, 1*time.Second).Should(Equal("bar"))
		})
	})
})
