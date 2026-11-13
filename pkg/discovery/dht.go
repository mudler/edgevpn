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
	coreDiscovery "github.com/libp2p/go-libp2p/core/discovery"
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
	RefreshDiscoveryTime time.Duration
	*dht.IpfsDHT
	dhtOptions []dht.Option
	clock      func() time.Time
}

const (
	// rendezvousOverlap is how long before a switch a node also meets at the
	// next meeting point, and how long after it the node also meets at the
	// previous one. Nodes compute the meeting point from their own clocks,
	// so around a switch some are on one side of it and some on the other.
	rendezvousOverlap = 2 * time.Minute

	// rendezvousQueryTimeout bounds one announcement or one search at a
	// meeting point.
	rendezvousQueryTimeout = 2 * time.Minute

	// rendezvousDialTimeout bounds a dial to a peer found at a meeting point.
	// Records of peers that are gone stay in the DHT for hours, and a dial
	// to one of them only ends when this runs out.
	rendezvousDialTimeout = 30 * time.Second
)

func NewDHT(d ...dht.Option) *DHT {
	return &DHT{dhtOptions: d, clock: time.Now}
}

func (d *DHT) Option(ctx context.Context) func(c *libp2p.Config) error {
	return libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		// make the DHT with the given Host
		return d.startDHT(ctx, h)
	})
}

// Rendezvous returns the current meeting point in the DHT.
func (d *DHT) Rendezvous() string {
	return d.rendezvousAt(d.clock())
}

// rotates reports whether the meeting point changes over time, which it does
// when it comes from a one-time password.
func (d *DHT) rotates() bool {
	return d.OTPKey != "" && d.OTPInterval > 0
}

// rendezvousAt returns the meeting point at time t.
func (d *DHT) rendezvousAt(t time.Time) string {
	if !d.rotates() {
		return d.RendezvousString
	}
	window := uint64(t.Unix()) / uint64(d.OTPInterval)
	totp := internalCrypto.TOTPAt(sha256.New, d.KeyLength, window, d.OTPKey)
	return internalCrypto.MD5(totp)
}

// rendezvousKeys returns the meeting points a round at time t uses, the
// current one first. Within rendezvousOverlap of a switch it adds the
// meeting point on the other side of the switch.
func (d *DHT) rendezvousKeys(t time.Time) []string {
	keys := []string{d.rendezvousAt(t)}
	if !d.rotates() {
		return keys
	}
	window, overlap := d.window()
	start := d.windowStart(t)
	if start.Add(window).Sub(t) <= overlap {
		keys = append(keys, d.rendezvousAt(start.Add(window)))
	} else if t.Sub(start) < overlap {
		keys = append(keys, d.rendezvousAt(start.Add(-window)))
	}
	return keys
}

// window returns how long one meeting point lasts and how long the overlap
// around a switch is. The overlap is at most half the window, so that a short
// interval does not put every moment within the overlap of some switch.
func (d *DHT) window() (time.Duration, time.Duration) {
	window := time.Duration(d.OTPInterval) * time.Second
	return window, min(rendezvousOverlap, window/2)
}

// windowStart returns the start of the window that contains t. Windows are
// counted from the Unix epoch, as the one-time password counts them, which
// time.Time.Truncate does not do.
func (d *DHT) windowStart(t time.Time) time.Time {
	interval := int64(d.OTPInterval)
	return time.Unix(t.Unix()/interval*interval, 0)
}

