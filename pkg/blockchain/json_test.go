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
)

// An unsigned entry must serialise to the exact pre-authentication wire format
// (a bare JSON value), so existing nodes that decode Storage values as plain
// strings keep working.
func TestSignedDataLegacyWireFormat(t *testing.T) {
	d := SignedData{Value: Data(`{"PeerID":"x"}`)}

	got, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want, _ := json.Marshal(`{"PeerID":"x"}`) // legacy: Data is a string -> JSON string
	if string(got) != string(want) {
		t.Fatalf("legacy marshal = %s, want %s", got, want)
	}

	var back SignedData
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Value != d.Value || back.Owner != "" || back.Version != 0 {
		t.Fatalf("legacy round-trip mismatch: %+v", back)
	}
}

func TestSignedDataSignedWireRoundTrip(t *testing.T) {
	d := SignedData{Value: Data(`"v"`), Owner: "peerX", Version: 3, UpdatedAt: 100, Sig: []byte{1, 2, 3}}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back SignedData
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Owner != "peerX" || back.Version != 3 || back.UpdatedAt != 100 ||
		string(back.Sig) != string([]byte{1, 2, 3}) || back.Value != d.Value {
		t.Fatalf("signed round-trip mismatch: %+v", back)
	}
}
