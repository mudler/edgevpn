package discovery

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/lthibault/jitterbug"
	"github.com/xlzd/gotp"
)

type DHT struct {
	OTPKey               string
	OTPInterval          int
	KeyLength            int
	RendezvousString     string
	BootstrapPeers       AddrList
	latestRendezvous     string
	console              *zap.Logger
	RefreshDiscoveryTime int64
	dht                  *dht.IpfsDHT
}

func (d *DHT) Option(ctx context.Context) func(c *libp2p.Config) error {
	return libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		// make the DHT with the given Host
		return d.startDHT(ctx, h)
	})
}
func (d *DHT) Rendezvous() string {
	if d.OTPKey != "" {
		totp := gotp.NewTOTP(d.OTPKey, d.KeyLength, d.OTPInterval, nil)

		//totp := gotp.NewDefaultTOTP(d.OTPKey)
		rv := totp.Now()
		d.latestRendezvous = rv
		return rv
	}
	return d.RendezvousString
}

func (d *DHT) startDHT(ctx context.Context, h host.Host) (*dht.IpfsDHT, error) {
	if d.dht == nil {
		// Start a DHT, for use in peer discovery. We can't just make a new DHT
		// client because we want each peer to maintain its own local copy of the
		// DHT, so that the bootstrapping node of the DHT can go down without
		// inhibiting future peer discovery.
		kad, err := dht.New(ctx, h)
		if err != nil {
			return d.dht, err
		}
		d.dht = kad
	}

	return d.dht, nil
}

func (d *DHT) Run(c *zap.Logger, ctx context.Context, host host.Host) error {
	if d.KeyLength == 0 {
		d.KeyLength = 12
	}

	d.console = c
	if len(d.BootstrapPeers) == 0 {
		d.BootstrapPeers = dht.DefaultBootstrapPeers
	}
	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := d.startDHT(ctx, host)
	if err != nil {
		return err
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	c.Sugar().Info("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return err
	}

	connect := func() {
		d.bootstrapPeers(c, ctx, host)
		if d.latestRendezvous != "" {
			d.announceAndConnect(ctx, kademliaDHT, host, d.latestRendezvous)
		}

		rv := d.Rendezvous()
		d.announceAndConnect(ctx, kademliaDHT, host, rv)
	}

	go func() {
		connect()

		t := jitterbug.New(
			time.Second*time.Duration(d.RefreshDiscoveryTime),
			&jitterbug.Norm{Stdev: time.Second * 10},
		)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				connect()

			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (d *DHT) bootstrapPeers(c *zap.Logger, ctx context.Context, host host.Host) {
	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var wg sync.WaitGroup
	for _, peerAddr := range d.BootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
				c.Sugar().Warn(err.Error())
			} else {
				c.Sugar().Info("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()
}

func (d *DHT) announceAndConnect(ctx context.Context, kademliaDHT *dht.IpfsDHT, host host.Host, rv string) error {
	d.console.Sugar().Info("Announcing ourselves...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	discovery.Advertise(ctx, routingDiscovery, rv)
	d.console.Sugar().Info("Successfully announced!")
	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	d.console.Sugar().Info("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, rv)
	if err != nil {
		return err
	}
	//	var wg sync.WaitGroup

	for p := range peerChan {
		// Don't dial ourselves or peers without address
		if p.ID == host.ID() || len(p.Addrs) == 0 {
			continue
		}
		//	wg.Add(1)
		//	go func(a peer.AddrInfo) {
		//	defer wg.Done()

		if host.Network().Connectedness(p.ID) != network.Connected {
			d.console.Sugar().Info("Found peer:", p)
			if err := host.Connect(ctx, p); err != nil {
				d.console.Sugar().Info("Failed connecting to", p)
			} else {
				d.console.Sugar().Info("Connected to:", p)
			}
		} else {
			d.console.Sugar().Info("Known peer (already connected):", p)
		}
		//}(p)

	}
	//	wg.Wait()

	return nil
}
