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

package node_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	"github.com/marcopolo/simnet"
	ma "github.com/multiformats/go-multiaddr"
)

// The simulated network has a relay with a public address, peer A behind a
// router that blocks incoming connections, and peer B that starts on a
// Wi-Fi address A can reach and then moves to a mobile address.
var (
	simRelayAddr   = ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1")
	simAAddr       = ma.StringCast("/ip4/2.2.0.1/udp/8000/quic-v1")
	simBWifiAddr   = ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1")
	simBMobileAddr = ma.StringCast("/ip4/3.3.0.1/udp/9000/quic-v1")

	simBWifiUDP   = &net.UDPAddr{IP: net.ParseIP("2.2.0.2"), Port: 8001}
	simBMobileUDP = &net.UDPAddr{IP: net.ParseIP("3.3.0.1"), Port: 9000}
)

type simSourceIP struct{ ip atomic.Pointer[net.IP] }

func (s *simSourceIP) PreferredSourceIPForDestination(*net.UDPAddr) (net.IP, error) {
	return *s.ip.Load(), nil
}

// simLatency is the default one-way delay of every simulated packet. The
// watcher races libp2p's hole puncher, which waits for identify over the
// relayed connection. With this delay the watcher wins, as on real relayed
// paths with tens of milliseconds of round trip. With noLatency, identify
// finishes first.
const (
	simLatency = 10 * time.Millisecond
	noLatency  = 0
)

// delayed hands each packet to the router after latency.
type delayed struct {
	r       simnet.PacketReceiver
	latency time.Duration
}

func (d delayed) RecvPacket(p *simnet.Packet) {
	time.AfterFunc(d.latency, func() { d.r.RecvPacket(p) })
}

// simQUIC puts the host's QUIC sockets on the simulated network. public
// decides which of the addresses the host listens on accept incoming
// connections from anyone.
func simQUIC(router *simnet.SimpleFirewallRouter, latency time.Duration, public func(*net.UDPAddr) bool) libp2p.Option {
	src := &simSourceIP{}
	return libp2p.QUICReuse(
		quicreuse.NewConnManager,
		quicreuse.OverrideSourceIPSelector(func() (quicreuse.SourceIPSelector, error) { return src, nil }),
		quicreuse.OverrideListenUDP(func(_ string, addr *net.UDPAddr) (net.PacketConn, error) {
			src.ip.Store(&addr.IP)
			if public(addr) {
				router.SetAddrPubliclyReachable(addr)
			}
			c := simnet.NewSimConn(addr)
			c.SetUpPacketReceiver(delayed{r: router, latency: latency})
			router.AddNode(addr, c)
			return c, nil
		}))
}

