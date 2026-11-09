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
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/marcopolo/simnet"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
)

// testGrace is short so the tests do not wait for the default grace; each
// test lets the Wi-Fi connection age past it before B switches networks.
const testGrace = 100 * time.Millisecond

func TestStaleDirectConnReplacedAfterHandover(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true, simLatency)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))
	time.Sleep(2 * testGrace)

	n.switchB(t, ctx)

	took := waitFor(t, 5*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	t.Logf("direct connection on the mobile address after %s", took)
}

// Without packet delay, identify over the relayed connection finishes
// before the watcher closes the stale connection, so libp2p's hole puncher
// sees a direct connection and does nothing. The watcher's own dial brings
// the direct connection back.
func TestStaleDirectConnReplacedAfterHandoverNoLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true, noLatency)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))
	time.Sleep(2 * testGrace)

	n.switchB(t, ctx)

	took := waitFor(t, 5*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	t.Logf("direct connection on the mobile address after %s", took)
}

// When identify over the relayed connection fails, A learns nothing new
// about B, and libp2p's hole puncher, which runs before the watcher closes
// the stale connection, sees a direct connection and does nothing. The
// watcher dials anyway. On edgevpn's routed host a failed dial looks the
// peer up in the DHT; here the test puts B's mobile address in A's
// peerstore instead.
func TestStaleDirectConnReplacedWhenIdentifyFails(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true, noLatency)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))
	time.Sleep(2 * testGrace)
	// B answers no identify request, and A takes no identify push, which
	// would also count as a completed identify.
	n.b.RemoveStreamHandler(identify.ID)
	n.a.RemoveStreamHandler(identify.IDPush)
	n.a.Peerstore().AddAddr(n.b.ID(), simBMobileAddr, time.Hour)
	identifyFailed := identifyFailures(t, n.a)

	n.switchB(t, ctx)

	waitFor(t, 5*time.Second, func() bool { return identifyFailed.Load() > 0 }, func() string {
		return "identify over the relayed connection did not fail"
	})
	took := waitFor(t, 5*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	t.Logf("direct connection on the mobile address after %s", took)
}

// identifyFailures counts the failed identifies of h.
func identifyFailures(t *testing.T, h host.Host) *atomic.Int32 {
	t.Helper()
	sub, err := h.EventBus().Subscribe(new(event.EvtPeerIdentificationFailed))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sub.Close() })
	n := &atomic.Int32{}
	go func() {
		for range sub.Out() {
			n.Add(1)
		}
	}()
	return n
}

// Neither B's mobile address nor A's address accepts incoming connections,
// so the direct connection can only come back through libp2p's hole punch.
func TestStaleDirectConnReplacedByHolePunch(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, false, simLatency)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))
	time.Sleep(2 * testGrace)

	n.switchB(t, ctx)

	took := waitFor(t, 10*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	waitFor(t, time.Second, n.aHolePunch.succeeded, func() string {
		return fmt.Sprintf("the direct connection came back without a successful hole punch; A events: %s", n.aHolePunch)
	})
	t.Logf("hole-punched connection on the mobile address after %s", took)
}

func TestDirectConnYoungerThanGraceIsKept(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true, simLatency)
	watch(t, n.a, node.WithStaleConnsGrace(time.Hour))

	n.switchB(t, ctx)
	time.Sleep(time.Second)

	if !n.aHasDirectConnOn(simBWifiAddr) {
		t.Fatalf("A closed the Wi-Fi connection that is younger than grace; A conns: %v", simConns(n.a, n.b.ID()))
	}
}

// The fast path leaves a stale direct connection that is younger than
// grace. When it closes later, for example on the QUIC idle timeout, the
// watcher dials the peer directly on the addresses identify over the
// relayed connection announced.
func TestDirectDialWhenLastDirectConnCloses(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true, simLatency)
	watch(t, n.a, node.WithStaleConnsGrace(time.Hour))

	n.switchB(t, ctx)
	waitFor(t, 5*time.Second, n.aKnowsBMobileAddr, func() string {
		return fmt.Sprintf("identify did not give A B's mobile address; A knows: %v", n.a.Peerstore().Addrs(n.b.ID()))
	})
	if !n.aHasDirectConnOn(simBWifiAddr) {
		t.Fatalf("the fast path closed the Wi-Fi connection that is younger than grace; A conns: %v", simConns(n.a, n.b.ID()))
	}
	n.aCloseConnsOn(simBWifiAddr)

	took := waitFor(t, 5*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	t.Logf("direct connection on the mobile address after %s", took)
}

