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
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Gossip reaches most peers through other peers. The sender of a message is
// the peer that published it, whichever peer passed it on.
func TestTheSenderIsTheAuthorOfARelayedMessage(t *testing.T) {
	a, b, c := newLocalHost(t), newLocalHost(t), newLocalHost(t)
	// a and c are both connected to b, and not to each other.
	for _, h := range []host.Host{a, c} {
		if err := h.Connect(context.Background(), peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
			t.Fatal(err)
		}
	}
	hubs := map[peer.ID]*MessageHub{}
	for _, h := range []host.Host{a, b, c} {
		hubs[h.ID()] = NewHub("key", 1<<20, 43, 1<<30, false)
		rotate(t, hubs[h.ID()], h, "key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				hubs[c.ID()].PublishMessage(NewMessage("from c"))
			case <-ctx.Done():
				return
			}
		}
	}()

	var got *Message
	select {
	case got = <-hubs[a.ID()].Messages:
	case <-ctx.Done():
		t.Fatal("a never received c's message through b")
	}
	if a.Network().Connectedness(c.ID()) == network.Connected {
		t.Fatal("a connected to c directly, so the message may not have been relayed")
	}
	if got.SenderID != c.ID().String() {
		who := got.SenderID
		if who == b.ID().String() {
			who = "b, the peer that relayed it"
		}
		t.Errorf("sender of c's message is %s, want c (%s)", who, c.ID())
	}
}
