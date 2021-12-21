package discovery

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns_legacy"
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
	// XXX: Valid for new mdns
	// disc := mdns.NewMdnsService(host, d.DiscoveryServiceTag, &discoveryNotifee{h: host, c: l})
	// return disc.Start()
	// We stick to legacy atm as mdns 0.15 is kinda of broken
	// see: https://github.com/libp2p/go-libp2p/pull/1192
	disc, err := mdns.NewMdnsService(ctx, host, time.Hour, d.DiscoveryServiceTag)
	if err != nil {
		return err
	}

	n := &discoveryNotifee{h: host, c: l}

	disc.RegisterNotifee(n)

	return nil
}
