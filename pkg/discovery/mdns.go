// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package discovery

import (
	"context"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type MDNS struct {
	DiscoveryServiceTag string
}

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
	c log.StandardLogger
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	//n.c.Infof("mDNS: discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		n.c.Debugf("mDNS: error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

func (d *MDNS) Option(ctx context.Context) func(c *libp2p.Config) error {
	return func(*libp2p.Config) error { return nil }
}

func (d *MDNS) Run(l log.StandardLogger, ctx context.Context, host host.Host) error {
	// setup mDNS discovery to find local peers

	disc := mdns.NewMdnsService(host, d.DiscoveryServiceTag, &discoveryNotifee{h: host, c: l})
	return disc.Start()
}
