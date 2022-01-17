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
	"io"
	"net"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	protocol "github.com/mudler/edgevpn/pkg/protocol"

	"github.com/mudler/edgevpn/pkg/types"
)

func ExposeService(ledger *blockchain.Ledger, node types.Node, l log.StandardLogger, announcetime time.Duration, serviceID, dstaddress string) {

	l.Infof("Exposing service '%s' (%s)", serviceID, dstaddress)

	// 1) Register the ServiceID <-> PeerID Association
	// By announcing periodically our service to the blockchain
	ledger.Announce(
		context.Background(),
		announcetime,
		func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(protocol.ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found || service.PeerID != node.Host().ID().String() {
				updatedMap := map[string]interface{}{}
				updatedMap[serviceID] = types.Service{PeerID: node.Host().ID().String(), Name: serviceID}
				ledger.Add(protocol.ServicesLedgerKey, updatedMap)
			}
		},
	)

	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	node.AddStreamHandler(protocol.ServiceProtocol, func(stream network.Stream) {
		go func() {
			l.Infof("(service %s) Received connection from %s", serviceID, stream.Conn().RemotePeer().String())

			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(protocol.UsersLedgerKey, stream.Conn().RemotePeer().String())
			// If mismatch, update the blockchain
			if !found {
				l.Debugf("Reset '%s': not found in the ledger", stream.Conn().RemotePeer().String())
				stream.Reset()
				return
			}

			l.Infof("Connecting to '%s'", dstaddress)
			c, err := net.Dial("tcp", dstaddress)
			if err != nil {
				l.Debugf("Reset %s: %s", stream.Conn().RemotePeer().String(), err.Error())
				stream.Reset()
				return
			}
			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, c)
			go copyStream(closer, c, stream)
			<-closer

			stream.Close()
			c.Close()

			l.Infof("(service %s) Handled correctly '%s'", serviceID, stream.Conn().RemotePeer().String())
		}()
	})
}

func ConnectToService(ledger *blockchain.Ledger, node types.Node, ll log.StandardLogger, announcetime time.Duration, serviceID string, srcaddr string) error {

	// Open local port for listening
	l, err := net.Listen("tcp", srcaddr)
	if err != nil {
		return err
	}
	ll.Info("Binding local port on", srcaddr)

	// Announce ourselves so nodes accepts our connection
	ledger.Announce(
		context.Background(),
		announcetime,
		func() {
			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(protocol.UsersLedgerKey, node.Host().ID().String())
			// If mismatch, update the blockchain
			if !found {
				updatedMap := map[string]interface{}{}
				updatedMap[node.Host().ID().String()] = &types.User{
					PeerID:    node.Host().ID().String(),
					Timestamp: time.Now().String(),
				}
				ledger.Add(protocol.UsersLedgerKey, updatedMap)
			}
		},
	)
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			ll.Error("Error accepting: ", err.Error())
			continue
		}

		ll.Info("New connection from", l.Addr().String())
		// Handle connections in a new goroutine, forwarding to the p2p service
		go func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(protocol.ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found {
				conn.Close()
				ll.Debugf("service '%s' not found on blockchain", serviceID)
				return
			}

			// Decode the Peer
			d, err := peer.Decode(service.PeerID)
			if err != nil {
				conn.Close()
				ll.Debugf("could not decode peer '%s'", service.PeerID)
				return
			}

			// Open a stream
			stream, err := node.Host().NewStream(context.Background(), d, protocol.ServiceProtocol.ID())
			if err != nil {
				conn.Close()
				ll.Debugf("could not open stream '%s'", err.Error())
				return
			}
			ll.Debugf("(service %s) Redirecting", serviceID, l.Addr().String())

			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, conn)
			go copyStream(closer, conn, stream)
			<-closer

			stream.Close()
			conn.Close()
			ll.Infof("(service %s) Done handling %s", serviceID, l.Addr().String())
		}()
	}
}

func copyStream(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}
