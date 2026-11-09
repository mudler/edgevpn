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
	"context"
	"io"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// DefaultStaleConnsGrace is how much older than an incoming relayed
// connection a direct connection must be before the watcher treats it as
// stale. It keeps a startup race, where the relayed and the direct
// connection open at about the same time, from closing a working direct
// connection.
const DefaultStaleConnsGrace = 10 * time.Second

// staleConnsDialTimeout bounds the watcher's direct dial to a peer.
const staleConnsDialTimeout = 30 * time.Second

// staleConnsDialCooldown is the shortest time between two dials of the
// watcher to the same peer. A relayed connection can stay open for half an
// hour, and a peer whose direct connections keep dropping would otherwise
// get a dial each time. A forced direct dial skips the swarm's dial backoff,
// and on a routed host every failed dial also runs a DHT lookup.
const staleConnsDialCooldown = time.Minute

// StaleConnsOption configures WatchStaleDirectConns.
type StaleConnsOption func(*staleConnsWatcher)

// WithStaleConnsGrace sets the grace described in DefaultStaleConnsGrace.
func WithStaleConnsGrace(d time.Duration) StaleConnsOption {
	return func(w *staleConnsWatcher) { w.grace = d }
}

// WithStaleConnsLogger sets the logger for debug messages about closed
// connections and dials.
func WithStaleConnsLogger(l log.StandardLogger) StaleConnsOption {
	return func(w *staleConnsWatcher) { w.log = l }
}

type debugLogger interface {
	Debugf(format string, args ...interface{})
}

type noopLogger struct{}

func (noopLogger) Debugf(string, ...interface{}) {}

type staleConnsWatcher struct {
	network.NoopNotifiee
	h      host.Host
	grace  time.Duration
	log    debugLogger
	ctx    context.Context
	cancel context.CancelFunc
	sub    event.Subscription
	wg     sync.WaitGroup

	mu sync.Mutex
	// closedFor holds the relayed connections for which the watcher closed
	// a stale direct connection and still waits for identify.
	closedFor map[network.Conn]struct{}
	// closing holds the direct connections the watcher closes itself. The
	// fast path dials after identify, so their Disconnected must not dial
	// earlier, on addresses that may be gone.
	closing map[network.Conn]struct{}
	dialing map[peer.ID]struct{}
	// lastDial holds when the watcher last started a dial to each peer.
	lastDial     map[peer.ID]time.Time
	dialCooldown time.Duration
	// deferred holds the timer of the dial to each peer that waits for the
	// cooldown to end.
	deferred map[peer.ID]*time.Timer
}