func newSimHost(t *testing.T, opts ...libp2p.Option) host.Host {
	t.Helper()
	h, err := libp2p.New(append([]libp2p.Option{
		libp2p.ResourceManager(&network.NullResourceManager{}),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
	}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

// holePunchLog records the hole punch events of one host.
type holePunchLog struct {
	mu     sync.Mutex
	events []*holepunch.Event
}

func (l *holePunchLog) Trace(e *holepunch.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *holePunchLog) succeeded() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.events {
		if end, ok := e.Evt.(*holepunch.EndHolePunchEvt); ok && end.Success {
			return true
		}
	}
	return false
}

func (l *holePunchLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.events {
		out = append(out, fmt.Sprintf("%s %+v", e.Type, e.Evt))
	}
	return fmt.Sprint(out)
}

type simHandover struct {
	router     *simnet.SimpleFirewallRouter
	relay      host.Host
	relayInfo  peer.AddrInfo
	a, b       host.Host
	aHolePunch *holePunchLog
}

// newSimHandover builds the network and connects A and B directly over B's
// Wi-Fi address. A holds a relay reservation so B can reach it through the
// relay later. mobilePublic decides whether B's mobile address accepts
// incoming connections, and latency is the one-way delay of every packet.
// Both peers run libp2p's hole puncher with a short direct dial timeout so
// that a failed direct dial does not delay the hole punch.
func newSimHandover(t *testing.T, ctx context.Context, mobilePublic bool, latency time.Duration) *simHandover {
	t.Helper()
	router := &simnet.SimpleFirewallRouter{}
	every := func(*net.UDPAddr) bool { return true }
	none := func(*net.UDPAddr) bool { return false }
	bPublic := func(a *net.UDPAddr) bool {
		return a.IP.Equal(simBWifiUDP.IP) || (mobilePublic && a.IP.Equal(simBMobileUDP.IP))
	}
	aHolePunch := &holePunchLog{}

	relay := newSimHost(t,
		simQUIC(router, latency, every),
		libp2p.ListenAddrs(simRelayAddr),
		libp2p.DisableRelay(),
		libp2p.ForceReachabilityPublic(),
	)
	if _, err := relayv2.New(relay); err != nil {
		t.Fatal(err)
	}
	a := newSimHost(t,
		simQUIC(router, latency, none),
		libp2p.ListenAddrs(simAAddr),
		libp2p.ForceReachabilityPrivate(),
		libp2p.EnableHolePunching(holepunch.DirectDialTimeout(100*time.Millisecond), holepunch.WithTracer(aHolePunch)),
	)
	b := newSimHost(t,
		simQUIC(router, latency, bPublic),
		libp2p.ListenAddrs(simBWifiAddr),
		libp2p.ForceReachabilityPrivate(),
		libp2p.EnableHolePunching(holepunch.DirectDialTimeout(100*time.Millisecond)),
	)
	n := &simHandover{
		router:     router,
		relay:      relay,
		relayInfo:  peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()},
		a:          a,
		b:          b,
		aHolePunch: aHolePunch,
	}

	if err := a.Connect(ctx, n.relayInfo); err != nil {
		t.Fatal(err)
	}
	if _, err := relayclient.Reserve(ctx, a, n.relayInfo); err != nil {
		t.Fatal(err)
	}
	if err := b.Connect(ctx, n.relayInfo); err != nil {
		t.Fatal(err)
	}
	if err := a.Connect(ctx, peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
		t.Fatal(err)
	}
	if !n.aHasDirectConnOn(simBWifiAddr) {
		t.Fatalf("no direct connection on the Wi-Fi address; A conns: %v", simConns(a, b.ID()))
	}
	return n
}

// switchB moves B from Wi-Fi to the mobile address. Packets from and to the
// Wi-Fi address are lost from now on, so A's connection on it is dead but A
// does not know yet. B closes its own connections on the Wi-Fi address and
// redials A through the relay. B dials only the relay address: a dial to
// A's direct address would open B's firewall for A's packets and let A in
// without a hole punch.
func (n *simHandover) switchB(t *testing.T, ctx context.Context) {
	t.Helper()
	n.router.RemoveNode(simBWifiUDP)
	if err := n.b.Network().Listen(simBMobileAddr); err != nil {
		t.Fatal(err)
	}
	n.b.Network().(*swarm.Swarm).ListenClose(simBWifiAddr)
	for _, c := range n.b.Network().Conns() {
		if c.LocalMultiaddr().Equal(simBWifiAddr) {
			c.Close()
		}
	}
	if err := n.b.Connect(ctx, n.relayInfo); err != nil {
		t.Fatalf("B reconnect to relay: %v", err)
	}

	circuit := ma.StringCast("/p2p/" + n.relay.ID().String() + "/p2p-circuit")
	var aAddrs []ma.Multiaddr
	for _, a := range n.relay.Addrs() {
		aAddrs = append(aAddrs, a.Encapsulate(circuit))
	}
	n.b.Peerstore().ClearAddrs(n.a.ID())
	n.b.Peerstore().AddAddrs(n.a.ID(), aAddrs, time.Hour)
	if err := n.b.Connect(network.WithDialPeerTimeout(ctx, 10*time.Second), peer.AddrInfo{ID: n.a.ID()}); err != nil {
		t.Fatalf("B redial A: %v", err)
	}
	waitFor(t, 2*time.Second, n.aHasRelayedConn, func() string {
		return fmt.Sprintf("A has no relayed connection to B; A conns: %v", simConns(n.a, n.b.ID()))
	})
}

func (n *simHandover) aHasDirectConnOn(addr ma.Multiaddr) bool {
	for _, c := range n.a.Network().ConnsToPeer(n.b.ID()) {
		if !c.Stat().Limited && c.RemoteMultiaddr().Equal(addr) {
			return true
		}
	}
	return false
}

func (n *simHandover) aHasRelayedConn() bool {
	for _, c := range n.a.Network().ConnsToPeer(n.b.ID()) {
		if c.Stat().Limited {
			return true
		}
	}
	return false
}

// waitFor polls cond until it holds or timeout passes. It returns how long
// the wait took and fails the test with describe's text on timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, describe func() string) time.Duration {
	t.Helper()
	start := time.Now()
	for !cond() {
		if time.Since(start) > timeout {
			t.Fatalf("after %s: %s", timeout, describe())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return time.Since(start)
}

func simConns(h host.Host, p peer.ID) []string {
	var out []string
	for _, c := range h.Network().ConnsToPeer(p) {
		s := c.RemoteMultiaddr().String()
		if c.Stat().Limited {
			s += " (relayed)"
		}
		out = append(out, s)
	}
	return out
}
