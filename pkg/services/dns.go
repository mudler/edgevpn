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
	"regexp"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
	"github.com/pkg/errors"
)

func DNSNetworkService(listenAddr string, forwarder bool, forward []string, cacheSize int) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
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
	}
}

// DNS returns a network service binding a dns blockchain resolver on listenAddr.
// Takes an associated name for the addresses in the blockchain
func DNS(listenAddr string, forwarder bool, forward []string, cacheSize int) []node.Option {
	return []node.Option{
		node.WithNetworkService(DNSNetworkService(listenAddr, forwarder, forward, cacheSize)),
	}
}

// PersistDNSRecord is syntatic sugar around the ledger
// It persists a DNS record to the blockchain until it sees it reconciled.
// It automatically stop announcing and it is not *guaranteed* to persist data.
func PersistDNSRecord(ctx context.Context, b *blockchain.Ledger, announcetime, timeout time.Duration, regex string, record types.DNS) {
	b.Persist(ctx, announcetime, timeout, protocol.DNSKey, regex, record)
}

// AnnounceDNSRecord is syntatic sugar around the ledger
// Announces a DNS record binding to the blockchain, and keeps announcing for the ctx lifecycle
func AnnounceDNSRecord(ctx context.Context, b *blockchain.Ledger, announcetime time.Duration, regex string, record types.DNS) {
	b.AnnounceUpdate(ctx, announcetime, protocol.DNSKey, regex, record)
}

type dnsHandler struct {
	ctx       context.Context
	b         *blockchain.Ledger
	forwarder bool
	forward   []string
	cache     *lru.Cache
}

func (d dnsHandler) parseQuery(m *dns.Msg, forward bool) *dns.Msg {
	response := m.Copy()
	if len(m.Question) > 0 {
		q := m.Question[0]
		// Resolve the entry to an IP from the blockchain data
		for k, v := range d.b.CurrentData()[protocol.DNSKey] {
			r, err := regexp.Compile(k)
			if err == nil && r.MatchString(q.Name) {
				var res types.DNS
				v.Unmarshal(&res)
				if val, exists := res[dns.Type(q.Qtype)]; exists {
					rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", q.Name, dns.TypeToString[q.Qtype], val))
					if err == nil {
						response.Answer = append(m.Answer, rr)
						return response
					}
				}
			}
		}
		if forward {
			r, err := d.forwardQuery(m)
			if err == nil {
				response.Answer = r.Answer
			}
		}
	}
	return response
}

func (d dnsHandler) handleDNSRequest() func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Compress = false
		var resp *dns.Msg
		switch r.Opcode {
		case dns.OpcodeQuery:
			resp = d.parseQuery(r, d.forwarder)
		}

		w.WriteMsg(resp)
	}
}

func (d dnsHandler) forwardQuery(dnsMessage *dns.Msg) (*dns.Msg, error) {
	reqCopy := dnsMessage.Copy()
	if len(reqCopy.Question) > 0 {
		if v, ok := d.cache.Get(reqCopy.Question[0].String()); ok {
			q := v.(*dns.Msg)
			return q, nil
		}
	}
	for _, server := range d.forward {
		r, err := QueryDNS(d.ctx, reqCopy, server)
		if r != nil && len(r.Answer) == 0 && !r.MsgHdr.Truncated {
			continue
		}

		if err != nil {
			return nil, err
		}

		if r.Rcode == dns.RcodeSuccess {
			d.cache.Add(reqCopy.Question[0].String(), r)
		}

		if r == nil || r.Rcode == dns.RcodeNameError || r.Rcode == dns.RcodeSuccess || err == nil {
			return r, err
		}
	}
	return nil, errors.New("not available")
}

// QueryDNS queries a dns server with a dns message and return the answer
// it is blocking.
func QueryDNS(ctx context.Context, msg *dns.Msg, dnsServer string) (*dns.Msg, error) {
	client := &dns.Client{
		Net:            "udp",
		Timeout:        30 * time.Second,
		SingleInflight: true}
	r, _, err := client.Exchange(msg, dnsServer)
	return r, err
}
