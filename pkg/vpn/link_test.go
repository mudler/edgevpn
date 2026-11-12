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
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/mudler/edgevpn/pkg/protocol"
)

// recorder is a VPN interface that keeps every packet written to it.
// recorder is a VPN interface that keeps every write as one packet.
type recorder struct {
	mu      sync.Mutex
	packets [][]byte
}

func (r *recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.packets = append(r.packets, append([]byte(nil), p...))
	return len(p), nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.packets)
}

func (r *recorder) packet(i int) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.packets[i]
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

func packet(size int, fill byte) []byte {
	return bytes.Repeat([]byte{fill}, size)
}

// framedInput hands out a framed byte stream in reads of chunk bytes.
type framedInput struct {
	data  []byte
	chunk int
}

func (f *framedInput) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p[:min(len(p), f.chunk)], f.data)
	f.data = f.data[n:]
	return n, nil
}
func (f *framedInput) Write(p []byte) (int, error) { return len(p), nil }
func (f *framedInput) Close() error                { return nil }
func (f *framedInput) Reset() error                { return nil }

// A stream carries bytes, and one read can return several packets, or part of
// one. Each packet must still reach the VPN interface as one write.
func TestReceiverWritesEachPacketToTheInterfaceOnItsOwn(t *testing.T) {
	var want [][]byte
	var stream []byte
	for i := 0; i < 100; i++ {
		p := packet(40+i*13, byte(i))
		want = append(want, p)
		stream = binary.BigEndian.AppendUint16(stream, uint16(len(p)))
		stream = append(stream, p...)
	}
	for _, chunk := range []int{7, 1500, 64 << 10} {
		ifce := &recorder{}
		if err := receiveFramed(&framedInput{data: stream, chunk: chunk}, ifce); err != nil {
			t.Fatalf("reads of %d bytes: %v", chunk, err)
		}
		if ifce.count() != len(want) {
			t.Fatalf("reads of %d bytes: the interface got %d writes, want %d", chunk, ifce.count(), len(want))
		}
		for i, p := range want {
			if !bytes.Equal(ifce.packet(i), p) {
				t.Fatalf("reads of %d bytes: write %d has %d bytes, want packet %d of %d bytes", chunk, i, len(ifce.packet(i)), i, len(p))
			}
		}
	}
}

// hostPair returns two connected libp2p hosts on loopback QUIC, with no
// resource limits, as edgevpn runs by default. A read from a QUIC stream
// returns whatever has arrived, so packets that pile up come out of one read
// together. (Over TCP, yamux returns one frame per read, which hides that.)
func hostPair(t *testing.T) (host.Host, host.Host) {
	t.Helper()
	var hs []host.Host
	for i := 0; i < 2; i++ {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"), libp2p.ResourceManager(&network.NullResourceManager{}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { h.Close() })
		hs = append(hs, h)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs[0].Connect(ctx, peer.AddrInfo{ID: hs[1].ID(), Addrs: hs[1].Addrs()}); err != nil {
		t.Fatal(err)
	}
	return hs[0], hs[1]
}

// waitUntilAnnounced waits until a has learned through identify that b
// speaks the framed protocol, as it has for a peer whose VPN was running
// when the two connected.
func waitUntilAnnounced(t *testing.T, a host.Host, b peer.ID) {
	t.Helper()
	if !waitFor(t, 5*time.Second, func() bool {
		p, err := a.Peerstore().SupportsProtocols(b, protocol.EdgeVPNFramed.ID())
		return err == nil && len(p) > 0
	}) {
		t.Fatal("the peer never announced the framed protocol")
	}
}

// burst sends n packets of distinct sizes from a to b's peer ID as fast as
// the links allow and returns them.
func burst(t *testing.T, ls *links, to network.Network, n int) [][]byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var sent [][]byte
	for i := 0; i < n; i++ {
		p := packet(200+i, byte(i))
		if err := ls.send(ctx, to.LocalPeer(), p); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		sent = append(sent, p)
	}
	return sent
}

