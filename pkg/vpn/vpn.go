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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"

	"github.com/pkg/errors"
	"github.com/songgao/packets/ethernet"
)

// packetSender delivers one packet to a peer.
type packetSender interface {
	send(ctx context.Context, pid peer.ID, frame []byte) error
}

func VPNNetworkService(p ...Option) node.NetworkService {
	return func(ctx context.Context, nc node.Config, n *node.Node, b *blockchain.Ledger) error {
		c := &Config{
			Concurrency:        1,
			LedgerAnnounceTime: 5 * time.Second,
			Timeout:            15 * time.Second,
			Logger:             logger.New(log.LevelDebug),
			MaxStreams:         30,
		}
		if err := c.Apply(p...); err != nil {
			return err
		}

		ifce := c.Interface
		if ifce == nil {
			created, err := createInterface(c)
			if err != nil {
				return err
			}
			ifce = created
		}
		defer ifce.Close()

		// Peers on the framed protocol get one stream each, kept open.
		ls := newLinks(n.Host())
		ls.debug = c.Logger.Debugf
		go logDrops(ctx, ls, c)
		go func() {
			<-ctx.Done()
			ls.close()
		}()

		// Set stream handler during runtime
		n.Host().SetStreamHandler(protocol.EdgeVPN.ID(), streamHandler(b, ifce, c, nc))
		n.Host().SetStreamHandler(protocol.EdgeVPNFramed.ID(), framedStreamHandler(func(p peer.ID) bool { return peerAllowed(b, nc, p) }, ifce, recheckAllowed))

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
		return readPackets(ctx, ls, c, n, b, ifce, nc)
	}
}

// Start the node and the vpn. Returns an error in case of failure
// When starting the vpn, there is no need to start the node
func Register(p ...Option) ([]node.Option, error) {
	return []node.Option{node.WithNetworkService(VPNNetworkService(p...))}, nil
}

// peerAllowed reports whether VPN packets from the peer are accepted: it must
// be in the peer table when there is one, or own a machine in the ledger.
func peerAllowed(l *blockchain.Ledger, nc node.Config, remote peer.ID) bool {
	if len(nc.PeerTable) > 0 {
		for _, p := range nc.PeerTable {
			if p.String() == remote.String() {
				return true
			}
		}
		return false
	}
	return l.Exists(protocol.MachinesLedgerKey,
		func(d blockchain.Data) bool {
			machine := &types.Machine{}
			d.Unmarshal(machine)
			return machine.PeerID == remote.String()
		})
}

// framedStreamHandler writes the packets of a framed stream to ifce while its
// sender is allowed, checking again every recheck while the stream stays
// open.
func framedStreamHandler(allowed func(peer.ID) bool, ifce io.Writer, recheck time.Duration) func(stream network.Stream) {
	return func(stream network.Stream) {
		pid := stream.Conn().RemotePeer()
		if !allowed(pid) {
			stream.Reset()
			return
		}
		s := &recheckedStream{rawStream: stream, allowed: func() bool { return allowed(pid) }, every: recheck, checked: time.Now()}
		if err := receiveFramed(s, ifce); err != nil {
			stream.Reset()
			return
		}
		stream.Close()
	}
}

// logDrops logs, every 10 seconds, the packets dropped per peer because more
// was waiting in the stream than the path carries.
func logDrops(ctx context.Context, ls *links, c *Config) {
	last := map[peer.ID]uint64{}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		for pid, n := range ls.dropped() {
			if n > last[pid] {
				c.Logger.Debugf("dropped %d packets to %s in the last 10s: more was waiting than the path carries", n-last[pid], pid)
			}
			last[pid] = n
		}
	}
}

func streamHandler(l *blockchain.Ledger, ifce io.ReadWriteCloser, c *Config, nc node.Config) func(stream network.Stream) {
	return func(stream network.Stream) {
		if !peerAllowed(l, nc, stream.Conn().RemotePeer()) {
			stream.Reset()
			return
		}
		_, err := io.Copy(ifce, stream)
		if err != nil {
			stream.Reset()
		}
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

func getFrame(ifce io.Reader, c *Config) (ethernet.Frame, error) {
	var frame ethernet.Frame
	frame.Resize(c.MTU)

	n, err := ifce.Read([]byte(frame))
	if err != nil {
		return frame, errors.Wrap(err, "could not read from interface")
	}

	frame = frame[:n]
	return frame, nil
}

func handleFrame(sender packetSender, frame ethernet.Frame, c *Config, n *node.Node, ip net.IP, ledger *blockchain.Ledger, ifce io.ReadWriteCloser, nc node.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	var dstIP, srcIP net.IP
	var packet layers.IPv4
	if err := packet.DecodeFromBytes(frame, gopacket.NilDecodeFeedback); err != nil {
		var packet layers.IPv6
		if err := packet.DecodeFromBytes(frame, gopacket.NilDecodeFeedback); err != nil {
			return errors.Wrap(err, "could not parse header from frame")
		} else {
			dstIP = packet.DstIP
			srcIP = packet.SrcIP
		}
	} else {
		dstIP = packet.DstIP
		srcIP = packet.SrcIP
	}

	dst := dstIP.String()
	if c.RouterAddress != "" && srcIP.Equal(ip) {
		if _, found := ledger.GetKey(protocol.MachinesLedgerKey, dst); !found {
			dst = c.RouterAddress
		}
	}

	var d peer.ID
	var err error
	notFoundErr := fmt.Errorf("'%s' not found in the routing table", dst)
	if len(nc.PeerTable) > 0 {
		found := false
		for ip, p := range nc.PeerTable {
			if ip == dst {
				found = true
				d = peer.ID(p)
			}
		}
		if !found {
			return notFoundErr
		}
	} else {
		// Query the routing table
		value, found := ledger.GetKey(protocol.MachinesLedgerKey, dst)
		if !found {
			return notFoundErr
		}
		machine := &types.Machine{}
		value.Unmarshal(machine)

		// Don't route to an owner that has gone inactive (under ownership
		// enforcement); this is a no-op when enforcement is disabled.
		if !ledger.IsOwnerLive(machine.PeerID) {
			return notFoundErr
		}

		// Decode the Peer
		d, err = peer.Decode(machine.PeerID)
	}

	if err != nil {
		return errors.Wrap(err, "could not decode peer")
	}

	return sender.send(ctx, d, frame)
}

func connectionWorker(
	p chan ethernet.Frame,
	sender packetSender,
	c *Config,
	n *node.Node,
	ip net.IP,
	wg *sync.WaitGroup,
	ledger *blockchain.Ledger,
	ifce io.ReadWriteCloser,
	nc node.Config) {
	defer wg.Done()
	for f := range p {
		if err := handleFrame(sender, f, c, n, ip, ledger, ifce, nc); err != nil {
			c.Logger.Debugf("could not handle frame: %s", err.Error())
		}
	}
}

// readPackets packets from the interface to the node using the routing table in the blockchain
func readPackets(ctx context.Context, sender packetSender, c *Config, n *node.Node, ledger *blockchain.Ledger, ifce io.ReadWriteCloser, nc node.Config) error {
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
		go connectionWorker(packets, sender, c, n, ip, wg, ledger, ifce, nc)
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
