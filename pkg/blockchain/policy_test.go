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

package blockchain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/protocol"
)

func tsData(t time.Time) Data {
	b, _ := json.Marshal(t.UTC().Format(time.RFC3339))
	return Data(b)
}

func TestIsLive(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hd := map[string]Data{
		"peerA": tsData(now.Add(-10 * time.Second)),
		"peerB": tsData(now.Add(-10 * time.Minute)),
	}

	if !IsLive(hd, "peerA", time.Minute, now) {
		t.Fatal("peerA heartbeat is recent, should be live")
	}
	if IsLive(hd, "peerB", time.Minute, now) {
		t.Fatal("peerB heartbeat is stale, should not be live")
	}
	if IsLive(hd, "unknown", time.Minute, now) {
		t.Fatal("peer with no heartbeat should not be live")
	}
}

func TestDefaultRegistryOwnership(t *testing.T) {
	r := DefaultRegistry(time.Minute)

	machine, _ := json.Marshal(map[string]string{"PeerID": "peerX", "Address": "10.1.0.1"})
	if got := r.Policy(protocol.MachinesLedgerKey).OwnerOf("10.1.0.1", Data(machine)); got != "peerX" {
		t.Fatalf("machine owner = %q, want peerX", got)
	}

	if got := r.Policy(protocol.UsersLedgerKey).OwnerOf("peerY", ""); got != "peerY" {
		t.Fatalf("user owner = %q, want peerY (key is owner)", got)
	}

	if r.Policy("some-unregistered-bucket").Owned {
		t.Fatal("unregistered bucket must be unowned (open) by default")
	}
}
