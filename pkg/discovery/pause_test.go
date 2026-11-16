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
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

// loopUnderTest runs d.loop with ticks the test sends itself, and reports
// each round on the returned channel.
func loopUnderTest(t *testing.T, d *DHT) (chan<- time.Time, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ticks := make(chan time.Time)
	rounds := make(chan struct{}, 16)
	go d.loop(log.Logger("test"), ctx, ticks, func() { rounds <- struct{}{} })
	expectRounds(t, rounds, 1, "at start")
	return ticks, rounds
}

// expectRounds waits for n rounds, then checks that no other round follows
// right away.
func expectRounds(t *testing.T, rounds <-chan struct{}, n int, when string) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-rounds:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: got %d rounds, want %d", when, i, n)
		}
	}
	select {
	case <-rounds:
		t.Fatalf("%s: got more than %d rounds", when, n)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestEveryTickStartsARoundWithoutPause(t *testing.T) {
	ticks, rounds := loopUnderTest(t, NewDHT())
	ticks <- time.Now()
	ticks <- time.Now()
	expectRounds(t, rounds, 2, "after two ticks")
}

func TestAPausedNodeSkipsTheRegularRounds(t *testing.T) {
	d := NewDHT()
	d.Pause = func() bool { return true }
	ticks, rounds := loopUnderTest(t, d)
	ticks <- time.Now()
	ticks <- time.Now()
	expectRounds(t, rounds, 0, "after two ticks while paused")
}

func TestARoundRunsOnceThePauseEnds(t *testing.T) {
	answers := make(chan bool, 2)
	answers <- true
	answers <- false
	d := NewDHT()
	d.Pause = func() bool { return <-answers }
	ticks, rounds := loopUnderTest(t, d)
	ticks <- time.Now()
	ticks <- time.Now()
	expectRounds(t, rounds, 1, "after a tick while paused and one after")
}

func TestWakeStartsARoundWhilePaused(t *testing.T) {
	wake := make(chan struct{})
	d := NewDHT()
	d.Pause = func() bool { return true }
	d.Wake = wake
	_, rounds := loopUnderTest(t, d)
	wake <- struct{}{}
	expectRounds(t, rounds, 1, "after a wake")
}

func TestAClosedWakeStartsNoMoreRounds(t *testing.T) {
	wake := make(chan struct{})
	d := NewDHT()
	d.Wake = wake
	ticks, rounds := loopUnderTest(t, d)
	close(wake)
	expectRounds(t, rounds, 0, "after closing wake")
	ticks <- time.Now()
	expectRounds(t, rounds, 1, "after a tick")
}

// keyChangeSoon returns a rotating DHT whose round before the next switch of
// the meeting point is due half a second from now, well after loopUnderTest
// stops checking for extra rounds.
func keyChangeSoon() *DHT {
	window := testInterval * time.Second
	_, overlap := rotatingDHT(windowStart).window()
	return rotatingDHT(windowStart.Add(window - overlap/2 - 500*time.Millisecond))
}

func TestANodeAnnouncesBeforeTheMeetingPointSwitches(t *testing.T) {
	_, rounds := loopUnderTest(t, keyChangeSoon())
	select {
	case <-rounds:
	case <-time.After(2 * time.Second):
		t.Fatal("no round before the switch")
	}
}

func TestAPausedNodeSkipsTheRoundBeforeTheMeetingPointSwitches(t *testing.T) {
	asked := make(chan struct{}, 4)
	d := keyChangeSoon()
	d.Pause = func() bool { asked <- struct{}{}; return true }
	_, rounds := loopUnderTest(t, d)
	select {
	case <-asked:
	case <-time.After(2 * time.Second):
		t.Fatal("Pause was not asked before the switch")
	}
	expectRounds(t, rounds, 0, "before the switch while paused")
}

func TestTheDHTGetsTheOptionsItWasMadeWith(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewDHT(dht.Mode(dht.ModeServer))
	h, err := libp2p.New(libp2p.NoListenAddrs, d.Option(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if got := d.IpfsDHT.Mode(); got != dht.ModeServer {
		t.Fatalf("DHT mode = %v, want server", got)
	}
}
