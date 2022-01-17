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

package protocol

import (
	p2pprotocol "github.com/libp2p/go-libp2p-core/protocol"
)

const (
	EdgeVPN         Protocol = "/edgevpn/0.1"
	ServiceProtocol Protocol = "/edgevpn/service/0.1"
	FileProtocol    Protocol = "/edgevpn/file/0.1"
)

const (
	FilesLedgerKey    = "files"
	MachinesLedgerKey = "machines"
	ServicesLedgerKey = "services"
	UsersLedgerKey    = "users"
)

type Protocol string

func (p Protocol) ID() p2pprotocol.ID {
	return p2pprotocol.ID(string(p))
}
