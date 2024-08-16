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
	"github.com/mudler/edgevpn/pkg/stream"
	"github.com/mudler/edgevpn/pkg/types"

	"github.com/mudler/water"
	"github.com/pkg/errors"
	"github.com/songgao/packets/ethernet"
)

type streamManager interface {
	Connected(n network.Network, c network.Stream)
	Disconnected(n network.Network, c network.Stream)
	HasStream(n network.Network, pid peer.ID) (network.Stream, error)
	Close() error
}

func VPNNetworkService(p ...Option) node.NetworkService {
	return func(ctx context.Context, nc node.Config, n *node.Node, b *blockchain.Ledger) error {
		c := &Config{
			Concurrency:        1,
			LedgerAnnounceTime: 5 * time.Second,
			Timeout:            120 * time.Second,
			Logger:             logger.New(log.LevelDebug),
			MaxStreams:         30,
		}
		if err := c.Apply(p...); err != nil {
			return err
		}

		ifce, err := createInterface(c)
		if err != nil {
			return err
		}
		defer ifce.Close()

		var mgr streamManager

		if c.lowProfile {
			// Create stream manager for outgoing connections
			mgr, err = stream.NewConnManager(10, c.MaxStreams)
			if err != nil {
				return err
			}
			// Attach it to the same context
			go func() {
				<-ctx.Done()
				mgr.Close()
			}()
		}

		// Set stream handler during runtime
		n.Host().SetStreamHandler(protocol.EdgeVPN.ID(), streamHandler(b, ifce, c, nc))

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
		return readPackets(ctx, mgr, c, n, b, ifce, nc)
	}
}

// Start the node and the vpn. Returns an error in case of failure
// When starting the vpn, there is no need to start the node
func Register(p ...Option) ([]node.Option, error) {
	return []node.Option{node.WithNetworkService(VPNNetworkService(p...))}, nil
}

func streamHandler(l *blockchain.Ledger, ifce *water.Interface, c *Config, nc node.Config) func(stream network.Stream) {
	return func(stream network.Stream) {
		if len(nc.PeerTable) == 0 && !l.Exists(protocol.MachinesLedgerKey,
			func(d blockchain.Data) bool {
				machine := &types.Machine{}
				d.Unmarshal(machine)
				return machine.PeerID == stream.Conn().RemotePeer().String()
			}) {
			stream.Reset()
			return
		}
		if len(nc.PeerTable) > 0 {
			found := false
			for _, p := range nc.PeerTable {
				if p.String() == stream.Conn().RemotePeer().String() {
					found = true
				}
			}
			if !found {
				stream.Reset()
				return
			}
		}
		_, err := io.Copy(ifce.ReadWriteCloser, stream)
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

func handleFrame(mgr streamManager, frame ethernet.Frame, c *Config, n *node.Node, ip net.IP, ledger *blockchain.Ledger, ifce *water.Interface, nc node.Config) error {
	t := time.Now().Add(c.Timeout)
	ctx, cancel := context.WithDeadline(context.Background(), t)
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

		// Decode the Peer
		d, err = peer.Decode(machine.PeerID)
	}

	if err != nil {
		return errors.Wrap(err, "could not decode peer")
	}

	var stream network.Stream
	if mgr != nil {
		// Open a stream if necessary
		stream, err = mgr.HasStream(n.Host().Network(), d)
		if err == nil {
			_, err = stream.Write(frame)
			if err == nil {
				return nil
			}
			mgr.Disconnected(n.Host().Network(), stream)
		}
	}

	stream, err = n.Host().NewStream(ctx, d, protocol.EdgeVPN.ID())
	if err != nil {
		return fmt.Errorf("could not open stream to %s: %w", d.String(), err)
	}
	defer stream.Close()

	if mgr != nil {
		mgr.Connected(n.Host().Network(), stream)
	}

	_, err = stream.Write(frame)
	return err
}

func connectionWorker(
	p chan ethernet.Frame,
	mgr streamManager,
	c *Config,
	n *node.Node,
	ip net.IP,
	wg *sync.WaitGroup,
	ledger *blockchain.Ledger,
	ifce *water.Interface,
	nc node.Config) {
	defer wg.Done()
	for f := range p {
		if err := handleFrame(mgr, f, c, n, ip, ledger, ifce, nc); err != nil {
			c.Logger.Debugf("could not handle frame: %s", err.Error())
		}
	}
}

// redirects packets from the interface to the node using the routing table in the blockchain
func readPackets(ctx context.Context, mgr streamManager, c *Config, n *node.Node, ledger *blockchain.Ledger, ifce *water.Interface, nc node.Config) error {
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
		go connectionWorker(packets, mgr, c, n, ip, wg, ledger, ifce, nc)
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
