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
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func newTestSigner(t *testing.T) Signer {
	t.Helper()
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	s, err := NewSigner(priv)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return s
}

func TestSignVerifyRoundTrip(t *testing.T) {
	s := newTestSigner(t)
	d := SignedData{Value: "hello", Owner: s.ID(), Version: 1, UpdatedAt: 100}

	sig, err := s.Sign(canonical("machines", "10.1.0.1", d))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	d.Sig = sig

	if err := Verify("machines", "10.1.0.1", d); err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}
}

func TestVerifyRejectsTamperedValue(t *testing.T) {
	s := newTestSigner(t)
	d := SignedData{Value: "hello", Owner: s.ID(), Version: 1, UpdatedAt: 100}
	d.Sig, _ = s.Sign(canonical("machines", "10.1.0.1", d))

	d.Value = "tampered"

	if err := Verify("machines", "10.1.0.1", d); err == nil {
		t.Fatal("expected verification to fail for a tampered value")
	}
}

// A signature is bound to the claimed owner: presenting it under a different
// Owner peer.ID must not verify (this is what stops impersonation).
func TestVerifyRejectsWrongOwner(t *testing.T) {
	s := newTestSigner(t)
	other := newTestSigner(t)

	d := SignedData{Value: "hello", Owner: s.ID(), Version: 1, UpdatedAt: 100}
	d.Sig, _ = s.Sign(canonical("machines", "10.1.0.1", d))

	d.Owner = other.ID()

	if err := Verify("machines", "10.1.0.1", d); err == nil {
		t.Fatal("expected verification to fail when Owner != signing key")
	}
}