func newStaleConnsWatcher(h host.Host, opts ...StaleConnsOption) *staleConnsWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	w := &staleConnsWatcher{
		h:            h,
		grace:        DefaultStaleConnsGrace,
		log:          noopLogger{},
		ctx:          ctx,
		cancel:       cancel,
		closedFor:    make(map[network.Conn]struct{}),
		closing:      make(map[network.Conn]struct{}),
		dialing:      make(map[peer.ID]struct{}),
		lastDial:     make(map[peer.ID]time.Time),
		dialCooldown: staleConnsDialCooldown,
		deferred:     make(map[peer.ID]*time.Timer),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// WatchStaleDirectConns closes direct connections that a relayed connection
// shows to be dead and dials the peer directly again.
//
// A peer that dials through a relay while this host holds a direct
// connection to it has no working direct connection of its own, because
// libp2p reuses a direct connection before it dials. The direct connection
// on this side is then dead, for example because the peer changed networks,
// but it stays open until the QUIC idle timeout. While it is open, the hole
// puncher that reacts to the relayed connection finds it and does nothing.
//
// The watcher closes it as soon as the relayed connection arrives. libp2p's
// hole puncher waits for identify over the relayed connection, so on most
// paths it then finds no direct connection and dials or hole punches. When
// identify wins that race, nothing in libp2p tries again, so once identify
// has given this host the peer's new addresses the watcher dials them
// itself. It also dials when identify fails, and then relies on the host to
// find the peer's addresses, as edgevpn's routed host does through the DHT. That dial reaches a peer whose new address accepts incoming
// connections; a peer behind NAT still needs the hole puncher.
//
// A stale direct connection younger than the grace stays open. When it, or
// any other direct connection, closes and leaves the peer with only relayed
// connections, the watcher dials the peer directly as well.
//
// Close stops the watcher, cancels its dials and waits for them.
func WatchStaleDirectConns(h host.Host, opts ...StaleConnsOption) io.Closer {
	w := newStaleConnsWatcher(h, opts...)
	sub, err := h.EventBus().Subscribe([]interface{}{
		new(event.EvtPeerIdentificationCompleted),
		new(event.EvtPeerIdentificationFailed),
	})
	if err != nil {
		w.log.Debugf("stale connection watcher cannot follow identify, it will not dial: %v", err)
	} else {
		w.sub = sub
		w.wg.Add(1)
		go w.handleIdentify()
	}
	h.Network().Notify(w)
	return w
}

func (w *staleConnsWatcher) Close() error {
	w.h.Network().StopNotify(w)
	w.cancel()
	// dialDirect schedules no deferred dial once the context is cancelled,
	// so the map holds every timer left to stop. A timer that Stop cannot
	// stop has fired, and its function returns on the cancelled context.
	w.mu.Lock()
	for p, t := range w.deferred {
		if t.Stop() {
			w.wg.Done()
		}
		delete(w.deferred, p)
	}
	w.mu.Unlock()
	if w.sub != nil {
		w.sub.Close()
	}
	w.wg.Wait()
	return nil
}

// isRelayed reports whether c goes through a circuit relay. The circuit
// client marks a connection limited only when the relay announces limits,
// so the address is checked as well.
func isRelayed(c network.Conn) bool {
	if c.Stat().Limited {
		return true
	}
	_, err := c.RemoteMultiaddr().ValueForProtocol(ma.P_CIRCUIT)
	return err == nil
}

// Connected runs inside the swarm's notification path, where closing a
// connection risks lock trouble, so the work happens in a goroutine.
func (w *staleConnsWatcher) Connected(_ network.Network, c network.Conn) {
	if !isRelayed(c) {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.mu.Lock()
		stale := w.takeStaleDirectConns(c)
		if len(stale) > 0 {
			w.closedFor[c] = struct{}{}
		}
		w.mu.Unlock()
		w.closeConns(stale)
	}()
}

// Disconnected dials the peer directly when the last direct connection to
// it closes while a relayed one remains. Identify over the relayed
// connection has then normally given this host the peer's current
// addresses. The check runs in a goroutine, as in Connected.
func (w *staleConnsWatcher) Disconnected(_ network.Network, c network.Conn) {
	w.mu.Lock()
	delete(w.closedFor, c)
	_, closedByWatcher := w.closing[c]
	delete(w.closing, c)
	w.mu.Unlock()
	if closedByWatcher || isRelayed(c) {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		p := c.RemotePeer()
		if !w.onlyRelayed(p) {
			return
		}
		w.log.Debugf("last direct connection to %s on %s closed, a relayed one remains", p, c.RemoteMultiaddr())
		w.dialDirect(p)
	}()
}

// onlyRelayed reports whether every connection to p is relayed and there
// is at least one.
func (w *staleConnsWatcher) onlyRelayed(p peer.ID) bool {
	conns := w.h.Network().ConnsToPeer(p)
	for _, c := range conns {
		if !isRelayed(c) {
			return false
		}
	}
	return len(conns) > 0
}

// handleIdentify dials a peer once identify over a relayed connection
// completes or fails, if the watcher closed a stale direct connection to
// that peer. A failed identify gives this host no new addresses, but on a
// routed host a dial that fails on the known addresses looks the peer up
// in the DHT. handleIdentify also closes stale connections itself, because
// identify can end before the goroutine started by Connected runs.
func (w *staleConnsWatcher) handleIdentify() {
	defer w.wg.Done()
	for e := range w.sub.Out() {
		var p peer.ID
		var relayed []network.Conn
		switch evt := e.(type) {
		case event.EvtPeerIdentificationCompleted:
			if evt.Conn == nil || !isRelayed(evt.Conn) {
				continue
			}
			p, relayed = evt.Peer, []network.Conn{evt.Conn}
		case event.EvtPeerIdentificationFailed:
			// The event does not name the connection, so every relayed
			// connection to the peer counts.
			p = evt.Peer
			for _, c := range w.h.Network().ConnsToPeer(p) {
				if isRelayed(c) {
					relayed = append(relayed, c)
				}
			}
		default:
			continue
		}
		w.mu.Lock()
		closedEarlier := false
		var stale []network.Conn
		for _, c := range relayed {
			if _, ok := w.closedFor[c]; ok {
				closedEarlier = true
				delete(w.closedFor, c)
			}
			stale = append(stale, w.takeStaleDirectConns(c)...)
		}
		w.mu.Unlock()
		w.closeConns(stale)
		if closedEarlier || len(stale) > 0 {
			// libp2p's hole puncher may dial p at the same time. The swarm
			// joins concurrent dials to one peer into one (dial_sync.go in
			// p2p/net/swarm), so the two do not open two connections.
			w.wg.Add(1)
			go func() {
				defer w.wg.Done()
				w.dialDirect(p)
			}()
		}
	}
}

// takeStaleDirectConns returns the direct connections to the peer of
// relayed that opened at least grace before it and marks them as closed by
// the watcher. The caller holds w.mu, so that what one goroutine takes is
// recorded before the other looks, and closes the connections with
// closeConns after releasing it. Both goroutines may take the same
// connection; closing it twice is harmless, and the swarm drops a closing
// connection from ConnsToPeer before the close returns.
//
// Every entry added to closing is removed by Disconnected. The swarm drops
// a connection from ConnsToPeer before it sends Disconnected, and the
// caller holds w.mu from ConnsToPeer until the entry is written, which
// Disconnected needs as well. So a connection that ConnsToPeer returns
// has its Disconnected handled after the entry exists, even when it closes
// on its own in between.
func (w *staleConnsWatcher) takeStaleDirectConns(relayed network.Conn) []network.Conn {
	arrived := relayed.Stat().Opened
	var stale []network.Conn
	for _, c := range w.h.Network().ConnsToPeer(relayed.RemotePeer()) {
		if isRelayed(c) || arrived.Sub(c.Stat().Opened) < w.grace {
			continue
		}
		w.closing[c] = struct{}{}
		stale = append(stale, c)
	}
	return stale
}

// closeConns closes stale connections without holding w.mu: a close that
// blocks would otherwise stop handleIdentify, and identify's event emitter
// with it.
func (w *staleConnsWatcher) closeConns(stale []network.Conn) {
	for _, c := range stale {
		w.log.Debugf("closing stale direct connection to %s on %s: relayed connection arrived", c.RemotePeer(), c.RemoteMultiaddr())
		c.Close()
	}
}

// dialDirect dials p without relays, unless a dial from the watcher to p
// is already running or started less than dialCooldown ago. A dial the
// cooldown blocks is deferred to the end of the cooldown, because the
// event that asked for it does not come again.
func (w *staleConnsWatcher) dialDirect(p peer.ID) {
	now := time.Now()
	w.mu.Lock()
	if _, ok := w.dialing[p]; ok {
		w.mu.Unlock()
		return
	}
	for id, t := range w.lastDial {
		if now.Sub(t) >= w.dialCooldown {
			delete(w.lastDial, id)
		}
	}
	if last, ok := w.lastDial[p]; ok {
		w.deferDialLocked(p, w.dialCooldown-now.Sub(last))
		w.mu.Unlock()
		w.log.Debugf("deferring the direct dial to %s: the last dial was less than %s ago", p, w.dialCooldown)
		return
	}
	w.dialing[p] = struct{}{}
	w.lastDial[p] = now
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.dialing, p)
		w.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(network.WithForceDirectDial(w.ctx, "replace stale direct connection"), staleConnsDialTimeout)
	defer cancel()
	w.log.Debugf("dialing %s directly to replace a stale connection", p)
	if err := w.h.Connect(ctx, peer.AddrInfo{ID: p}); err != nil {
		w.log.Debugf("direct dial to %s failed: %v", p, err)
	}
}

// deferDialLocked schedules a dial to p after wait, unless one is already
// scheduled, in which case the requests merge into it. The dial runs only
// if p then still has relayed connections and no direct one. The caller
// holds w.mu.
func (w *staleConnsWatcher) deferDialLocked(p peer.ID, wait time.Duration) {
	if _, ok := w.deferred[p]; ok || w.ctx.Err() != nil {
		return
	}
	w.wg.Add(1)
	w.deferred[p] = time.AfterFunc(wait, func() {
		defer w.wg.Done()
		w.mu.Lock()
		delete(w.deferred, p)
		w.mu.Unlock()
		if w.ctx.Err() != nil || !w.onlyRelayed(p) {
			return
		}
		w.dialDirect(p)
	})
}
