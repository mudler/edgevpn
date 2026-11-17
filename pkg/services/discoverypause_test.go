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

package services

import (
	"testing"
	"time"

	"github.com/ipfs/go-log"
)

func TestTheCheckInAgeCoversTwoLateCheckIns(t *testing.T) {
	for _, c := range []struct {
		interval, want time.Duration
	}{
		{0, pauseCheckInAge},
		{time.Minute, pauseCheckInAge},
		{2 * time.Minute, 6 * time.Minute},
		{4 * time.Minute, 12 * time.Minute},
	} {
		if got := checkInAgeFor(c.interval); got != c.want {
			t.Errorf("checkInAgeFor(%v) = %v, want %v", c.interval, got, c.want)
		}
	}
}

func TestEveryLivePeerConnected(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	live, stale := now.Add(-time.Minute), now.Add(-pauseCheckInAge-time.Second)
	for _, c := range []struct {
		name      string
		checkIns  map[string]time.Time
		connected map[string]bool
		want      bool
	}{
		{"no other peer", nil, nil, false},
		{"only this node", map[string]time.Time{"self": live}, map[string]bool{}, false},
		{"one live peer, connected", map[string]time.Time{"a": live}, map[string]bool{"a": true}, true},
		{"one live peer, not connected", map[string]time.Time{"a": live}, map[string]bool{}, false},
		{"a peer that is off does not count", map[string]time.Time{"a": live, "off": stale}, map[string]bool{"a": true}, true},
		{"only peers that are off", map[string]time.Time{"off": stale}, map[string]bool{}, false},
		{"two live peers, one connected", map[string]time.Time{"a": live, "b": live}, map[string]bool{"a": true}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := everyLivePeerConnected(c.checkIns, func(id string) bool { return c.connected[id] }, now, "self", pauseCheckInAge)
			if got != c.want {
				t.Fatalf("got %t, want %t", got, c.want)
			}
		})
	}
}

// pauseAt returns a DiscoveryPause on a clock the test moves, with the
// connection state the test sets.
func pauseAt(t *testing.T, start time.Time) (*DiscoveryPause, *time.Time, *bool) {
	t.Helper()
	now, connected := start, true
	p := newTestPause(t, func() time.Time { return now })
	p.setConnected(func() bool { return connected })
	return p, &now, &connected
}

// newTestPause returns a DiscoveryPause that waits 150 ms for the addresses
// to settle.
func newTestPause(t *testing.T, now func() time.Time) *DiscoveryPause {
	t.Helper()
	p := newDiscoveryPause(log.Logger("test"), now, 150*time.Millisecond)
	t.Cleanup(p.addrs.stop)
	return p
}

// woke reports whether a wake arrives within the given time. With zero it
// only checks for a wake already queued.
func woke(p *DiscoveryPause, within time.Duration) bool {
	if within == 0 {
		select {
		case <-p.Wake():
			return true
		default:
			return false
		}
	}
	select {
	case <-p.Wake():
		return true
	case <-time.After(within):
		return false
	}
}

func TestDiscoveryPausesWhileEveryPeerIsConnected(t *testing.T) {
	p, now, connected := pauseAt(t, time.Unix(1_800_000_000, 0))
	if !p.Pause() {
		t.Fatal("not paused while connected to every peer")
	}
	*connected = false
	if p.Pause() {
		t.Fatal("paused while a peer has no connection")
	}
	*connected = true
	*now = now.Add(time.Minute)
	if !p.Pause() {
		t.Fatal("not paused again once every peer is connected")
	}
}

func TestDiscoveryRunsARoundAtLeastOnceAnHour(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	p, now, _ := pauseAt(t, start)
	*now = start.Add(pauseMax - time.Second)
	if !p.Pause() {
		t.Fatal("not paused before the cap")
	}
	*now = start.Add(pauseMax)
	if p.Pause() {
		t.Fatal("still paused an hour after the last round")
	}
	*now = start.Add(pauseMax + time.Minute)
	if !p.Pause() {
		t.Fatal("the cap counts from the start instead of the last round")
	}
}