// A peer that runs an older edgevpn speaks only the unframed protocol and
// copies each stream to its interface as it reads it (streamHandler), so a
// stream must carry exactly one packet: with several, one read can hand the
// interface two packets in one write, or part of one.
func TestPeerOnTheUnframedProtocolGetsEachPacketOnItsOwn(t *testing.T) {
	a, b := hostPair(t)
	ifce := &recorder{}
	var streams atomic.Int32
	b.SetStreamHandler(protocol.EdgeVPN.ID(), func(s network.Stream) {
		streams.Add(1)
		// A receiver under load falls behind, and then one read returns
		// whatever has piled up in the stream.
		time.Sleep(200 * time.Millisecond)
		io.Copy(ifce, s)
		s.Close()
	})

	ls := newLinks(a)
	defer ls.close()
	sent := burst(t, ls, b.Network(), 200)

	if !waitFor(t, 5*time.Second, func() bool { return ifce.count() >= len(sent) }) {
		t.Fatalf("the older peer's interface got %d writes, want %d", ifce.count(), len(sent))
	}
	t.Logf("%d writes over %d streams", ifce.count(), streams.Load())
	got := map[string]bool{}
	for i := 0; i < ifce.count(); i++ {
		got[string(ifce.packet(i))] = true
	}
	for i, p := range sent {
		if !got[string(p)] {
			t.Fatalf("packet %d (%d bytes) never reached the older peer's interface as one write; %d writes over %d streams", i, len(p), ifce.count(), streams.Load())
		}
	}
}

// A peer on the framed protocol gets every packet over one stream.
func TestPeerOnTheFramedProtocolGetsAllPacketsOverOneStream(t *testing.T) {
	a, b := hostPair(t)
	ifce := &recorder{}
	var streams atomic.Int32
	b.SetStreamHandler(protocol.EdgeVPNFramed.ID(), func(s network.Stream) {
		streams.Add(1)
		receiveFramed(s, ifce)
	})
	b.SetStreamHandler(protocol.EdgeVPN.ID(), func(s network.Stream) { s.Reset() })

	waitUntilAnnounced(t, a, b.ID())
	ls := newLinks(a)
	defer ls.close()
	sent := burst(t, ls, b.Network(), 50)

	if !waitFor(t, 5*time.Second, func() bool { return ifce.count() == len(sent) }) {
		t.Fatalf("got %d packets over the framed protocol, want %d", ifce.count(), len(sent))
	}
	for i, p := range sent {
		if !bytes.Equal(ifce.packet(i), p) {
			t.Fatalf("write %d has %d bytes, want packet %d of %d bytes", i, len(ifce.packet(i)), i, len(p))
		}
	}
	if n := streams.Load(); n != 1 {
		t.Fatalf("the packets used %d streams, want 1", n)
	}
}

// When the stream to a peer breaks, the next packet opens a new one.
func TestLinksOpenANewStreamAfterTheOldOneBreaks(t *testing.T) {
	a, b := hostPair(t)
	ifce := &recorder{}
	var mu sync.Mutex
	var open []network.Stream
	b.SetStreamHandler(protocol.EdgeVPNFramed.ID(), func(s network.Stream) {
		mu.Lock()
		open = append(open, s)
		mu.Unlock()
		receiveFramed(s, ifce)
	})

	waitUntilAnnounced(t, a, b.ID())
	ls := newLinks(a)
	defer ls.close()
	burst(t, ls, b.Network(), 1)
	if !waitFor(t, 2*time.Second, func() bool { return ifce.count() == 1 }) {
		t.Fatal("the first packet did not arrive")
	}
	mu.Lock()
	open[0].Reset()
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !waitFor(t, 5*time.Second, func() bool {
		ls.send(ctx, b.ID(), packet(300, 9))
		return ifce.count() > 1
	}) {
		t.Fatal("no packet arrived after the stream broke")
	}
}

