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
			node.FromBase64(true, true, token,nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)...)

	Context("DNS service", func() {
		It("Set DNS records and can resolve IPs", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := DNS(logg, "127.0.0.1:19192", true, []string{"8.8.8.8:53"}, 10)
			opts = append(opts, node.FromBase64(true, true, token,nil, nil), node.WithStore(&blockchain.MemoryStore{}), l)
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
