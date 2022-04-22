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
	"time"

	"github.com/ipfs/go-log"
	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	. "github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/types"
)

var _ = Describe("DNS service", func() {
	token := node.GenerateNewConnectionData().Base64()

	logg := logger.New(log.LevelDebug)
	l := node.Logger(logg)

	e2, _ := node.New(
		append(Alive(15*time.Second, 90*time.Minute, 15*time.Minute),
			node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)...)

	Context("DNS service", func() {
		It("Set DNS records and can resolve IPs", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := DNS(logg, "127.0.0.1:19192", true, []string{"8.8.8.8:53"}, 10)
			opts = append(opts, node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)
			e, _ := node.New(opts...)

			e.Start(ctx)
			e2.Start(ctx)

			ll, _ := e2.Ledger()

			AnnounceDNSRecord(ctx, ll, 60*time.Second, `test.foo.`, types.DNS{
				dns.Type(dns.TypeA): "2.2.2.2",
			})

			searchDomain := func(d string) func() string {
				return func() string {
					var s string
					dnsMessage := new(dns.Msg)
					dnsMessage.SetQuestion(fmt.Sprintf("%s.", d), dns.TypeA)

					r, err := QueryDNS(ctx, dnsMessage, "127.0.0.1:19192")
					if r != nil {
						answers := r.Answer
						for _, a := range answers {

							s = a.String() + s
						}
					}
					if err != nil {
						fmt.Println(err)
					}
					return s
				}
			}

			Eventually(searchDomain("google.com"), 230*time.Second, 1*time.Second).Should(ContainSubstring("A"))
			// We hit the same record again, this time it's faster as there is a cache
			Eventually(searchDomain("google.com"), 1*time.Second, 1*time.Second).Should(ContainSubstring("A"))
			Eventually(searchDomain("test.foo"), 230*time.Second, 1*time.Second).Should(ContainSubstring("2.2.2.2"))
		})
	})
})
