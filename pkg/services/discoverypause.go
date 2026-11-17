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

package services

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
)

const (
	// pauseCheckInAge is how recent a peer's check-in in the health check
	// bucket must be for the pause to wait for a connection to it. Peers
	// check in every 2 minutes by default, so a peer that missed two
	// check-ins is taken as off.
	pauseCheckInAge = 5 * time.Minute

	// pauseMax is the longest time without a discovery round. It bounds
	// what a wrong "connected to everyone" costs, and how long a node that
	// comes online waits when every node that is on is paused.
	pauseMax = time.Hour

	// pauseAddrDelay is how long the node's addresses have to stay the same
	// after a change before a discovery round announces them. A relay
	// reservation or a network switch changes them several times within a
	// few seconds.
	pauseAddrDelay = 30 * time.Second

	// pauseWatchInterval is how often the watcher checks whether a peer
	// lost its connection while discovery is paused.
	pauseWatchInterval = 30 * time.Second
)

// DiscoveryPause holds back the regular DHT discovery rounds while the node
// has a direct connection to every peer that checked in recently. Set Pause
// and Wake as config.Discovery's Pause and Wake, and add NetworkService to
// the node.
//
// A round still runs at least once every pauseMax, when a peer loses its
// connection, when the node's own addresses change, and when WakeUp is
// called. The pause needs the health check bucket, which the Alive service
// writes. Without check-ins from other peers it never pauses.
type DiscoveryPause struct {
	l     log.StandardLogger
	now   func() time.Time
	wake  chan struct{}
	addrs *debouncer

	mu        sync.Mutex
	connected func() bool
	lastRound time.Time
	paused    bool
	// watched is what the last watch saw.
	watched bool
	// sawAddrs is true once the first address event arrived.
	sawAddrs bool
}

// NewDiscoveryPause returns a DiscoveryPause that logs to l.
func NewDiscoveryPause(l log.StandardLogger) *DiscoveryPause {
	return newDiscoveryPause(l, time.Now, pauseAddrDelay)
}

func newDiscoveryPause(l log.StandardLogger, now func() time.Time, addrDelay time.Duration) *DiscoveryPause {
	p := &DiscoveryPause{l: l, now: now, wake: make(chan struct{}, 1), lastRound: now()}
	p.addrs = newDebouncer(addrDelay, func() { p.WakeUp("this node's addresses changed") })
	return p
}

// Wake receives once for each round WakeUp asks for.
func (p *DiscoveryPause) Wake() <-chan struct{} {
	return p.wake
}

// Pause reports whether the next regular discovery round can be skipped.
func (p *DiscoveryPause) Pause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	pause := p.connected != nil && p.connected() && now.Sub(p.lastRound) < pauseMax
	if !pause {
		p.lastRound = now
	}
	if pause != p.paused {
		p.paused = pause
		if pause {
			p.l.Info("Discovery paused: connected to every peer that is on")
		} else {
			p.l.Info("Discovery resumed")
		}
	}
	return pause
}

// WakeUp asks for a discovery round now, also while discovery is paused. A
// round already asked for and not started yet covers this one too.
func (p *DiscoveryPause) WakeUp(reason string) {
	p.mu.Lock()
	p.lastRound = p.now()
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
		p.l.Info("Discovery round now: ", reason)
	default:
	}
}

func (p *DiscoveryPause) setConnected(f func() bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connected = f
}

// addressEvent handles an EvtLocalAddressesUpdated. The event bus hands a new
// subscriber the last event at once, which holds the addresses the node has
// had since before its first round, so the first event is skipped.
func (p *DiscoveryPause) addressEvent(diffs bool) {
	p.mu.Lock()
	first := !p.sawAddrs
	p.sawAddrs = true
	p.mu.Unlock()
	if first || !diffs {
		return
	}
	p.addrs.trigger()
}

// watch wakes discovery up when a peer lost its connection since the last
// watch, so that the node looks for it without waiting for the next regular
// round. It also starts the round that the hour cap asks for, because Pause
// sees the cap only on a regular tick, and those can be many minutes apart.
func (p *DiscoveryPause) watch() {
	p.mu.Lock()
	ok := p.connected != nil && p.connected()
	lost := p.watched && !ok
	p.watched = ok
	overdue := p.now().Sub(p.lastRound) >= pauseMax
	p.mu.Unlock()
	switch {
	case lost:
		p.WakeUp("a peer has no connection")
	case overdue:
		p.WakeUp("an hour without a discovery round")
	}
}

