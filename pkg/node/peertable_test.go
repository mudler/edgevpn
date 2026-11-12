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
	"crypto/rand"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	. "github.com/mudler/edgevpn/pkg/node"
)

func newIdentity(t *testing.T) ([]byte, peer.ID) {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return b, id
}

// A static peer table is meant to keep every peer that is not on it out of
// a node's ledger. Here a and b both list only each other, and c holds the
// network token. c can reach a only through b, whose pubsub passes c's
// messages on to a before b's own peer table check drops them.
func TestPeersOutsideTheStaticPeerTableCannotWriteToTheLedgerThroughARelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token := GenerateNewConnectionData(25).Base64()
	keyA, idA := newIdentity(t)
	keyB, idB := newIdentity(t)
	keyC, idC := newIdentity(t)

	start := func(key []byte, extra ...Option) *Node {
		t.Helper()
		opts := append([]Option{
			FromBase64(false, false, token, nil, nil),
			WithStore(&blockchain.MemoryStore{}),
			Logger(logger.New(log.LevelFatal)),
			ListenAddresses("/ip4/127.0.0.1/tcp/0"),
			WithLedgerAnnounceTime(time.Second),
			WithPrivKey(key),
		}, extra...)
		n, err := New(opts...)
		if err != nil {
			t.Fatal(err)
		}
		if err := n.Start(ctx); err != nil {
			t.Fatal(err)
		}
		return n
	}
	table := []Option{WithStaticPeer("10.1.0.1", idA), WithStaticPeer("10.1.0.2", idB)}
	a := start(keyA, append(table, WithBlacklist(string(idC)))...)
	b := start(keyB, table...)
	c := start(keyC)

	for _, n := range []*Node{a, c} {
		if err := n.Host().Connect(ctx, peer.AddrInfo{ID: idB, Addrs: b.Host().Addrs()}); err != nil {
			t.Fatal(err)
		}
	}

	ledgerA, _ := a.Ledger()
	ledgerB, _ := b.Ledger()
	ledgerC, _ := c.Ledger()
	ledgerB.Announce(ctx, time.Second, func() { ledgerB.Add("test", map[string]interface{}{"from-b": "b"}) })
	ledgerC.Announce(ctx, time.Second, func() { ledgerC.Add("test", map[string]interface{}{"from-c": "c"}) })

	// b's entry reaching a shows that gossip flows from b to a.
	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, ok := ledgerA.GetKey("test", "from-b"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("b's entry never reached a")
		}
		time.Sleep(200 * time.Millisecond)
	}
	// c announces every second; give its entry several rounds to arrive.
	time.Sleep(10 * time.Second)

	if a.Host().Network().Connectedness(idC) == network.Connected {
		t.Fatal("a connected to c directly")
	}
	if _, ok := ledgerA.GetKey("test", "from-c"); ok {
		t.Error("c's entry reached a's ledger, although c is not on a's static peer table")
	}
	if _, ok := ledgerB.GetKey("test", "from-c"); ok {
		t.Error("c's entry reached b's ledger, although c is not on b's static peer table")
	}
}
