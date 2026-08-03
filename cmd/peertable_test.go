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

package cmd

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

// A real peer ID as an operator would paste it into --static-peertable.
const samplePeerID = "12D3KooWDgQ7VBLdrPZ8QqXAsBjbjBAmMuJmBHDGz2cCXVDLDGzZ"

// The entries have to decode to the very same peer.ID the rest of the stack
// compares against: pkg/vpn matches the table against
// stream.Conn().RemotePeer() and dials the stored ID directly. A raw
// peer.ID(string) conversion instead yields an ID made of the base58 text's
// bytes, which matches nothing and dials nowhere.
func TestParsePeerTableDecodesPeerIDs(t *testing.T) {
	want, err := peer.Decode(samplePeerID)
	if err != nil {
		t.Fatalf("test fixture is not a valid peer ID: %v", err)
	}

	table, err := parsePeerTable([]string{"10.1.0.1:" + samplePeerID})
	if err != nil {
		t.Fatalf("parsePeerTable() error = %v, want nil", err)
	}

	got, ok := table["10.1.0.1"]
	if !ok {
		t.Fatalf("parsePeerTable() = %v, want an entry for 10.1.0.1", table)
	}
	if got != want {
		t.Errorf("parsePeerTable() peer ID = %q, want %q", got, want)
	}
	// String() round-trips only if the ID holds decoded multihash bytes.
	if got.String() != samplePeerID {
		t.Errorf("peer ID String() = %q, want %q", got.String(), samplePeerID)
	}
}

// A typo in a peer ID must stop the node at startup. Accepting it would build a
// table that silently blackholes every packet routed through that address.
func TestParsePeerTableRejectsMalformedPeerID(t *testing.T) {
	if _, err := parsePeerTable([]string{"10.1.0.1:not-a-peer-id"}); err == nil {
		t.Fatal("parsePeerTable() error = nil, want an error for a malformed peer ID")
	}
}

// The format check has to be enforced too, not just documented.
func TestParsePeerTableRejectsMalformedEntry(t *testing.T) {
	if _, err := parsePeerTable([]string{"10.1.0.1"}); err == nil {
		t.Fatal("parsePeerTable() error = nil, want an error for an entry without `:`")
	}
}
