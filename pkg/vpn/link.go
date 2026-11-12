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
//
// A stream is reliable and buffers whatever it is given, so when the sender
// outruns the network, packets would wait in it for seconds instead of being
// dropped, and the TCP connections inside the VPN would never see the loss
// they slow down on. So the receiver reports how many bytes it has read. The
// sender keeps the bytes in flight near what the path delivers in a round
// trip, and drops packets beyond that, as a router with a short queue does.
// Dropped packets are gone: TCP inside the VPN resends them and slows down,
// other traffic copes as it does with any lossy link.

const (
	frameHeaderLen = 2 // big-endian packet length
	ackLen         = 8 // big-endian count of bytes read, headers included

	// The receiver reports at least this often, and whenever it has read
	// everything the stream holds.
	ackEvery = 16 << 10

	// Limits on the bytes in flight. The limit starts at initialInflight and,
	// once the link has been busy, follows inflightGain times the bytes the
	// path delivers in its shortest round trip.
	//
	// inflightGain trades speed for delay. Up to one round trip's worth of
	// bytes is on the wire, so with a gain of 2 up to one more round trip's
	// worth can wait in the queue, and on a congested path a packet waits at
	// most about one extra round trip. A higher gain wastes less of the path
	// when the rate estimate runs low, but lets the queue, and the wait for
	// everything else sent to the peer, grow by a round trip for each step.
	// A lower gain keeps the queue shorter but drops packets the path could
	// have carried, and TCP inside the VPN slows down on each drop.
	initialInflight = 64 << 10
	minInflight     = 32 << 10
	maxInflight     = 8 << 20
	inflightGain    = 2

	// While the delivery rate still grows, as when TCP inside the VPN starts
	// by doubling what it sends every round trip, the limit leaves room for
	// that growth with startupGain. Startup ends when the rate has failed
	// for plateauSamples round trips in a row to beat its best by
	// plateauGrowth.
	startupGain = 4

	// The round trip the limit counts is at least limitRTTFloor. On a fast
	// path with a round trip far below a millisecond, the bytes the path
	// carries in one round trip are fewer than arrive together in one burst
	// of transport acknowledgements. This keeps at most twice this long of
	// queue on such a path.
	limitRTTFloor  = 5 * time.Millisecond
	plateauGrowth  = 1.25
	plateauSamples = 3

	// A send time is kept for every rttMarkEvery bytes, to measure the round
	// trip and the delivery rate when the receiver reports them.
	rttMarkEvery = 4 << 10

	// The shortest round trip and the highest delivery rate are each kept
	// for this long, so a path that gets slower is followed.
	minRTTWindow  = 10 * time.Second
	maxRateWindow = 2 * time.Second
)

// rawStream is the part of a libp2p stream a link uses.
type rawStream interface {
	io.Reader
	io.Writer
	Close() error
	Reset() error
}

// sendMark records, for one sent byte offset, when it was sent and how much
// the receiver had reported by then. When the receiver reports the offset,
// the bytes delivered since the mark over the time since give one delivery
// rate sample.
type sendMark struct {
	offset      uint64
	at          time.Time
	delivered   uint64
	deliveredAt time.Time
	// busy marks a sample taken while at least half the limit was in
	// flight. Only such a sample shows what the path carries, rather than
	// how little the sender had to send.
	busy bool
}

// link sends packets to one peer over the framed protocol.
type link struct {
	s rawStream

	// writeMu keeps frames whole on the stream. mu guards the rest and is
	// never held during a write, which can block on flow control while the
	// receiver's reports keep arriving.
	writeMu sync.Mutex
	buf     []byte

	mu      sync.Mutex
	err     error
	sent    uint64 // bytes written, headers included
	acked   uint64 // bytes the receiver reported
	ackedAt time.Time
	limit   uint64
	drops   uint64

	marks    []sendMark
	minRTT   time.Duration
	minRTTAt time.Time
	maxRate  float64 // bytes per second
	rateAt   time.Time
	rateSeen bool // a busy sample has been taken

	// Startup: the best busy rate so far, how many round trips in a row
	// have not beaten it by plateauGrowth, and when the last round began.
	plateauRate float64
	plateauRuns int
	roundAt     time.Time
}

func newLink(s rawStream) *link {
	l := &link{s: s, limit: initialInflight}
	go l.readAcks()
	return l
}

