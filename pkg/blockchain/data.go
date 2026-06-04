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
	"bytes"
	"encoding/json"
)

type Data string

// Unmarshal the result into the interface. Use it to retrieve data
// set with SetValue
func (d Data) Unmarshal(i interface{}) error {
	return json.Unmarshal([]byte(d), i)
}

// SignedData is a ledger entry authenticated by its author. The Value is the
// original JSON payload (readers still call Value.Unmarshal); the remaining
// fields bind that value to the owning peer and a monotonic version so that
// only the owner can update it and an older value cannot be replayed over a
// newer one. See docs/design/authenticated-ledger.md.
type SignedData struct {
	Value     Data   // original JSON payload
	Owner     string // peer.ID (base58) of the author
	Version   uint64 // monotonic per (bucket,key); bumps only when Value changes
	UpdatedAt int64  // unix seconds, signed; drives TTL/lease renewal
	Deleted   bool   // signed tombstone marker
	Sig       []byte // signature over canonical(bucket,key,...)
}

// signedDataAlias avoids infinite recursion in (Un)MarshalJSON.
type signedDataAlias SignedData

// MarshalJSON keeps unsigned entries on the legacy wire format (a bare JSON
// value), so nodes that predate authentication can still decode them. Signed
// entries are encoded as the full object.
func (d SignedData) MarshalJSON() ([]byte, error) {
	if d.Owner == "" && d.Sig == nil && !d.Deleted && d.Version == 0 {
		return json.Marshal(string(d.Value))
	}
	return json.Marshal(signedDataAlias(d))
}

// UnmarshalJSON accepts either the legacy bare value or the full signed object.
func (d *SignedData) UnmarshalJSON(b []byte) error {
	t := bytes.TrimSpace(b)
	if len(t) > 0 && t[0] != '{' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		d.Value = Data(s)
		return nil
	}
	var a signedDataAlias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*d = SignedData(a)
	return nil
}
