// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package services

import (
	"context"
	"fmt"
	"net"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/pkg/errors"
)

const (
	DNSKey string = "dns"
)

// DNS returns a network service binding a dns blockchain resolver on listenAddr.
// Takes an associated name for the addresses in the blockchain
func DNS(listenAddr string, forwarder bool, forward []string, cacheSize int) []node.Option {
	return []node.Option{
		node.WithNetworkService(
			func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {

				server := &dns.Server{Addr: listenAddr, Net: "udp"}
				cache, err := lru.New(cacheSize)
				if err != nil {
					return err
				}
				go func() {
					dns.HandleFunc(".", dnsHandler{ctx, b, forwarder, forward, cache}.handleDNSRequest())
					fmt.Println(server.ListenAndServe())
				}()

				go func() {
					<-ctx.Done()
					server.Shutdown()
				}()

				return nil
			},
		),
	}
}

func AnnounceDomain(ctx context.Context, b *blockchain.Ledger, announcetime, timeout time.Duration, record, ip string) {
	b.Announce(ctx, announcetime, func() {
		b.Add(DNSKey, map[string]interface{}{fmt.Sprintf("%s.", record): ip})
	})
}

type dnsHandler struct {
	ctx       context.Context
	b         *blockchain.Ledger
	forwarder bool
	forward   []string
	cache     *lru.Cache
}

func (d dnsHandler) parseQuery(m *dns.Msg) {
	for _, q := range m.Question {
		// Resolve the entry to an IP from the blockchain data
		switch q.Qtype {
		case dns.TypeA, dns.TypeAAAA:
			if v, exists := d.b.GetKey(DNSKey, q.Name); exists {
				var res string
				v.Unmarshal(&res)
				rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", q.Name, dns.TypeToString[q.Qtype], res))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			} else if d.forwarder {
				r, err := d.forwardQuery(m)
				if err == nil {
					m.Answer = r.Answer
				}
			}
		}
	}
}

func (d dnsHandler) handleDNSRequest() func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Compress = false

		switch r.Opcode {
		case dns.OpcodeQuery:
			d.parseQuery(m)
		}

		w.WriteMsg(m)
	}
}

func (d dnsHandler) forwardQuery(dnsMessage *dns.Msg) (*dns.Msg, error) {
	mess := new(dns.Msg)
	mess.Question = dnsMessage.Copy().Question
	if len(mess.Question) > 0 {
		if v, ok := d.cache.Get(mess.Question[0].String()); ok {
			q := v.(*dns.Msg)
			return q, nil
		}
	}

	for _, server := range d.forward {
		r, err := QueryDNS(d.ctx, mess, server)
		if err != nil {
			return nil, err
		}
		if r == nil || r.Rcode == dns.RcodeNameError || r.Rcode == dns.RcodeSuccess {
			d.cache.Add(mess.Question[0].String(), r)
			return r, err
		}
	}
	return nil, errors.New("not available")
}

// Queries a dns server with a dns message
func QueryDNS(ctx context.Context, msg *dns.Msg, dnsServer string) (*dns.Msg, error) {
	c := new(dns.Conn)
	cc, _ := (&net.Dialer{Timeout: 35 * time.Second}).DialContext(ctx, "udp", dnsServer)
	c.Conn = cc
	defer c.Close()

	err := c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return nil, err
	}
	err = c.WriteMsg(msg)
	if err != nil {
		return nil, err
	}
	err = c.SetReadDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return nil, err
	}
	return c.ReadMsg()
}
