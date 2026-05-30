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

package api_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
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

			e, _ := node.New(node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)
			e.Start(ctx)

			e2, _ := node.New(node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)
			e2.Start(ctx)

			go func() {
				err := API(ctx, fmt.Sprintf("unix://%s", socket), 10*time.Second, 20*time.Second, e, nil, false)
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

		It("applies hardened permissions to the socket file", func() {
			d, _ := ioutil.TempDir("", "xxx-perm")
			defer os.RemoveAll(d)
			socket := filepath.Join(d, "socket")

			token := node.GenerateNewConnectionData().Base64()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			l := node.Logger(logger.New(log.LevelFatal))
			e, _ := node.New(node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)
			e.Start(ctx)

			go func() {
				_ = API(ctx, "unix://"+socket, 10*time.Second, 20*time.Second, e, nil, false)
			}()

			// Wait for the socket to actually appear before asserting on perms.
			Eventually(func() error {
				_, err := os.Stat(socket)
				return err
			}, 5*time.Second, 100*time.Millisecond).ShouldNot(HaveOccurred())

			fi, err := os.Stat(socket)
			Expect(err).ToNot(HaveOccurred())
			// We must NOT be world-writable; that's the entire point of moving
			// off 127.0.0.1. Owner+group RW (0660) is the documented default.
			Expect(fi.Mode().Perm() & 0o002).To(Equal(os.FileMode(0)),
				"socket must not be world-writable, got mode %o", fi.Mode().Perm())
			Expect(fi.Mode()&os.ModeSocket).ToNot(Equal(os.FileMode(0)),
				"file must be a unix socket")
		})

		It("reaps a stale socket file from a previous run", func() {
			d, _ := ioutil.TempDir("", "xxx-stale")
			defer os.RemoveAll(d)
			socket := filepath.Join(d, "socket")

			// Simulate a crashed previous instance by binding+closing
			// a socket at the target path. net.Listen would otherwise
			// fail with "address already in use" on the second bind.
			pre, err := net.Listen("unix", socket)
			Expect(err).ToNot(HaveOccurred())
			pre.Close() // leaves the file on disk

			token := node.GenerateNewConnectionData().Base64()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			l := node.Logger(logger.New(log.LevelFatal))
			e, _ := node.New(node.FromBase64(true, true, token, nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)
			e.Start(ctx)

			started := make(chan error, 1)
			go func() {
				started <- API(ctx, "unix://"+socket, 10*time.Second, 20*time.Second, e, nil, false)
			}()

			c := client.NewClient(client.WithHost("unix://" + socket))
			// Successful Put proves the listener is up on the reused
			// path; checking err==nil is the cleanest signal because
			// GetBuckets returns an empty slice (not an error) when
			// the ledger has no entries yet.
			Eventually(func() error {
				return c.Put("stale", "key", "v")
			}, 5*time.Second, 100*time.Millisecond).ShouldNot(HaveOccurred())
			_ = started
		})
	})
})
