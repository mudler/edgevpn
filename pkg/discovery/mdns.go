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

package discovery

import (
	"context"
	"net"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type MDNS struct {
	DiscoveryServiceTag string

	// InterfaceFilter, when set, restricts the network interfaces mDNS runs on and
	// advertises addresses from. mDNS only considers interfaces that are up and
	// support multicast; the filter can narrow that set further, for example to
	// Wi-Fi and Ethernet so that mobile data and VPN addresses are not advertised.
	// Only the Android implementation uses it. On other platforms go-libp2p's mDNS
	// service picks the interfaces and this field is ignored.
	InterfaceFilter func(net.Interface) bool
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
	//n.c.Infof("mDNS: discovered new peer %s\n", pi.ID.String())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		n.c.Debugf("mDNS: error connecting to peer %s: %s\n", pi.ID.String(), err)
	}
}

func (d *MDNS) Option(ctx context.Context) func(c *libp2p.Config) error {
	return func(*libp2p.Config) error { return nil }
}