// edgevpn sends packets from several workers at once. Packets to a peer on
// the unframed protocol each open, write and close a stream, which takes a
// round trip, so they must not wait for each other.
func TestPacketsToAPeerOnTheUnframedProtocolGoOutInParallel(t *testing.T) {
	mn := mocknet.New()
	defer mn.Close()
	a, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	b, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	const oneWay = 25 * time.Millisecond
	if _, err := mn.LinkPeers(a.ID(), b.ID()); err != nil {
		t.Fatal(err)
	}
	mn.SetLinkDefaults(mocknet.LinkOptions{Latency: oneWay})
	for _, l := range mn.LinksBetweenPeers(a.ID(), b.ID()) {
		l.SetOptions(mocknet.LinkOptions{Latency: oneWay})
	}
	if _, err := mn.ConnectPeers(a.ID(), b.ID()); err != nil {
		t.Fatal(err)
	}
	ifce := &recorder{}
	b.SetStreamHandler(protocol.EdgeVPN.ID(), func(s network.Stream) {
		io.Copy(ifce, s)
		s.Close()
	})

	ls := newLinks(a)
	defer ls.close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// The first packet lets libp2p learn which protocol the peer speaks.
	if err := ls.send(ctx, b.ID(), packet(100, 0)); err != nil {
		t.Fatal(err)
	}

	const workers, each = 8, 5
	start := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if err := ls.send(ctx, b.ID(), packet(100+w*each+i, byte(w))); err != nil {
					t.Error(err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	took := time.Since(start)
	t.Logf("%d packets from %d workers in %s", workers*each, workers, took)
	// One after the other they would take workers*each round trips.
	if serial := time.Duration(workers*each) * 2 * oneWay; took > serial/2 {
		t.Fatalf("%d packets took %s, close to the %s they take one round trip after the other", workers*each, took, serial)
	}
}

// blockingInterface takes packets until stop is closed, then blocks every
// write, as an interface on a stuck machine does.
type blockingInterface struct {
	stop chan struct{}
	n    atomic.Int64
}

func (b *blockingInterface) Write(p []byte) (int, error) {
	select {
	case <-b.stop:
		select {}
	default:
	}
	b.n.Add(int64(len(p)))
	return len(p), nil
}

// stuckPeer connects a to a new host whose interface stops taking packets
// after a second of fast traffic, and starts sending to it from a for six
// seconds, from several goroutines as edgevpn's workers do, with the given
// timeout per packet. It returns how long the oldest pending send has been
// pending, zero when none is. A send blocks once the peer stops reading and
// the stream's buffers are full.
func stuckPeer(t *testing.T, a host.Host, ls *links, timeout time.Duration) func() time.Duration {
	t.Helper()
	stuck, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"), libp2p.ResourceManager(&network.NullResourceManager{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stuck.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Connect(ctx, peer.AddrInfo{ID: stuck.ID(), Addrs: stuck.Addrs()}); err != nil {
		t.Fatal(err)
	}
	ifce := &blockingInterface{stop: make(chan struct{})}
	stuck.SetStreamHandler(protocol.EdgeVPNFramed.ID(), func(s network.Stream) { receiveFramed(s, ifce) })
	waitUntilAnnounced(t, a, stuck.ID())

	const senders = 4
	pendingSince := make([]atomic.Int64, senders)
	var wg sync.WaitGroup
	time.AfterFunc(time.Second, func() { close(ifce.stop) })
	end := time.Now().Add(6 * time.Second)
	for i := range pendingSince {
		wg.Add(1)
		go func(since *atomic.Int64) {
			defer wg.Done()
			p := packet(1200, 1)
			for time.Now().Before(end) {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				since.Store(time.Now().UnixNano())
				ls.send(ctx, stuck.ID(), p)
				since.Store(0)
				cancel()
			}
		}(&pendingSince[i])
	}
	// Closing the host ends a write that is still blocked.
	t.Cleanup(func() { stuck.Close(); wg.Wait() })
	pendingFor := func() time.Duration {
		var longest time.Duration
		for i := range pendingSince {
			if since := pendingSince[i].Load(); since != 0 {
				longest = max(longest, time.Since(time.Unix(0, since)))
			}
		}
		return longest
	}
	if !waitFor(t, 4*time.Second, func() bool { return pendingFor() > 300*time.Millisecond }) {
		t.Fatal("no send to the stuck peer blocked")
	}
	return pendingFor
}

// Sends to a peer that stops reading its stream end when their context runs
// out. Meanwhile a send to another peer from another goroutine carries on.
// (edgevpn's workers share one queue, so in a running node packets to other
// peers can still wait up to the timeout behind the stuck ones, as they do
// on the unframed protocol.)
func TestSendsToAPeerThatStopsReadingEndAtTheirTimeout(t *testing.T) {
	a, healthy := hostPair(t)
	healthyIfce := &recorder{}
	healthy.SetStreamHandler(protocol.EdgeVPNFramed.ID(), func(s network.Stream) { receiveFramed(s, healthyIfce) })
	waitUntilAnnounced(t, a, healthy.ID())
	ls := newLinks(a)
	defer ls.close()
	const timeout = 3 * time.Second
	pendingFor := stuckPeer(t, a, ls, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sent := make(chan error, 1)
	go func() { sent <- ls.send(ctx, healthy.ID(), packet(300, 2)) }()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatalf("sending to the healthy peer: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("a packet to the healthy peer waited behind the stuck peer")
	}
	if !waitFor(t, 2*time.Second, func() bool { return healthyIfce.count() == 1 }) {
		t.Fatal("the healthy peer never got its packet")
	}

	// Well after the first send blocked, no send has been pending for longer
	// than its context allows.
	time.Sleep(timeout + time.Second)
	if d := pendingFor(); d > timeout+500*time.Millisecond {
		t.Fatalf("a send to the stuck peer has been pending for %v with a %v context", d, timeout)
	}
}

// Closing the links resets a stream with a blocked write instead of waiting
// for the write.
func TestClosingTheLinksDoesNotWaitForAStuckPeer(t *testing.T) {
	a, _ := hostPair(t)
	ls := newLinks(a)
	stuckPeer(t, a, ls, 3*time.Second)

	closed := make(chan struct{})
	go func() { ls.close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("closing the links waited behind the stuck peer")
	}
}

// A sender is checked again while its framed stream stays open, as it is for
// every stream on the unframed protocol. Once it is no longer allowed, its
// stream is reset and nothing more it sends reaches the interface.
func TestAFramedStreamEndsWhenItsSenderIsNoLongerAllowed(t *testing.T) {
	a, b := hostPair(t)
	ifce := &recorder{}
	var allowed atomic.Bool
	allowed.Store(true)
	b.SetStreamHandler(protocol.EdgeVPNFramed.ID(), framedStreamHandler(func(peer.ID) bool { return allowed.Load() }, ifce, 50*time.Millisecond))
	waitUntilAnnounced(t, a, b.ID())
	ls := newLinks(a)
	defer ls.close()

	burst(t, ls, b.Network(), 1)
	if !waitFor(t, 2*time.Second, func() bool { return ifce.count() == 1 }) {
		t.Fatal("the first packet did not arrive")
	}
	allowed.Store(false)
	time.Sleep(100 * time.Millisecond)
	before := ifce.count()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < 50; i++ {
		ls.send(ctx, b.ID(), packet(300, 3))
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	// The receiver was already waiting in a read when the sender went
	// quiet, so what arrives next comes out of a read that began while
	// the sender was allowed.
	if got := ifce.count() - before; got > 0 {
		t.Fatalf("%d packets reached the interface after the sender was no longer allowed", got)
	}
}

// A timeout well under a second still leaves every write on a healthy stream
// its full time.
func TestShortTimeoutsDoNotBreakAHealthyStream(t *testing.T) {
	a, b := hostPair(t)
	ifce := &recorder{}
	var streams atomic.Int32
	b.SetStreamHandler(protocol.EdgeVPNFramed.ID(), func(s network.Stream) {
		streams.Add(1)
		receiveFramed(s, ifce)
	})
	waitUntilAnnounced(t, a, b.ID())
	ls := newLinks(a)
	defer ls.close()
	for i := 0; i < 150; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		if err := ls.send(ctx, b.ID(), packet(300, byte(i))); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		cancel()
		time.Sleep(10 * time.Millisecond)
	}
	if n := streams.Load(); n != 1 {
		t.Fatalf("the packets used %d streams, want 1", n)
	}
}
