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

package hub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

func newLocalHost(t *testing.T) host.Host {
	t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

// lastHeard records when a hub last received a message from the other one.
type lastHeard struct {
	mu sync.Mutex
	at time.Time
}

func (l *lastHeard) since(t time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.at.After(t)
}

// rotate switches h to the room of a new key, as Start does when the TOTP
// window changes.
func rotate(t *testing.T, m *MessageHub, h host.Host, key string) {
	t.Helper()
	m.Lock()
	m.otpKey = key
	m.Unlock()
	if err := m.joinRoom(h); err != nil {
		t.Fatal(err)
	}
}

// Two devices never switch rooms at the same instant: each polls its clock
// once a second, and their clocks differ. The hub that switches second must
// still hear, and be heard by, the one that switched first.
func TestMessagesFlowAfterHubsSwitchRoomsAtDifferentTimes(t *testing.T) {
	a, b := newLocalHost(t), newLocalHost(t)
	if err := a.Connect(context.Background(), peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
		t.Fatal(err)
	}
	// An interval this long keeps the TOTP window fixed, so the room changes
	// only when the test changes the key.
	hubA := NewHub("key-0", 1<<20, 43, 1<<30, false)
	hubB := NewHub("key-0", 1<<20, 43, 1<<30, false)
	rotate(t, hubA, a, "key-0")
	rotate(t, hubB, b, "key-0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var heardByA, heardByB lastHeard
	listen := func(m *MessageHub, l *lastHeard) {
		for {
			select {
			case <-m.Messages:
				l.mu.Lock()
				l.at = time.Now()
				l.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}
	go listen(hubA, &heardByA)
	go listen(hubB, &heardByB)
	send := func(m *MessageHub) {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-tick.C:
				m.PublishMessage(NewMessage(fmt.Sprint(i)))
			case <-ctx.Done():
				return
			}
		}
	}
	go send(hubA)
	go send(hubB)

	waitBothWays := func(after time.Time, what string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if heardByA.since(after) && heardByB.since(after) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s: a heard b: %v, b heard a: %v, within 3s", what, heardByA.since(after), heardByB.since(after))
	}
	waitBothWays(time.Now(), "before any switch")

	for i := 1; i <= 4; i++ {
		first, second := hubA, hubB
		firstHost, secondHost := a, b
		if i%2 == 0 {
			first, second, firstHost, secondHost = hubB, hubA, b, a
		}
		key := fmt.Sprintf("key-%d", i)
		rotate(t, first, firstHost, key)
		time.Sleep(100 * time.Millisecond)
		rotate(t, second, secondHost, key)
		// Messages sent while the two are in different rooms are lost; give
		// the second hub's subscription a moment to reach the first.
		time.Sleep(500 * time.Millisecond)
		waitBothWays(time.Now(), fmt.Sprintf("after switch %d", i))
	}

	for name, m := range map[string]*MessageHub{"a": hubA, "b": hubB} {
		if topics := m.ps.GetTopics(); len(topics) != 1 {
			t.Errorf("hub %s holds %d topics after the switches, want only the current one", name, len(topics))
		}
	}
}

// A hub that cannot join the room of a new key keeps trying, instead of
// staying in the old room until the key changes again.
func TestHubRetriesARoomItCouldNotJoin(t *testing.T) {
	h := newLocalHost(t)
	m := NewHub("key-0", 1<<20, 43, 1<<30, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Start(ctx, h)

	room := func() string {
		m.Lock()
		defer m.Unlock()
		if m.blockchain == nil {
			return ""
		}
		return m.blockchain.roomName
	}
	waitForRoom := func(want string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for room() != want {
			if time.Now().After(deadline) {
				t.Fatalf("hub is in room %q, want %q", room(), want)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	waitForRoom(NewHub("key-0", 1<<20, 43, 1<<30, false).topicKey())
	old := room()

	// Joining the next room fails while its topic is already joined on the
	// hub's pubsub instance.
	next := NewHub("key-1", 1<<20, 43, 1<<30, false).topicKey()
	m.Lock()
	blocker, err := m.ps.Join(next)
	m.otpKey = "key-1"
	m.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	if got := room(); got != old {
		t.Fatalf("hub left room %q for %q while the join could not succeed", old, got)
	}

	if err := blocker.Close(); err != nil {
		t.Fatal(err)
	}
	waitForRoom(next)
}