func TestDiscoveryDoesNotPauseBeforeTheHostIsReady(t *testing.T) {
	p := newTestPause(t, time.Now)
	if p.Pause() {
		t.Fatal("paused without knowing the connections")
	}
}

func TestWakeUpNeverBlocks(t *testing.T) {
	p := newTestPause(t, time.Now)
	p.WakeUp("first")
	p.WakeUp("second")
	if !woke(p, 0) {
		t.Fatal("no wake queued")
	}
	if woke(p, 0) {
		t.Fatal("two wakes queued for one round")
	}
}

func TestAWakeCountsAsARound(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	p, now, _ := pauseAt(t, start)
	*now = start.Add(50 * time.Minute)
	p.WakeUp("network changed")
	*now = start.Add(pauseMax + time.Minute)
	if !p.Pause() {
		t.Fatal("the cap ignores the round a wake started")
	}
}

func TestTheWatcherWakesWhenAPeerLosesItsConnection(t *testing.T) {
	p, _, connected := pauseAt(t, time.Unix(1_800_000_000, 0))
	p.watch()
	if woke(p, 0) {
		t.Fatal("woke while connected to every peer")
	}
	*connected = false
	p.watch()
	if !woke(p, 0) {
		t.Fatal("no wake when a peer lost its connection")
	}
	p.watch()
	if woke(p, 0) {
		t.Fatal("woke again while nothing changed")
	}
}

func TestTheWatcherEnforcesTheHourCap(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	p, now, _ := pauseAt(t, start)
	p.watch()
	*now = start.Add(pauseMax - time.Second)
	p.watch()
	if woke(p, 0) {
		t.Fatal("woke before the cap")
	}
	*now = start.Add(pauseMax)
	p.watch()
	if !woke(p, 0) {
		t.Fatal("no round after an hour without one")
	}
}

func TestTheFirstAddressEventIsTheStateAtSubscription(t *testing.T) {
	p := newTestPause(t, time.Now)
	p.addressEvent(true)
	if woke(p, 400*time.Millisecond) {
		t.Fatal("a round for the addresses the node already had")
	}
	p.addressEvent(true)
	if !woke(p, 2*time.Second) {
		t.Fatal("no round after the addresses changed")
	}
}

func TestAddressChangesCloseTogetherStartOneRound(t *testing.T) {
	p := newTestPause(t, time.Now)
	p.addressEvent(true)
	for i := 0; i < 3; i++ {
		p.addressEvent(true)
		time.Sleep(5 * time.Millisecond)
	}
	if !woke(p, 2*time.Second) {
		t.Fatal("no round after the addresses changed")
	}
	if woke(p, 400*time.Millisecond) {
		t.Fatal("more than one round for changes close together")
	}
}

func TestAnAddressEventWithoutChangesStartsNoRound(t *testing.T) {
	p := newTestPause(t, time.Now)
	p.addressEvent(true)
	p.addressEvent(false)
	if woke(p, 400*time.Millisecond) {
		t.Fatal("a round for an event without changes")
	}
}

func TestACheckInSeenRecentlyCountsWhateverItsTimestamp(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	var c checkInClock
	// A node whose clock is 10 minutes behind writes a new check-in.
	behind := now.Add(-10 * time.Minute)
	c.observe(map[string]time.Time{"a": behind}, now.Add(-time.Hour))
	got := c.observe(map[string]time.Time{"a": behind.Add(2 * time.Minute)}, now)
	if !now.Equal(got["a"]) {
		t.Fatalf("a check-in that changed now counts as %v, want %v", got["a"], now)
	}
	// Ten minutes later without a new check-in, the time it was seen is
	// newer than its own timestamp, so that one counts.
	later := now.Add(10 * time.Minute)
	got = c.observe(map[string]time.Time{"a": behind.Add(2 * time.Minute)}, later)
	if !now.Equal(got["a"]) {
		t.Fatalf("an unchanged check-in counts as %v, want when it was seen, %v", got["a"], now)
	}
	// A node with a correct clock keeps its own timestamp.
	got = c.observe(map[string]time.Time{"b": later.Add(-time.Minute)}, later)
	if !later.Add(-time.Minute).Equal(got["b"]) {
		t.Fatalf("a current check-in counts as %v, want its timestamp", got["b"])
	}
}
