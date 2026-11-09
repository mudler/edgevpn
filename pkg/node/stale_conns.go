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
	"io"
	"sync"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

// DefaultStaleConnsGrace is how much older than an incoming relayed
// connection a direct connection must be before the watcher treats it as
// stale. It keeps a startup race, where the relayed and the direct
// connection open at about the same time, from closing a working direct
// connection.
const DefaultStaleConnsGrace = 5 * time.Second

// StaleConnsOption configures WatchStaleDirectConns.
type StaleConnsOption func(*staleConnsWatcher)

// WithStaleConnsGrace sets the grace described in DefaultStaleConnsGrace.
func WithStaleConnsGrace(d time.Duration) StaleConnsOption {
	return func(w *staleConnsWatcher) { w.grace = d }
}

// WithStaleConnsLogger sets the logger for debug messages about closed
// connections.
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
	h     host.Host
	grace time.Duration
	log   debugLogger
	wg    sync.WaitGroup
}

// WatchStaleDirectConns closes direct connections that a relayed connection
// shows to be dead, so that libp2p's hole puncher can replace them.
//
// A peer that dials through a relay while this host holds a direct
// connection to it has no working direct connection of its own, because
// libp2p reuses a direct connection before it dials. The direct connection
// on this side is then dead, for example because the peer changed networks,
// but it stays open until the QUIC idle timeout. While it is open, the hole
// puncher that reacts to the relayed connection finds it and does nothing.
// The watcher closes it first, so the hole puncher dials the peer's new
// addresses once identify over the relayed connection completes.
//
// Close stops the watcher and waits for its work in progress.
func WatchStaleDirectConns(h host.Host, opts ...StaleConnsOption) io.Closer {
	w := &staleConnsWatcher{h: h, grace: DefaultStaleConnsGrace, log: noopLogger{}}
	for _, o := range opts {
		o(w)
	}
	h.Network().Notify(w)
	return w
}

func (w *staleConnsWatcher) Close() error {
	w.h.Network().StopNotify(w)
	w.wg.Wait()
	return nil
}

// Connected runs inside the swarm's notification path, where closing a
// connection risks lock trouble, so the work happens in a goroutine.
func (w *staleConnsWatcher) Connected(_ network.Network, c network.Conn) {
	if !c.Stat().Limited {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.closeStaleDirectConns(c)
	}()
}

func (w *staleConnsWatcher) closeStaleDirectConns(relayed network.Conn) {
	p := relayed.RemotePeer()
	arrived := relayed.Stat().Opened
	for _, c := range w.h.Network().ConnsToPeer(p) {
		st := c.Stat()
		if st.Limited || arrived.Sub(st.Opened) < w.grace {
			continue
		}
		w.log.Debugf("closing stale direct connection to %s on %s: relayed connection arrived", p, c.RemoteMultiaddr())
		c.Close()
	}
}
