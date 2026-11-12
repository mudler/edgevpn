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

package vpn

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/protocol"
)

// On the framed protocol, VPN packets to a peer travel over one stream that
// stays open, and every packet carries its length, so the receiver writes
// each one to its interface on its own. A peer that speaks only the unframed
// protocol gets a new stream for every packet: its receiver copies a stream
// to the interface as it reads it, which is correct only when the stream
// holds exactly one packet.

const frameHeaderLen = 2 // big-endian packet length

// rawStream is the part of a libp2p stream a link uses.
type rawStream interface {
	io.Reader
	io.Writer
	Close() error
	Reset() error
}

// link sends packets to one peer over the framed protocol.
type link struct {
	s   rawStream
	mu  sync.Mutex
	buf []byte
}

func newLink(s rawStream) *link {
	return &link{s: s}
}

// send writes pkt as one frame.
func (l *link) send(pkt []byte) error {
	if len(pkt) > 0xffff {
		return fmt.Errorf("packet of %d bytes is too large to frame", len(pkt))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf[:0], byte(len(pkt)>>8), byte(len(pkt)))
	l.buf = append(l.buf, pkt...)
	_, err := l.s.Write(l.buf)
	return err
}

// recheckAllowed is how often a framed stream's sender is checked again
// against the peer table or the ledger while the stream stays open.
const recheckAllowed = time.Second

var errNotAllowed = errors.New("the sender is no longer allowed")

// recheckedStream fails a read once allowed reports false, checking at most
// every interval. The check runs after the read returns, so bytes that arrive
// after a quiet spell are checked before anyone sees them.
type recheckedStream struct {
	rawStream
	allowed func() bool
	every   time.Duration
	checked time.Time
}

func (s *recheckedStream) Read(p []byte) (int, error) {
	n, err := s.rawStream.Read(p)
	if now := time.Now(); now.Sub(s.checked) >= s.every {
		if !s.allowed() {
			return 0, errNotAllowed
		}
		s.checked = now
	}
	return n, err
}

// receiveFramed reads frames from s and writes each packet to ifce. It
// returns when s ends or fails.
func receiveFramed(s rawStream, ifce io.Writer) error {
	// The buffer holds the largest packet, so each packet is written to ifce
	// straight from it.
	r := bufio.NewReaderSize(s, 0xffff)
	var hdr [frameHeaderLen]byte
	for {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		n := int(binary.BigEndian.Uint16(hdr[:]))
		pkt, err := r.Peek(n)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
		// A packet the interface refuses is lost like any other; the
		// stream carries on.
		ifce.Write(pkt)
		r.Discard(n)
	}
}

// links holds the framed stream to each peer that speaks the framed
// protocol.
type links struct {
	h     host.Host
	debug func(format string, args ...interface{})
	mu    sync.Mutex
	peers map[peer.ID]*peerLink
}

// peerLink is the framed stream to one peer. Senders take turns. A write
// that blocks ends at its packet's deadline, so a peer that stops reading
// holds each packet sent to it for at most that packet's timeout.
type peerLink struct {
	mu     sync.Mutex
	framed atomic.Pointer[link]
}

// writeDeadliner is the part of a libp2p stream that bounds a blocked write.
type writeDeadliner interface {
	SetWriteDeadline(time.Time) error
}

func newLinks(h host.Host) *links {
	return &links{h: h, peers: make(map[peer.ID]*peerLink), debug: func(string, ...interface{}) {}}
}

// send sends frame to pid: on the peer's framed stream, opening it when there
// is none, when the peer announced the framed protocol through identify, and
// otherwise on a stream of its own over the unframed protocol, as edgevpn
// always did.
func (ls *links) send(ctx context.Context, pid peer.ID, frame []byte) error {
	if !ls.speaksFramed(pid) {
		return ls.sendUnframed(ctx, pid, frame)
	}

	ls.mu.Lock()
	pl, ok := ls.peers[pid]
	if !ok {
		pl = &peerLink{}
		ls.peers[pid] = pl
	}
	ls.mu.Unlock()

	pl.mu.Lock()
	defer pl.mu.Unlock()

	l := pl.framed.Load()
	if l == nil {
		s, err := ls.h.NewStream(ctx, pid, protocol.EdgeVPNFramed.ID())
		if err != nil {
			return fmt.Errorf("could not open stream to %s: %w", pid, err)
		}
		ls.debug("opened a %s stream for VPN packets to %s", s.Protocol(), pid)
		l = newLink(s)
		pl.framed.Store(l)
	}

	// A write blocks while the peer does not read. The packet's deadline
	// bounds it, and the stream, which may then hold part of a frame, is
	// reset.
	if d, ok := ctx.Deadline(); ok {
		if w, ok := l.s.(writeDeadliner); ok {
			w.SetWriteDeadline(d)
		}
	}
	if err := l.send(frame); err != nil {
		l.s.Reset()
		pl.framed.Store(nil)
		return err
	}
	return nil
}

// speaksFramed reports whether pid announced the framed protocol. Until
// identify has run, nothing is known and the unframed protocol is used.
func (ls *links) speaksFramed(pid peer.ID) bool {
	supported, err := ls.h.Peerstore().SupportsProtocols(pid, protocol.EdgeVPNFramed.ID())
	return err == nil && len(supported) > 0
}

// sendUnframed sends frame on a new stream that carries only this packet.
// Opening and closing the stream takes a round trip, so it holds no lock:
// edgevpn's workers send packets to the same peer at the same time.
func (ls *links) sendUnframed(ctx context.Context, pid peer.ID, frame []byte) error {
	s, err := ls.h.NewStream(ctx, pid, protocol.EdgeVPN.ID())
	if err != nil {
		return fmt.Errorf("could not open stream to %s: %w", pid, err)
	}
	defer s.Close()
	_, err = s.Write(frame)
	return err
}

func (ls *links) close() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	for pid, pl := range ls.peers {
		// A stream with a write in progress is reset: closing it would race
		// with the write.
		if pl.mu.TryLock() {
			if l := pl.framed.Load(); l != nil {
				l.s.Close()
			}
			pl.mu.Unlock()
		} else {
			if l := pl.framed.Load(); l != nil {
				l.s.Reset()
			}
		}
		delete(ls.peers, pid)
	}
}
