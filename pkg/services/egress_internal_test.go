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

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestEgressPeerID guards against decoding the peer IDs announced in the
// egress ledger bucket with a plain peer.ID() conversion: the bucket keys are
// the base58 representation (peer.ID.String()), while peer.ID itself holds the
// raw multihash, so a plain conversion yields a completely different - and
// unreachable - peer.
func TestEgressPeerID(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	// This is what an egress node writes into the ledger.
	announced := id.String()

	decoded, err := egressPeerID(announced)
	if err != nil {
		t.Fatalf("decoding announced peer id %q: %v", announced, err)
	}
	if decoded != id {
		t.Fatalf("expected %q, got %q", id, decoded)
	}
}

func TestEgressPeerIDInvalid(t *testing.T) {
	if _, err := egressPeerID("not-a-peer-id"); err == nil {
		t.Fatal("expected an error decoding an invalid peer id")
	}
}

// TestProxyServiceListenReportsBindErrors makes sure the proxy's listener is
// bound where the caller can see the failure. Binding in the background and
// discarding the error leaves the command running with nothing listening.
func TestProxyServiceListenReportsBindErrors(t *testing.T) {
	free := &proxyService{listenAddr: "127.0.0.1:0"}
	l, err := free.listen()
	if err != nil {
		t.Fatalf("binding an ephemeral port: %v", err)
	}
	defer l.Close()

	taken := &proxyService{listenAddr: l.Addr().String()}
	second, err := taken.listen()
	if err == nil {
		second.Close()
		t.Fatalf("expected binding %q twice to fail", l.Addr())
	}
}

func TestPickEgressNoEgresses(t *testing.T) {
	chosen, ok := pickEgress(nil)
	if ok {
		t.Fatalf("expected no egress to be picked, got %q", chosen)
	}

	chosen, ok = pickEgress([]string{})
	if ok {
		t.Fatalf("expected no egress to be picked from an empty slice, got %q", chosen)
	}
}

func TestPickEgressSingleEgress(t *testing.T) {
	for i := 0; i < 100; i++ {
		chosen, ok := pickEgress([]string{"peerA"})
		if !ok {
			t.Fatal("expected an egress to be picked when one is available")
		}
		if chosen != "peerA" {
			t.Fatalf("expected peerA, got %q", chosen)
		}
	}
}

// TestPickEgressCoversAllElements makes sure every available egress can be
// selected - in particular the last one of the slice.
func TestPickEgressCoversAllElements(t *testing.T) {
	for _, available := range [][]string{
		{"peerA", "peerB"},
		{"peerA", "peerB", "peerC", "peerD", "peerE"},
	} {
		seen := map[string]bool{}
		for i := 0; i < 1000; i++ {
			chosen, ok := pickEgress(available)
			if !ok {
				t.Fatalf("expected an egress to be picked out of %v", available)
			}
			seen[chosen] = true
		}

		for _, e := range available {
			if !seen[e] {
				t.Errorf("egress %q was never selected out of %v", e, available)
			}
		}
		if len(seen) != len(available) {
			t.Errorf("expected to select %d distinct egresses out of %v, selected %d", len(available), available, len(seen))
		}
	}
}
