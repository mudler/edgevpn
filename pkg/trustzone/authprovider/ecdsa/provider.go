// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package ecdsa

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ipfs/go-log/v2"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/node"
)

type ECDSA521 struct {
	privkey string
	logger  log.StandardLogger
}

// ECDSA521Provider returns an ECDSA521 auth provider.
// To use it, use the following configuration to provide a
// private key: AuthProviders: map[string]map[string]interface{}{"ecdsa": {"private_key": "<key>"}},
// While running, keys can be added from a TZ node also from the api, for example:
// curl -X PUT 'http://localhost:8081/api/ledger/trustzoneAuth/ecdsa_1/<key>'
// Note: privkey and pubkeys are in the format generated by GenerateKeys() down below
// The provider resolves "ecdsa" keys in the trustzone auth area, and
// uses each one as pubkey to try to auth against
func ECDSA521Provider(ll log.StandardLogger, privkey string) (*ECDSA521, error) {
	return &ECDSA521{privkey: privkey, logger: ll}, nil
}

// Authenticate a message against a set of pubkeys.
// It cycles over all the Trusted zone Auth data ( providers options, not where senders ID are stored)
// and detects any key with ecdsa prefix. Values are assumed to be string and parsed as pubkeys.
// The pubkeys are then used to authenticate nodes and verify if any of the pubkeys validates the challenge.
func (e *ECDSA521) Authenticate(m *hub.Message, c chan *hub.Message, tzdata map[string]blockchain.Data) bool {

	sigs, ok := m.Annotations["sigs"]
	if !ok {
		e.logger.Debug("No signature in message", m.Message, m.Annotations)

		return false
	}

	e.logger.Debug("ECDSA auth Received", m)

	pubKeys := []string{}
	for k, t := range tzdata {
		if strings.Contains(k, "ecdsa") {
			var s string
			t.Unmarshal(&s)
			pubKeys = append(pubKeys, s)
		}
	}
	if len(pubKeys) == 0 {
		e.logger.Debug("ECDSA auth: No pubkeys to auth against")
		// no pubkeys to authenticate present in the ledger
		return false
	}
	for _, pubkey := range pubKeys {
		// Try verifying the signature
		if err := verify([]byte(pubkey), []byte(fmt.Sprint(sigs)), bytes.NewBufferString(m.Message)); err == nil {
			e.logger.Debug("ECDSA auth: Signature verified")
			return true
		}
		e.logger.Debug("ECDSA auth: Signature not verified")
	}
	return false
}

// Challenger sends ECDSA521 challenges over the public channel if the current node is not in the trusted zone.
// This start a challenge which eventually should get the node into the TZ
func (e *ECDSA521) Challenger(inTrustZone bool, c node.Config, n *node.Node, b *blockchain.Ledger, trustData map[string]blockchain.Data) {
	if !inTrustZone {
		e.logger.Debug("ECDSA auth: current node not in trustzone, sending challanges")
		signature, err := sign([]byte(e.privkey), bytes.NewBufferString("challenge"))
		if err != nil {
			e.logger.Error("Error signing message: ", err.Error())
			return
		}
		msg := hub.NewMessage("challenge")
		msg.Annotations = make(map[string]interface{})
		msg.Annotations["sigs"] = string(signature)
		n.PublishMessage(msg)
		return
	}
}
