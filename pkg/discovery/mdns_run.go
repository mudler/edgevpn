//go:build !android

package discovery

import (
	"context"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

func (d *MDNS) Run(l log.StandardLogger, ctx context.Context, host host.Host) error {
	disc := mdns.NewMdnsService(host, d.DiscoveryServiceTag, &discoveryNotifee{h: host, c: l})
	return disc.Start()
}
