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
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// A pubsub stream can be reset while the connection under it stays up, for
// example when a phone changes networks and only some of its streams survive.
// Pubsub then opens a new stream, and the other hub must keep hearing from
// this one on it.
func TestMessagesFlowAfterAPubsubStreamIsReset(t *testing.T) {
	a, b := newLocalHost(t), newLocalHost(t)
	if err := a.Connect(context.Background(), peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
		t.Fatal(err)
	}
	hubA := NewHub("key-0", 1<<20, 43, 1<<30, false)
	hubB := NewHub("key-0", 1<<20, 43, 1<<30, false)
	rotate(t, hubA, a, "key-0")
	rotate(t, hubB, b, "key-0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var heardByB lastHeard
	go func() {
		for {
			select {
			case <-hubB.Messages:
				heardByB.mu.Lock()
				heardByB.at = time.Now()
				heardByB.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-tick.C:
				hubA.PublishMessage(NewMessage(fmt.Sprint(i)))
			case <-ctx.Done():
				return
			}
		}
	}()

	heardWithin := func(after time.Time, timeout time.Duration) bool {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if heardByB.since(after) {
				return true
			}
			time.Sleep(50 * time.Millisecond)
		}
		return heardByB.since(after)
	}
	if !heardWithin(time.Now(), 3*time.Second) {
		t.Fatal("b did not hear a before the reset")
	}

	reset := 0
	for _, c := range a.Network().ConnsToPeer(b.ID()) {
		for _, s := range c.GetStreams() {
			if s.Stat().Direction == network.DirOutbound && strings.HasPrefix(string(s.Protocol()), "/meshsub/") {
				s.Reset()
				reset++
			}
		}
	}
	if reset == 0 {
		t.Fatal("found no outbound pubsub stream from a to b")
	}
	// Messages queued before the reset can still arrive just after it, so
	// only messages from a second later count.
	if !heardWithin(time.Now().Add(time.Second), 5*time.Second) {
		t.Fatalf("b heard nothing from a within 5s after a's pubsub stream to b was reset; a and b still connected: %v",
			a.Network().Connectedness(b.ID()) == network.Connected)
	}
}
