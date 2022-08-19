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
	"bufio"
	"context"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
)

func egressHandler(n *node.Node, b *blockchain.Ledger) func(stream network.Stream) {
	return func(stream network.Stream) {
		// Remember to close the stream when we are done.
		defer stream.Close()

		// Retrieve current ID for ip in the blockchain
		_, found := b.GetKey(protocol.UsersLedgerKey, stream.Conn().RemotePeer().String())
		// If mismatch, update the blockchain
		if !found {
			//		ll.Debugf("Reset '%s': not found in the ledger", stream.Conn().RemotePeer().String())
			stream.Reset()
			return
		}

		// Create a new buffered reader, as ReadRequest needs one.
		// The buffered reader reads from our stream, on which we
		// have sent the HTTP request (see ServeHTTP())
		buf := bufio.NewReader(stream)
		// Read the HTTP request from the buffer
		req, err := http.ReadRequest(buf)
		if err != nil {
			stream.Reset()
			log.Println(err)
			return
		}
		defer req.Body.Close()

		// We need to reset these fields in the request
		// URL as they are not maintained.
		req.URL.Scheme = "http"
		hp := strings.Split(req.Host, ":")
		if len(hp) > 1 && hp[1] == "443" {
			req.URL.Scheme = "https"
		} else {
			req.URL.Scheme = "http"
		}
		req.URL.Host = req.Host

		outreq := new(http.Request)
		*outreq = *req

		// We now make the request
		//fmt.Printf("Making request to %s\n", req.URL)
		resp, err := http.DefaultTransport.RoundTrip(outreq)
		if err != nil {
			stream.Reset()
			log.Println(err)
			return
		}

		// resp.Write writes whatever response we obtained for our
		// request back to the stream.
		resp.Write(stream)
	}
}

// ProxyService starts a local http proxy server which redirects requests to egresses into the network
// It takes a deadtime to consider hosts which are alive within a time window
func ProxyService(announceTime time.Duration, listenAddr string, deadtime time.Duration) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {

		ps := &proxyService{
			host:       n,
			listenAddr: listenAddr,
			deadTime:   deadtime,
		}

		// Announce ourselves so nodes accepts our connection
		b.Announce(
			ctx,
			announceTime,
			func() {
				// Retrieve current ID for ip in the blockchain
				_, found := b.GetKey(protocol.UsersLedgerKey, n.Host().ID().String())
				// If mismatch, update the blockchain
				if !found {
					updatedMap := map[string]interface{}{}
					updatedMap[n.Host().ID().String()] = &types.User{
						PeerID:    n.Host().ID().String(),
						Timestamp: time.Now().String(),
					}
					b.Add(protocol.UsersLedgerKey, updatedMap)
				}
			},
		)

		go ps.Serve()
		return nil
	}
}

type proxyService struct {
	host       *node.Node
	listenAddr string
	deadTime   time.Duration
}

func (p *proxyService) Serve() error {
	return http.ListenAndServe(p.listenAddr, p)
}

func (p *proxyService) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	l, err := p.host.Ledger()
	if err != nil {
		//fmt.Printf("no ledger")
		return
	}

	egress := l.CurrentData()[protocol.EgressService]
	nodes := AvailableNodes(l, p.deadTime)

	availableEgresses := []string{}
	for _, n := range nodes {
		for e := range egress {
			if e == n {
				availableEgresses = append(availableEgresses, e)
			}
		}
	}

	chosen := availableEgresses[rand.Intn(len(availableEgresses)-1)]

	//fmt.Printf("proxying request for %s to peer %s\n", r.URL, chosen)
	// We need to send the request to the remote libp2p peer, so
	// we open a stream to it
	stream, err := p.host.Host().NewStream(context.Background(), peer.ID(chosen), protocol.EgressProtocol.ID())
	// If an error happens, we write an error for response.
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	// r.Write() writes the HTTP request to the stream.
	err = r.Write(stream)
	if err != nil {
		stream.Reset()
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Now we read the response that was sent from the dest
	// peer
	buf := bufio.NewReader(stream)
	resp, err := http.ReadResponse(buf, r)
	if err != nil {
		stream.Reset()
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Copy any headers
	for k, v := range resp.Header {
		for _, s := range v {
			w.Header().Add(k, s)
		}
	}

	// Write response status and headers
	w.WriteHeader(resp.StatusCode)

	// Finally copy the body
	io.Copy(w, resp.Body)
	resp.Body.Close()
}

func EgressService(announceTime time.Duration) node.NetworkService {
	return func(ctx context.Context, c node.Config, n *node.Node, b *blockchain.Ledger) error {
		b.AnnounceUpdate(ctx, announceTime, protocol.EgressService, n.Host().ID().String(), "ok")
		return nil
	}
}

func Egress(announceTime time.Duration) []node.Option {
	return []node.Option{
		node.WithNetworkService(EgressService(announceTime)),
		node.WithStreamHandler(protocol.EgressProtocol, egressHandler),
	}
}

func Proxy(announceTime, deadtime time.Duration, listenAddr string) []node.Option {
	return []node.Option{
		node.WithNetworkService(ProxyService(announceTime, listenAddr, deadtime)),
	}
}
