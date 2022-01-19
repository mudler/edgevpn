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

package vpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
	"github.com/pkg/errors"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"golang.org/x/net/ipv4"
)

// Start the node and the vpn. Returns an error in case of failure
// When starting the vpn, there is no need to start the node
func Register(p ...Option) ([]node.Option, error) {
	c := &Config{
		Concurrency:        1,
		LedgerAnnounceTime: 5 * time.Second,
		Timeout:            15 * time.Second,
		Logger:             logger.New(log.LevelDebug),
	}
	c.Apply(p...)

	ifce, err := createInterface(c)
	if err != nil {
		return nil, err
	}

	return []node.Option{
		node.WithStreamHandler(protocol.EdgeVPN,
			func(n *node.Node, l *blockchain.Ledger) func(stream network.Stream) {
				return streamHandler(l, ifce)
			},
		),
		node.WithNetworkService(func(ctx context.Context, nc node.Config, n *node.Node, b *blockchain.Ledger) error {
			defer ifce.Close()
			// Announce our IP
			ip, _, err := net.ParseCIDR(c.InterfaceAddress)
			if err != nil {
				return err
			}

			b.Announce(
				ctx,
				c.LedgerAnnounceTime,
				func() {
					machine := &types.Machine{}
					// Retrieve current ID for ip in the blockchain
					existingValue, found := b.GetKey(protocol.MachinesLedgerKey, ip.String())
					existingValue.Unmarshal(machine)

					// If mismatch, update the blockchain
					if !found || machine.PeerID != n.Host().ID().String() {
						updatedMap := map[string]interface{}{}
						updatedMap[ip.String()] = newBlockChainData(n, ip.String())
						b.Add(protocol.MachinesLedgerKey, updatedMap)
					}
				},
			)

			if c.NetLinkBootstrap {
				if err := prepareInterface(c); err != nil {
					return err
				}
			}

			// read packets from the interface
			return readPackets(ctx, c, n, b, ifce)
		}),
	}, nil

}

func streamHandler(l *blockchain.Ledger, ifce *water.Interface) func(stream network.Stream) {
	return func(stream network.Stream) {
		if !l.Exists(protocol.MachinesLedgerKey,
			func(d blockchain.Data) bool {
				machine := &types.Machine{}
				d.Unmarshal(machine)
				return machine.PeerID == stream.Conn().RemotePeer().String()
			}) {
			stream.Reset()
			return
		}
		io.Copy(ifce.ReadWriteCloser, stream)
		stream.Close()
	}
}

func newBlockChainData(n *node.Node, address string) types.Machine {
	hostname, _ := os.Hostname()

	return types.Machine{
		PeerID:   n.Host().ID().String(),
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  internal.Version,
		Address:  address,
	}
}

func getFrame(ifce *water.Interface, c *Config) (ethernet.Frame, error) {
	var frame ethernet.Frame
	frame.Resize(c.MTU)

	n, err := ifce.Read([]byte(frame))
	if err != nil {
		return frame, errors.Wrap(err, "could not read from interface")
	}

	frame = frame[:n]
	return frame, nil
}

func handleFrame(frame ethernet.Frame, c *Config, n *node.Node, ip net.IP, ledger *blockchain.Ledger, ifce *water.Interface) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	header, err := ipv4.ParseHeader(frame)
	if err != nil {
		return errors.Wrap(err, "could not parse ipv4 header from frame")
	}

	dst := header.Dst.String()
	if c.RouterAddress != "" && header.Src.Equal(ip) {
		dst = c.RouterAddress
	}

	// Query the routing table
	value, found := ledger.GetKey(protocol.MachinesLedgerKey, dst)
	if !found {
		return fmt.Errorf("'%s' not found in the routing table", dst)
	}
	machine := &types.Machine{}
	value.Unmarshal(machine)

	// Decode the Peer
	d, err := peer.Decode(machine.PeerID)
	if err != nil {
		return errors.Wrap(err, "could not decode peer")
	}

	// Open a stream
	stream, err := n.Host().NewStream(ctx, d, protocol.EdgeVPN.ID())
	if err != nil {
		return errors.Wrap(err, "could not open stream")
	}

	stream.Write(frame)
	stream.Close()
	return nil
}

func connectionWorker(
	p chan ethernet.Frame,
	c *Config,
	n *node.Node,
	ip net.IP,
	wg *sync.WaitGroup,
	ledger *blockchain.Ledger,
	ifce *water.Interface) {
	defer wg.Done()
	for f := range p {
		if err := handleFrame(f, c, n, ip, ledger, ifce); err != nil {
			c.Logger.Debugf("could not handle frame: %s", err.Error())
		}
	}
}

// redirects packets from the interface to the node using the routing table in the blockchain
func readPackets(ctx context.Context, c *Config, n *node.Node, ledger *blockchain.Ledger, ifce *water.Interface) error {
	ip, _, err := net.ParseCIDR(c.InterfaceAddress)
	if err != nil {
		return err
	}

	wg := new(sync.WaitGroup)

	packets := make(chan ethernet.Frame, c.ChannelBufferSize)

	defer func() {
		close(packets)
		wg.Wait()
	}()

	for i := 0; i < c.Concurrency; i++ {
		wg.Add(1)
		go connectionWorker(packets, c, n, ip, wg, ledger, ifce)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			frame, err := getFrame(ifce, c)
			if err != nil {
				c.Logger.Errorf("could not get frame '%s'", err.Error())
				continue
			}

			packets <- frame
		}
	}
}
