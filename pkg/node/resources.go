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

package node

import (
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	libp2pprotocol "github.com/libp2p/go-libp2p-core/protocol"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
)

func NetSetLimit(mgr network.ResourceManager, scope string, limit rcmgr.Limit) error {
	setLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok {
			return fmt.Errorf("resource scope doesn't implement ResourceScopeLimiter interface")
		}

		limiter.SetLimit(limit)
		return nil
	}

	switch {
	case scope == "system":
		err := mgr.ViewSystem(func(s network.ResourceScope) error {
			return setLimit(s)
		})
		return err

	case scope == "transient":
		err := mgr.ViewTransient(func(s network.ResourceScope) error {
			return setLimit(s)
		})
		return err

	case strings.HasPrefix(scope, "svc:"):
		svc := scope[4:]
		err := mgr.ViewService(svc, func(s network.ServiceScope) error {
			return setLimit(s)
		})
		return err

	case strings.HasPrefix(scope, "proto:"):
		proto := scope[6:]
		err := mgr.ViewProtocol(libp2pprotocol.ID(proto), func(s network.ProtocolScope) error {
			return setLimit(s)
		})
		return err

	case strings.HasPrefix(scope, "peer:"):
		p := scope[5:]
		pid, err := peer.Decode(p)
		if err != nil {
			return fmt.Errorf("invalid peer ID: %s: %w", p, err)
		}
		err = mgr.ViewPeer(pid, func(s network.PeerScope) error {
			return setLimit(s)
		})
		return err

	default:
		return fmt.Errorf("invalid scope %s", scope)
	}
}
