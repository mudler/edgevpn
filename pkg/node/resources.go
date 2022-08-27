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

package node

import (
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pprotocol "github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
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
