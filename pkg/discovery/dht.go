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
	"crypto/sha256"
	"sync"
	"time"

	internalCrypto "github.com/mudler/edgevpn/pkg/crypto"
	"github.com/mudler/edgevpn/pkg/utils"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

type DHT struct {
	OTPKey               string
	OTPInterval          int
	KeyLength            int
	RendezvousString     string
	BootstrapPeers       AddrList
	latestRendezvous     string
	RefreshDiscoveryTime time.Duration
	*dht.IpfsDHT
	dhtOptions []dht.Option
}

func NewDHT(d ...dht.Option) *DHT {
	return &DHT{dhtOptions: d}
}

func (d *DHT) Option(ctx context.Context) func(c *libp2p.Config) error {
	return libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		// make the DHT with the given Host
		return d.startDHT(ctx, h)
	})
}
func (d *DHT) Rendezvous() string {
	if d.OTPKey != "" {
		totp := internalCrypto.TOTP(sha256.New, d.KeyLength, d.OTPInterval, d.OTPKey)

		rv := internalCrypto.MD5(totp)
		d.latestRendezvous = rv
		return rv
	}
	return d.RendezvousString
}

func (d *DHT) startDHT(ctx context.Context, h host.Host) (*dht.IpfsDHT, error) {
	if d.IpfsDHT == nil {
		// Start a DHT, for use in peer discovery. We can't just make a new DHT
		// client because we want each peer to maintain its own local copy of the
		// DHT, so that the bootstrapping node of the DHT can go down without
		// inhibiting future peer discovery.

		kad, err := dht.New(ctx, h, d.dhtOptions...)
		if err != nil {
			return d.IpfsDHT, err
		}
		d.IpfsDHT = kad
	}

	return d.IpfsDHT, nil
}

func (d *DHT) Run(c log.StandardLogger, ctx context.Context, host host.Host) error {
	if d.KeyLength == 0 {
		d.KeyLength = 12
	}

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
	c.Info("Bootstrapping DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return err
	}

	connect := func() {
		d.bootstrapPeers(c, ctx, host)
		if d.latestRendezvous != "" {
			d.announceAndConnect(c, ctx, kademliaDHT, host, d.latestRendezvous)
		}

		rv := d.Rendezvous()
		d.announceAndConnect(c, ctx, kademliaDHT, host, rv)
	}

	go func() {
		connect()
		t := utils.NewBackoffTicker(utils.BackoffMaxInterval(d.RefreshDiscoveryTime))
		defer t.Stop()
		for {
			select {
			case <-t.C:
				go connect()
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (d *DHT) bootstrapPeers(c log.StandardLogger, ctx context.Context, host host.Host) {
	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var wg sync.WaitGroup
	for _, peerAddr := range d.BootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if host.Network().Connectedness(peerinfo.ID) != network.Connected {
				if err := host.Connect(ctx, *peerinfo); err != nil {
					c.Debug(err.Error())
				} else {
					c.Debug("Connection established with bootstrap node:", *peerinfo)
				}
			}
		}()
	}
	wg.Wait()
}

func (d *DHT) FindClosePeers(ll log.StandardLogger, onlyStaticRelays bool, static ...string) func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
	return func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
		peerChan := make(chan peer.AddrInfo, numPeers)
		go func() {

			toStream := []peer.AddrInfo{}

			if !onlyStaticRelays {
				closestPeers, err := d.GetClosestPeers(ctx, d.PeerID().String())
				if err != nil {
					ll.Debug("Error getting closest peers: ", err)
				}

				for _, p := range closestPeers {
					addrs := d.Host().Peerstore().Addrs(p)
					if len(addrs) == 0 {
						continue
					}
					ll.Debugf("[relay discovery] Found close peer '%s'", p.String())
					toStream = append(toStream, peer.AddrInfo{ID: p, Addrs: addrs})
				}
			}

			for _, r := range static {
				pi, err := peer.AddrInfoFromString(r)
				if err == nil {
					ll.Debug("[static relay discovery] scanning ", pi.ID)
					toStream = append(toStream, peer.AddrInfo{ID: pi.ID, Addrs: pi.Addrs})
				}
			}

			if len(toStream) > numPeers {
				toStream = toStream[0 : numPeers-1]
			}

			for _, t := range toStream {
				peerChan <- t
			}

			close(peerChan)
		}()

		return peerChan
	}
}

func (d *DHT) announceAndConnect(l log.StandardLogger, ctx context.Context, kademliaDHT *dht.IpfsDHT, host host.Host, rv string) error {
	l.Debug("Announcing ourselves...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	routingDiscovery.Advertise(ctx, rv)
	l.Debug("Successfully announced!")
	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	l.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, rv)
	if err != nil {
		return err
	}

	for p := range peerChan {
		// Don't dial ourselves or peers without address
		if p.ID == host.ID() || len(p.Addrs) == 0 {
			continue
		}

		if host.Network().Connectedness(p.ID) != network.Connected {
			l.Debug("Found peer:", p)
			if err := host.Connect(ctx, p); err != nil {
				l.Debug("Failed connecting to", p)
			} else {
				l.Debug("Connected to:", p)
			}
		} else {
			l.Debug("Known peer (already connected):", p)
		}
	}

	return nil
}
