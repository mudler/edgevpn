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
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Signer authenticates ledger writes. In production it wraps the libp2p host
// identity key; in tests it can wrap any generated key.
type Signer interface {
	Sign(msg []byte) ([]byte, error)
	ID() string // base58 peer.ID of the signer
}

type keySigner struct {
	priv crypto.PrivKey
	id   string
}

// NewSigner builds a Signer from a libp2p private key, deriving the peer ID
// that owners are matched against.
func NewSigner(priv crypto.PrivKey) (Signer, error) {
	id, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		return nil, err
	}
	return &keySigner{priv: priv, id: id.String()}, nil
}

func (s *keySigner) Sign(msg []byte) ([]byte, error) { return s.priv.Sign(msg) }
func (s *keySigner) ID() string                      { return s.id }

// canonical produces the deterministic byte string that is signed/verified for
// an entry. Every field that must be tamper-evident is length-prefixed so the
// encoding is unambiguous and identical on every node (never rely on Go map or
// fmt ordering here).
func canonical(bucket, key string, d SignedData) []byte {
	buf := &bytes.Buffer{}
	writeField(buf, []byte(bucket))
	writeField(buf, []byte(key))
	writeField(buf, []byte(d.Owner))

	var u8 [8]byte
	binary.BigEndian.PutUint64(u8[:], d.Version)
	buf.Write(u8[:])
	binary.BigEndian.PutUint64(u8[:], uint64(d.UpdatedAt))
	buf.Write(u8[:])

	if d.Deleted {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	writeField(buf, []byte(d.Value))
	return buf.Bytes()
}

func writeField(buf *bytes.Buffer, b []byte) {
	var l [8]byte
	binary.BigEndian.PutUint64(l[:], uint64(len(b)))
	buf.Write(l[:])
	buf.Write(b)
}

// Verify checks that an entry was signed by the peer it claims as Owner. The
// verifying public key is recovered directly from the Owner peer.ID (libp2p
// Ed25519 identities embed it), so no key distribution is needed.
func Verify(bucket, key string, d SignedData) error {
	pid, err := peer.Decode(d.Owner)
	if err != nil {
		return fmt.Errorf("invalid owner id %q: %w", d.Owner, err)
	}
	pub, err := pid.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("cannot extract public key from owner id %q: %w", d.Owner, err)
	}
	if pub == nil {
		return errors.New("owner id has no embedded public key")
	}
	ok, err := pub.Verify(canonical(bucket, key, d), d.Sig)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("signature verification failed")
	}
	return nil
}
