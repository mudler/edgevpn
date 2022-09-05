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

package protocol

import (
	p2pprotocol "github.com/libp2p/go-libp2p/core/protocol"
)

const (
	EdgeVPN         Protocol = "/edgevpn/0.1"
	ServiceProtocol Protocol = "/edgevpn/service/0.1"
	FileProtocol    Protocol = "/edgevpn/file/0.1"
	EgressProtocol  Protocol = "/edgevpn/egress/0.1"
)

const (
	FilesLedgerKey    = "files"
	MachinesLedgerKey = "machines"
	ServicesLedgerKey = "services"
	UsersLedgerKey    = "users"
	HealthCheckKey    = "healthcheck"
	DNSKey            = "dns"
	EgressService     = "egress"
	TrustZoneKey      = "trustzone"
	TrustZoneAuthKey  = "trustzoneAuth"
)

type Protocol string

func (p Protocol) ID() p2pprotocol.ID {
	return p2pprotocol.ID(string(p))
}
