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

package discovery

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/crypto"
	coreDiscovery "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// A network key rotates its meeting point every six minutes by default
// (edgevpn -g uses a 360 second interval).
const testInterval = 360

func rotatingDHT(now time.Time) *DHT {
	d := NewDHT()
	d.OTPKey = "a shared secret from the network key"
	d.OTPInterval = testInterval
	d.KeyLength = 12
	d.clock = func() time.Time { return now }
	return d
}

// windowStart is the start of a rendezvous window, far enough from 1970 to
// look like a real date.
var windowStart = time.Unix(1_800_000_000/testInterval*testInterval, 0)

func TestTheMeetingPointChangesAtEachWindow(t *testing.T) {
	d := rotatingDHT(windowStart)
	a := d.rendezvousAt(windowStart)
	if b := d.rendezvousAt(windowStart.Add(testInterval*time.Second - time.Second)); b != a {
		t.Fatalf("the meeting point changed inside one window: %s, then %s", a, b)
	}
	if c := d.rendezvousAt(windowStart.Add(testInterval * time.Second)); c == a {
		t.Fatal("the meeting point stayed the same in the next window")
	}
	if d.Rendezvous() != a {
		t.Fatalf("Rendezvous() = %s, want the meeting point of the current window, %s", d.Rendezvous(), a)
	}
}