// untilNextRound returns how long to wait, from now, for the round that
// announces the next meeting point before it takes over. It reports false
// when the meeting point never changes.
func (d *DHT) untilNextRound(now time.Time) (time.Duration, bool) {
	if !d.rotates() {
		return 0, false
	}
	window, overlap := d.window()
	// Halfway into the overlap leaves time for the announcement to finish
	// before the switch.
	at := d.windowStart(now).Add(window - overlap/2)
	if !at.After(now) {
		at = at.Add(window)
	}
	return at.Sub(now), true
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

func (d *DHT) announceRendezvous(c log.StandardLogger, ctx context.Context, host host.Host, kademliaDHT *dht.IpfsDHT) {
	d.bootstrapPeers(c, ctx, host)
	connect := func(ctx context.Context, p peer.AddrInfo) error { return host.Connect(ctx, p) }
	connected := func(id peer.ID) bool { return host.Network().Connectedness(id) == network.Connected }
	d.round(c, ctx, discovery.NewRoutingDiscovery(kademliaDHT), connect, connected, host.ID())
}

// round announces this node at every meeting point rendezvousKeys gives, all
// at once, and searches each of them for other peers. It returns when the
// announcements are done. The searches and the dials to the peers they find
// go on in the background, each dial on its own, so that one slow meeting
// point or one peer that is gone holds up nothing else.
func (d *DHT) round(l log.StandardLogger, ctx context.Context, r coreDiscovery.Discovery, connect func(context.Context, peer.AddrInfo) error, connected func(peer.ID) bool, self peer.ID) {
	keys := d.rendezvousKeys(d.clock())
	l.Debugf("The following rendezvous points are being used: %+v", keys)

	var wg sync.WaitGroup
	for _, rv := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Debugf("Announcing with rendezvous: %s", rv)
			actx, cancel := context.WithTimeout(ctx, rendezvousQueryTimeout)
			if _, err := r.Advertise(actx, rv); err != nil {
				l.Debugf("Announcing with rendezvous %s failed: %s", rv, err)
			}
			cancel()
		}()
		go d.search(l, ctx, r, rv, connect, connected, self)
	}
	wg.Wait()
	l.Debug("Announcing to rendezvous done")
}

// search looks for peers at one meeting point and dials each new one in its
// own goroutine.
func (d *DHT) search(l log.StandardLogger, ctx context.Context, r coreDiscovery.Discovery, rv string, connect func(context.Context, peer.AddrInfo) error, connected func(peer.ID) bool, self peer.ID) {
	sctx, cancel := context.WithTimeout(ctx, rendezvousQueryTimeout)
	defer cancel()
	peers, err := r.FindPeers(sctx, rv)
	if err != nil {
		l.Debugf("Searching rendezvous %s failed: %s", rv, err)
		return
	}
	for p := range peers {
		if p.ID == self || len(p.Addrs) == 0 {
			continue
		}
		if connected(p.ID) {
			l.Debug("Known peer (already connected):", p)
			continue
		}
		l.Debug("Found peer:", p)
		go func() {
			dctx, cancel := context.WithTimeout(ctx, rendezvousDialTimeout)
			defer cancel()
			if err := connect(dctx, p); err != nil {
				l.Debugf("Failed connecting to '%s', error: '%s'", p, err.Error())
			} else {
				l.Debug("Connected to:", p)
			}
		}()
	}
	l.Debugf("Finished searching rendezvous %s", rv)
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

	go d.runBackground(c, ctx, host, kademliaDHT)

	return nil
}

// runBackground runs a round right away, then on a backoff ticker that
// slows down to RefreshDiscoveryTime, and also shortly before each switch of
// the meeting point, so the next meeting point has this node's record before
// anyone looks for it there.
func (d *DHT) runBackground(c log.StandardLogger, ctx context.Context, host host.Host, kademliaDHT *dht.IpfsDHT) {
	d.announceRendezvous(c, ctx, host, kademliaDHT)
	t := utils.NewBackoffTicker(utils.BackoffMaxInterval(d.RefreshDiscoveryTime))
	defer t.Stop()

	var beforeSwitch <-chan time.Time
	plan := func() {
		if wait, ok := d.untilNextRound(d.clock()); ok {
			beforeSwitch = time.After(wait)
		}
	}
	plan()
	for {
		select {
		case <-t.C:
			d.announceRendezvous(c, ctx, host, kademliaDHT)
		case <-beforeSwitch:
			d.announceRendezvous(c, ctx, host, kademliaDHT)
			plan()
		case <-ctx.Done():
			return
		}
	}
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