// NetworkService feeds the pause from the node's host and ledger: the
// connection check, a watch every pauseWatchInterval, and a wake once the
// node's own addresses change and settle, because the DHT record of this
// node holds the addresses it had when it was announced.
func (p *DiscoveryPause) NetworkService() node.NetworkService {
	return func(ctx context.Context, _ node.Config, n *node.Node, ledger *blockchain.Ledger) error {
		h := n.Host()
		self := h.ID().String()
		var clock checkInClock
		p.setConnected(func() bool {
			now := time.Now()
			return everyLivePeerConnected(clock.observe(checkInTimes(ledger.CurrentData()), now), func(id string) bool {
				pid, err := peer.Decode(id)
				return err == nil && h.Network().Connectedness(pid) == network.Connected
			}, now, self)
		})
		sub, err := h.EventBus().Subscribe(new(event.EvtLocalAddressesUpdated))
		if err != nil {
			p.l.Warn("Discovery pause cannot follow this node's addresses: ", err.Error())
		}
		go func() {
			t := time.NewTicker(pauseWatchInterval)
			defer t.Stop()
			var addrs <-chan interface{}
			if sub != nil {
				defer sub.Close()
				addrs = sub.Out()
			}
			defer p.addrs.stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					p.watch()
				case e, ok := <-addrs:
					if !ok {
						addrs = nil
						continue
					}
					if u, isUpdate := e.(event.EvtLocalAddressesUpdated); isUpdate {
						p.addressEvent(u.Diffs)
					}
				}
			}
		}()
		return nil
	}
}

// everyLivePeerConnected reports whether at least one peer other than self
// checked in within pauseCheckInAge of now, and connected reports true for
// each such peer. checkIns maps peer IDs to their last check-in.
func everyLivePeerConnected(checkIns map[string]time.Time, connected func(string) bool, now time.Time, self string) bool {
	live := 0
	for id, at := range checkIns {
		if id == self || now.Sub(at) > pauseCheckInAge {
			continue
		}
		if !connected(id) {
			return false
		}
		live++
	}
	return live > 0
}

// checkInClock tells when this node saw each peer's check-in change. A peer
// whose clock is behind writes check-ins that look old, and a peer with a
// long check-in interval writes them rarely. Counting the local time of a
// change as well keeps both from looking off while they check in.
type checkInClock struct {
	mu   sync.Mutex
	last map[string]time.Time
	seen map[string]time.Time
}

// observe returns, for each peer of checkIns, the later of its check-in and
// the time now at which this node first saw that check-in. A check-in
// already there at the first observe counts by its own timestamp only.
func (c *checkInClock) observe(checkIns map[string]time.Time, now time.Time) map[string]time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		c.last, c.seen = map[string]time.Time{}, map[string]time.Time{}
	}
	out := make(map[string]time.Time, len(checkIns))
	for id, at := range checkIns {
		prev, known := c.last[id]
		if known && !prev.Equal(at) {
			c.seen[id] = now
		}
		c.last[id] = at
		out[id] = at
		if s := c.seen[id]; s.After(at) {
			out[id] = s
		}
	}
	return out
}

// checkInTimes returns the last check-in of each peer in the ledger data.
// Entries that do not parse are left out.
func checkInTimes(data map[string]map[string]blockchain.Data) map[string]time.Time {
	out := map[string]time.Time{}
	for id, v := range data[protocol.HealthCheckKey] {
		var s string
		if v.Unmarshal(&s) != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			out[id] = t
		}
	}
	return out
}

// debouncer calls f once, delay after the last trigger.
type debouncer struct {
	delay time.Duration
	f     func()

	mu      sync.Mutex
	t       *time.Timer
	stopped bool
}

func newDebouncer(delay time.Duration, f func()) *debouncer {
	return &debouncer{delay: delay, f: f}
}

func (d *debouncer) trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	if d.t != nil {
		d.t.Stop()
	}
	d.t = time.AfterFunc(d.delay, d.f)
}

func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = true
	if d.t != nil {
		d.t.Stop()
	}
}
