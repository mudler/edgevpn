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
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ipfs/go-log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	. "github.com/mudler/edgevpn/pkg/services"
)

func get(url string) string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 1 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return string(body)
}

var _ = Describe("Expose services", func() {
	token := node.GenerateNewConnectionData().Base64()

	logg := logger.New(log.LevelFatal)
	l := node.Logger(logg)

	e2 := node.New(node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)

	Context("Service sharing", func() {
		It("expose services and can connect to them", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serviceUUID := "test"

			opts := RegisterService(logg, 1*time.Second, serviceUUID, "142.250.184.35:80")
			opts = append(opts, node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)
			e := node.New(opts...)

			// First node expose a service
			// redirects to google:80

			e.Start(ctx)
			e2.Start(ctx)

			ll, _ := e2.Ledger()

			go ConnectToService(ctx, ll, e2, logg, 1*time.Second, serviceUUID, "127.0.0.1:9999")

			Eventually(func() string {
				return get("http://127.0.0.1:9999")
			}, 60*time.Second, 1*time.Second).Should(ContainSubstring("The document has moved"))
		})
	})
})