// A peer that A has no connection to at all is gone, so the watcher does
// not dial it.
func TestNoDialWhenLastConnOfAnyKindCloses(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true, simLatency)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))

	n.aCloseConnsOn(simBWifiAddr)
	time.Sleep(time.Second)

	if conns := simConns(n.a, n.b.ID()); len(conns) != 0 {
		t.Fatalf("A dialed B after its last connection to B closed; A conns: %v", conns)
	}
}

func watch(t *testing.T, h host.Host, opts ...node.StaleConnsOption) {
	t.Helper()
	w := node.WatchStaleDirectConns(h, opts...)
	t.Cleanup(func() { w.Close() })
}

// A node started with node.New watches its connections: when its last
// direct connection to a peer closes while a relayed one remains, the node
// dials the peer directly again. libp2p does not do that by itself.
func TestNodeRedialsPeerWhenOnlyRelayedConnectionRemains(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	router := &simnet.SimpleFirewallRouter{}
	every := func(*net.UDPAddr) bool { return true }
	relay := newSimHost(t,
		simQUIC(router, noLatency, every),
		libp2p.ListenAddrs(simRelayAddr),
		libp2p.DisableRelay(),
		libp2p.ForceReachabilityPublic(),
	)
	if _, err := relayv2.New(relay); err != nil {
		t.Fatal(err)
	}
	relayInfo := peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()}
	b := newSimHost(t,
		simQUIC(router, noLatency, every),
		libp2p.ListenAddrs(simBWifiAddr),
		libp2p.ForceReachabilityPrivate(),
	)
	if err := b.Connect(ctx, relayInfo); err != nil {
		t.Fatal(err)
	}
	if _, err := relayclient.Reserve(ctx, b, relayInfo); err != nil {
		t.Fatal(err)
	}

	n, err := node.New(
		node.FromBase64(true, true, node.GenerateNewConnectionData(25).Base64(), nil, nil),
		node.WithStore(&blockchain.MemoryStore{}),
		node.Logger(logger.New(log.LevelFatal)),
		node.WithLibp2pAdditionalOptions(
			simQUIC(router, noLatency, every),
			libp2p.NoTransports,
			libp2p.Transport(libp2pquic.NewTransport),
			libp2p.ListenAddrs(simAAddr),
			libp2p.ForceReachabilityPrivate(),
			libp2p.ResourceManager(&network.NullResourceManager{}),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(ctx); err != nil {
		t.Fatal(err)
	}
	a := n.Host()

	circuit := ma.StringCast("/p2p/" + relay.ID().String() + "/p2p-circuit/p2p/" + b.ID().String())
	var relayed []ma.Multiaddr
	for _, addr := range relay.Addrs() {
		relayed = append(relayed, addr.Encapsulate(circuit))
	}
	if err := a.Connect(network.WithAllowLimitedConn(ctx, "test"), peer.AddrInfo{ID: b.ID(), Addrs: relayed}); err != nil {
		t.Fatal(err)
	}
	if err := a.Connect(network.WithForceDirectDial(ctx, "test"), peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
		t.Fatal(err)
	}
	hasConn := func(limited bool) bool {
		for _, c := range a.Network().ConnsToPeer(b.ID()) {
			if c.Stat().Limited == limited {
				return true
			}
		}
		return false
	}
	waitFor(t, 5*time.Second, func() bool { return hasConn(true) && hasConn(false) }, func() string {
		return fmt.Sprintf("node needs a direct and a relayed connection; conns: %v", simConns(a, b.ID()))
	})

	for _, c := range a.Network().ConnsToPeer(b.ID()) {
		if !c.Stat().Limited {
			c.Close()
		}
	}
	waitFor(t, 10*time.Second, func() bool { return hasConn(false) }, func() string {
		return fmt.Sprintf("node did not dial B directly; conns: %v", simConns(a, b.ID()))
	})
}
