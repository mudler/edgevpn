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

package node

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type fakeConn struct {
	network.Conn
	stat   network.ConnStats
	remote ma.Multiaddr
}

func (c fakeConn) Stat() network.ConnStats       { return c.stat }
func (c fakeConn) RemoteMultiaddr() ma.Multiaddr { return c.remote }
func (c fakeConn) RemotePeer() peer.ID           { return "peer" }

// blockingConn is a direct connection whose Close blocks until release is
// closed.
type blockingConn struct {
	fakeConn
	entered chan struct{}
	release chan struct{}
}

func (c *blockingConn) Close() error {
	close(c.entered)
	<-c.release
	return nil
}

type fakeNetwork struct {
	network.Network
	conns []network.Conn
}

func (n fakeNetwork) ConnsToPeer(peer.ID) []network.Conn { return n.conns }

type fakeHost struct {
	host.Host
	net   fakeNetwork
	dials *atomic.Int32
}

func (h fakeHost) Network() network.Network { return h.net }

func (h fakeHost) Connect(ctx context.Context, _ peer.AddrInfo) error {
	if h.dials != nil {
		h.dials.Add(1)
	}
	return ctx.Err()
}

type fakeSub struct {
	event.Subscription
	out chan interface{}
}

func (s fakeSub) Out() <-chan interface{} { return s.out }

// A close of a stale connection that blocks must not hold the watcher's
// lock: handleIdentify takes the lock for every identify event, and while
// it waits, identify's event emitter backs up for the whole host.
func TestStaleConnCloseDoesNotHoldLock(t *testing.T) {
	circuit := ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1/p2p/12D3KooWFhw67EU1Th6CSFeQWVRnug3RLVB6LavsNsFrszgkNjzp/p2p-circuit")
	direct := ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1")
	now := time.Now()
	relayed := &fakeConn{stat: network.ConnStats{Stats: network.Stats{Limited: true, Opened: now}}, remote: circuit}

	for _, trigger := range []string{"connected", "identify"} {
		t.Run(trigger, func(t *testing.T) {
			stale := &blockingConn{
				fakeConn: fakeConn{stat: network.ConnStats{Stats: network.Stats{Opened: now.Add(-time.Hour)}}, remote: direct},
				entered:  make(chan struct{}),
				release:  make(chan struct{}),
			}
			sub := fakeSub{out: make(chan interface{}, 1)}
			w := newStaleConnsWatcher(fakeHost{net: fakeNetwork{conns: []network.Conn{stale, relayed}}})
			// The dial that follows a close fails at once on the cancelled context.
			w.cancel()
			defer func() {
				close(sub.out)
				w.wg.Wait()
			}()
			defer close(stale.release)

			if trigger == "connected" {
				w.Connected(nil, relayed)
			} else {
				w.sub = sub
				w.wg.Add(1)
				go w.handleIdentify()
				sub.out <- event.EvtPeerIdentificationCompleted{Peer: relayed.RemotePeer(), Conn: relayed}
			}
			select {
			case <-stale.entered:
			case <-time.After(time.Second):
				t.Fatal("the watcher did not close the stale connection")
			}

			locked := make(chan struct{})
			go func() {
				w.mu.Lock()
				w.mu.Unlock()
				close(locked)
			}()
			select {
			case <-locked:
			case <-time.After(time.Second):
				t.Fatal("the watcher holds its lock while a close runs")
			}
		})
	}
}

func TestIsRelayed(t *testing.T) {
	circuit := ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1/p2p/12D3KooWFhw67EU1Th6CSFeQWVRnug3RLVB6LavsNsFrszgkNjzp/p2p-circuit")
	direct := ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1")
	limited := network.ConnStats{Stats: network.Stats{Limited: true}}

	for _, tc := range []struct {
		name string
		conn fakeConn
		want bool
	}{
		{"direct", fakeConn{remote: direct}, false},
		{"limited circuit", fakeConn{stat: limited, remote: circuit}, true},
		{"unlimited circuit", fakeConn{remote: circuit}, true},
		{"limited without circuit address", fakeConn{stat: limited, remote: direct}, true},
	} {
		if got := isRelayed(tc.conn); got != tc.want {
			t.Errorf("%s: isRelayed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A failed dial on the routed host also runs a DHT lookup, and a forced
// direct dial skips the swarm's dial backoff, so the watcher limits its own
// dials to a peer.
func TestDialDirectCooldown(t *testing.T) {
	dials := &atomic.Int32{}
	w := newStaleConnsWatcher(fakeHost{dials: dials})
	defer w.cancel()

	w.dialDirect("peer")
	w.dialDirect("peer")
	if got := dials.Load(); got != 1 {
		t.Fatalf("two dials to the same peer within the cooldown made %d connects, want 1", got)
	}
	w.dialDirect("other")
	if got := dials.Load(); got != 2 {
		t.Fatalf("a dial to another peer made %d connects in total, want 2", got)
	}

	w.dialCooldown = 10 * time.Millisecond
	time.Sleep(2 * w.dialCooldown)
	w.dialDirect("peer")
	if got := dials.Load(); got != 3 {
		t.Fatalf("a dial after the cooldown made %d connects in total, want 3", got)
	}
}