// send writes pkt as one frame. It returns false, without an error, when the
// packet is dropped because the bytes in flight would exceed the limit. At
// least one packet is always allowed in flight, whatever the limit.
func (l *link) send(pkt []byte) (bool, error) {
	if len(pkt) > 0xffff {
		return false, fmt.Errorf("packet of %d bytes is too large to frame", len(pkt))
	}
	l.writeMu.Lock()
	defer l.writeMu.Unlock()

	l.mu.Lock()
	if l.err != nil {
		err := l.err
		l.mu.Unlock()
		return false, err
	}
	size := uint64(frameHeaderLen + len(pkt))
	inflight := l.sent - l.acked
	if inflight > 0 && inflight+size > l.limit {
		l.drops++
		l.mu.Unlock()
		return false, nil
	}
	now := time.Now()
	l.sent += size
	if n := len(l.marks); n == 0 || l.sent-l.marks[n-1].offset >= rttMarkEvery {
		m := sendMark{offset: l.sent, at: now, delivered: l.acked, deliveredAt: l.ackedAt, busy: inflight+size >= l.limit/2}
		if inflight == 0 {
			// The path was idle: a rate sample must not count the idle time.
			m.deliveredAt = now
		}
		l.marks = append(l.marks, m)
	}
	l.mu.Unlock()

	l.buf = append(l.buf[:0], byte(len(pkt)>>8), byte(len(pkt)))
	l.buf = append(l.buf, pkt...)
	if _, err := l.s.Write(l.buf); err != nil {
		l.fail(err)
		return false, err
	}
	return true, nil
}

func (l *link) inflight() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sent - l.acked
}

func (l *link) dropped() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.drops
}

func (l *link) readAcks() {
	var b [ackLen]byte
	for {
		if _, err := io.ReadFull(l.s, b[:]); err != nil {
			l.fail(err)
			return
		}
		now := time.Now()
		l.mu.Lock()
		err := l.onAck(binary.BigEndian.Uint64(b[:]), now)
		l.mu.Unlock()
		if err != nil {
			l.fail(err)
			l.s.Reset()
			return
		}
	}
}

func (l *link) fail(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err == nil {
		l.err = err
	}
}

// onAck takes the receiver's count of bytes read and updates the round trip,
// the delivery rate and the limit from it. l.mu must be held.
func (l *link) onAck(n uint64, now time.Time) error {
	if n < l.acked || n > l.sent {
		return fmt.Errorf("peer acknowledged %d bytes, %d were sent and %d acknowledged before", n, l.sent, l.acked)
	}
	l.acked, l.ackedAt = n, now

	for len(l.marks) > 0 && l.marks[0].offset <= n {
		m := l.marks[0]
		l.marks = l.marks[1:]
		if rtt := now.Sub(m.at); l.minRTT == 0 || rtt <= l.minRTT || now.Sub(l.minRTTAt) > minRTTWindow {
			l.minRTT, l.minRTTAt = rtt, now
		}
		if elapsed := now.Sub(m.deliveredAt); elapsed > 0 {
			l.rateSample(float64(n-m.delivered)/elapsed.Seconds(), m.busy, now)
		}
	}

	if l.rateSeen && l.minRTT > 0 {
		gain := float64(inflightGain)
		if l.plateauRuns < plateauSamples {
			gain = startupGain
		}
		limit := uint64(gain * l.maxRate * max(l.minRTT, limitRTTFloor).Seconds())
		l.limit = min(max(limit, minInflight), maxInflight)
	}
	return nil
}

// rateSample takes one delivery rate sample. A sample from a link that was
// not busy can only raise the rate.
func (l *link) rateSample(rate float64, busy bool, now time.Time) {
	if rate >= l.maxRate || (busy && now.Sub(l.rateAt) > maxRateWindow) {
		l.maxRate, l.rateAt = rate, now
	}
	if !busy {
		return
	}
	l.rateSeen = true
	// Startup ends after plateauSamples round trips without growth.
	if now.Sub(l.roundAt) < l.minRTT {
		return
	}
	l.roundAt = now
	if l.maxRate >= plateauGrowth*l.plateauRate {
		l.plateauRate, l.plateauRuns = l.maxRate, 0
	} else if l.plateauRuns < plateauSamples {
		l.plateauRuns++
	}
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

// receiveFramed reads frames from s and writes each packet to ifce, and
// reports the bytes read back on s. It returns when s ends or fails.
func receiveFramed(s rawStream, ifce io.Writer) error {
	// The buffer holds the largest packet, so each packet is written to ifce
	// straight from it.
	r := bufio.NewReaderSize(s, 0xffff)
	var hdr [frameHeaderLen]byte
	var ack [ackLen]byte
	var read, reported uint64
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
		read += uint64(frameHeaderLen + n)
		if read-reported >= ackEvery || r.Buffered() == 0 {
			binary.BigEndian.PutUint64(ack[:], read)
			if _, err := s.Write(ack[:]); err != nil {
				return err
			}
			reported = read
		}
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
// always did. A packet the framed stream drops is not an error.
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
	if _, err := l.send(frame); err != nil {
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

// dropped returns, per peer, the packets dropped on the current framed stream.
func (ls *links) dropped() map[peer.ID]uint64 {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	out := make(map[peer.ID]uint64)
	for pid, pl := range ls.peers {
		if l := pl.framed.Load(); l != nil {
			out[pid] = l.dropped()
		}
	}
	return out
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