func TestARoundUsesTheCurrentMeetingPointFirst(t *testing.T) {
	window := testInterval * time.Second
	d := rotatingDHT(windowStart)
	previous := d.rendezvousAt(windowStart.Add(-window))
	current := d.rendezvousAt(windowStart)
	next := d.rendezvousAt(windowStart.Add(window))

	for _, c := range []struct {
		name string
		at   time.Time
		want []string
	}{
		{"in the middle of a window", windowStart.Add(window / 2), []string{current}},
		// Nodes whose clocks are a little ahead already meet at the next
		// point, and the next point is there before the switch.
		{"shortly before the switch", windowStart.Add(window - 30*time.Second), []string{current, next}},
		// Nodes whose clocks are a little behind still meet at the
		// previous point.
		{"shortly after the switch", windowStart.Add(30 * time.Second), []string{current, previous}},
	} {
		got := d.rendezvousKeys(c.at)
		if len(got) != len(c.want) {
			t.Errorf("%s: meeting points %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: meeting points %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// GenerateNewConnectionData uses 9000 seconds when it gets no interval, and
// 9000 does not divide a day, so its windows do not line up with days.
func TestWindowsFollowUnixTimeForAnyInterval(t *testing.T) {
	const interval = 9000
	window := interval * time.Second
	start := time.Unix(1_800_000_000/interval*interval, 0)
	d := NewDHT()
	d.OTPKey = "a shared secret from the network key"
	d.OTPInterval = interval
	d.KeyLength = 12

	current := d.rendezvousAt(start)
	next := d.rendezvousAt(start.Add(window))
	if current == next {
		t.Fatal("the meeting point did not change at the window boundary")
	}
	if got := d.rendezvousKeys(start.Add(window - 30*time.Second)); len(got) != 2 || got[0] != current || got[1] != next {
		t.Fatalf("shortly before the switch the meeting points are %v, want %v", got, []string{current, next})
	}
	if got := d.rendezvousKeys(start.Add(window / 2)); len(got) != 1 {
		t.Fatalf("in the middle of the window the meeting points are %v, want only the current one", got)
	}
	now := start.Add(window / 2)
	wait, ok := d.untilNextRound(now)
	if !ok {
		t.Fatal("no round planned before the switch")
	}
	at := now.Add(wait)
	if !at.Before(start.Add(window)) || start.Add(window).Sub(at) > rendezvousOverlap {
		t.Fatalf("the round before the switch runs at %s, want within %s before the switch at %s", at, rendezvousOverlap, start.Add(window))
	}
}

func TestAFixedMeetingPointNeverChanges(t *testing.T) {
	d := NewDHT()
	d.RendezvousString = "fixed"
	d.clock = func() time.Time { return windowStart.Add(10 * time.Second) }
	if got := d.rendezvousKeys(d.clock()); len(got) != 1 || got[0] != "fixed" {
		t.Fatalf("meeting points %v, want only the fixed one", got)
	}
	if _, ok := d.untilNextRound(d.clock()); ok {
		t.Fatal("a fixed meeting point asked for a round at a switch that never happens")
	}
}

func TestTheNodeWakesUpBeforeTheMeetingPointChanges(t *testing.T) {
	window := testInterval * time.Second
	now := windowStart.Add(window / 2)
	d := rotatingDHT(now)

	wait, ok := d.untilNextRound(now)
	if !ok {
		t.Fatal("no round planned before the switch")
	}
	at := now.Add(wait)
	switchAt := windowStart.Add(window)
	if !at.Before(switchAt) {
		t.Fatalf("the round runs at %s, after the switch at %s", at, switchAt)
	}
	keys := d.rendezvousKeys(at)
	if len(keys) < 2 || keys[1] != d.rendezvousAt(switchAt) {
		t.Fatalf("the round before the switch uses %v, which leaves out the next meeting point", keys)
	}
}

// fakeRouting stands in for the DHT. Advertise blocks for the meeting points
// in stuck, and FindPeers keeps its channel open for the ones in endless,
// until the context ends, which is how a slow DHT query looks.
type fakeRouting struct {
	stuck      map[string]bool
	endless    map[string]bool
	found      map[string][]peer.AddrInfo
	advertised chan string
	// handedOut, when set, gets the meeting point once every peer found
	// there has been handed to the search.
	handedOut chan string
}

func (f *fakeRouting) Advertise(ctx context.Context, ns string, _ ...coreDiscovery.Option) (time.Duration, error) {
	if f.stuck[ns] {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	f.advertised <- ns
	return time.Hour, nil
}

func (f *fakeRouting) FindPeers(ctx context.Context, ns string, _ ...coreDiscovery.Option) (<-chan peer.AddrInfo, error) {
	ch := make(chan peer.AddrInfo)
	go func() {
		defer close(ch)
		for _, p := range f.found[ns] {
			select {
			case ch <- p:
			case <-ctx.Done():
				return
			}
		}
		if f.handedOut != nil {
			f.handedOut <- ns
		}
		if f.endless[ns] {
			<-ctx.Done()
		}
	}()
	return ch, nil
}

// newPeer returns a peer with a throwaway identity and a documentation
// address that nothing listens on.
func newPeer(t *testing.T) peer.AddrInfo {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/192.0.2.1/udp/4001/quic-v1")}}
}

func TestASlowMeetingPointDoesNotDelayTheOthers(t *testing.T) {
	d := rotatingDHT(windowStart.Add(30 * time.Second))
	keys := d.rendezvousKeys(d.clock())
	current, previous := keys[0], keys[1]
	routing := &fakeRouting{stuck: map[string]bool{current: true}, advertised: make(chan string, 4)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.round(log.Logger("test"), ctx, routing, func(context.Context, peer.AddrInfo) error { return nil }, func(peer.ID) bool { return false }, newPeer(t).ID)

	select {
	case got := <-routing.advertised:
		if got != previous {
			t.Fatalf("advertised %s, want the previous meeting point %s", got, previous)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the previous meeting point waited for the slow current one")
	}
}

func TestARoundDoesNotWaitForSearchesOrDials(t *testing.T) {
	d := rotatingDHT(windowStart.Add(testInterval * time.Second / 2))
	current := d.rendezvousKeys(d.clock())[0]
	dead, live := newPeer(t), newPeer(t)
	routing := &fakeRouting{
		// The search finds a peer that no longer exists before the one that
		// is there, and then never finishes.
		stuck:      map[string]bool{current: false},
		found:      map[string][]peer.AddrInfo{current: {dead, live}},
		endless:    map[string]bool{current: true},
		advertised: make(chan string, 4),
	}

	connected := make(chan peer.ID, 2)
	connect := func(ctx context.Context, p peer.AddrInfo) error {
		if p.ID == dead.ID {
			<-ctx.Done()
			return ctx.Err()
		}
		connected <- p.ID
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		d.round(log.Logger("test"), ctx, routing, connect, func(peer.ID) bool { return false }, newPeer(t).ID)
		close(done)
	}()

	select {
	case got := <-connected:
		if got != live.ID {
			t.Fatalf("connected to %s, want %s", got, live.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the live peer waited for the dial to the dead one")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the round waited for its dials to finish")
	}
}

func TestARoundSkipsItselfAndPeersItIsConnectedTo(t *testing.T) {
	d := rotatingDHT(windowStart.Add(testInterval * time.Second / 2))
	current := d.rendezvousKeys(d.clock())[0]
	self, known := newPeer(t), newPeer(t)
	routing := &fakeRouting{
		found:      map[string][]peer.AddrInfo{current: {self, known}},
		advertised: make(chan string, 4),
		handedOut:  make(chan string, 4),
	}
	var mu sync.Mutex
	var dialed []peer.ID
	connect := func(_ context.Context, p peer.AddrInfo) error {
		mu.Lock()
		dialed = append(dialed, p.ID)
		mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.round(log.Logger("test"), ctx, routing, connect, func(id peer.ID) bool { return id == known.ID }, self.ID)
	select {
	case <-routing.handedOut:
	case <-time.After(2 * time.Second):
		t.Fatal("the search never handed out its peers")
	}
	// A dial would start right after its peer is handed out.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(dialed) != 0 {
		t.Fatalf("dialed %v, want no dials", dialed)
	}
}
