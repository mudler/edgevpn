// The MIT License (MIT)

// Copyright (c) 2017 Whyrusleeping (MIT)
// Copyright (c) 2022 Ettore Di Giacinto (Apache v2)

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// This package is a port of go-libp2p-connmgr, but adapted for streams

package stream

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("peermgr")

// Manager is a ConnManager that trims connections whenever the count exceeds the
// high watermark. New connections are given a grace period before they're subject
// to trimming. Trims are automatically run on demand, only if the time from the
// previous trim is higher than 10 seconds. Furthermore, trims can be explicitly
// requested through the public interface of this struct (see TrimOpenConns).
//
// See configuration parameters in NewConnManager.
type Manager struct {
	*decayer

	cfg      *config
	segments segments

	plk       sync.RWMutex
	protected map[peer.ID]map[string]struct{}

	// channel-based semaphore that enforces only a single trim is in progress
	trimMutex sync.Mutex
	connCount int32
	// to be accessed atomically. This is mimicking the implementation of a sync.Once.
	// Take care of correct alignment when modifying this struct.
	trimCount uint64

	lastTrimMu sync.RWMutex
	lastTrim   time.Time

	refCount                sync.WaitGroup
	ctx                     context.Context
	cancel                  func()
	unregisterMemoryWatcher func()
}

type segment struct {
	sync.Mutex
	peers map[peer.ID]*peerInfo
}

type segments [256]*segment

func (ss *segments) get(p peer.ID) *segment {
	return ss[byte(p[len(p)-1])]
}

func (ss *segments) countPeers() (count int) {
	for _, seg := range ss {
		seg.Lock()
		count += len(seg.peers)
		seg.Unlock()
	}
	return count
}

func (s *segment) tagInfoFor(p peer.ID) *peerInfo {
	pi, ok := s.peers[p]
	if ok {
		return pi
	}
	// create a temporary peer to buffer early tags before the Connected notification arrives.
	pi = &peerInfo{
		id:        p,
		firstSeen: time.Now(), // this timestamp will be updated when the first Connected notification arrives.
		temp:      true,
		tags:      make(map[string]int),
		decaying:  make(map[*decayingTag]*connmgr.DecayingValue),
		conns:     make(map[network.Stream]time.Time),
	}
	s.peers[p] = pi
	return pi
}

// NewConnManager creates a new Manager with the provided params:
// lo and hi are watermarks governing the number of connections that'll be maintained.
// When the peer count exceeds the 'high watermark', as many peers will be pruned (and
// their connections terminated) until 'low watermark' peers remain.
func NewConnManager(low, hi int, opts ...Option) (*Manager, error) {
	cfg := &config{
		highWater:     hi,
		lowWater:      low,
		gracePeriod:   10 * time.Second,
		silencePeriod: 10 * time.Second,
	}
	for _, o := range opts {
		if err := o(cfg); err != nil {
			return nil, err
		}
	}

	if cfg.decayer == nil {
		// Set the default decayer config.
		cfg.decayer = (&DecayerCfg{}).WithDefaults()
	}

	cm := &Manager{
		cfg:       cfg,
		protected: make(map[peer.ID]map[string]struct{}, 16),
		segments: func() (ret segments) {
			for i := range ret {
				ret[i] = &segment{
					peers: make(map[peer.ID]*peerInfo),
				}
			}
			return ret
		}(),
	}
	cm.ctx, cm.cancel = context.WithCancel(context.Background())

	decay, _ := NewDecayer(cfg.decayer, cm)
	cm.decayer = decay

	cm.refCount.Add(1)
	go cm.background()
	return cm, nil
}

func (cm *Manager) Close() error {
	cm.cancel()
	if cm.unregisterMemoryWatcher != nil {
		cm.unregisterMemoryWatcher()
	}
	if err := cm.decayer.Close(); err != nil {
		return err
	}
	cm.refCount.Wait()
	return nil
}

func (cm *Manager) Protect(id peer.ID, tag string) {
	cm.plk.Lock()
	defer cm.plk.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		tags = make(map[string]struct{}, 2)
		cm.protected[id] = tags
	}
	tags[tag] = struct{}{}
}

func (cm *Manager) Unprotect(id peer.ID, tag string) (protected bool) {
	cm.plk.Lock()
	defer cm.plk.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		return false
	}
	if delete(tags, tag); len(tags) == 0 {
		delete(cm.protected, id)
		return false
	}
	return true
}

func (cm *Manager) IsProtected(id peer.ID, tag string) (protected bool) {
	cm.plk.Lock()
	defer cm.plk.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		return false
	}

	if tag == "" {
		return true
	}

	_, protected = tags[tag]
	return protected
}

// peerInfo stores metadata for a given peer.
type peerInfo struct {
	id       peer.ID
	tags     map[string]int                          // value for each tag
	decaying map[*decayingTag]*connmgr.DecayingValue // decaying tags

	value int  // cached sum of all tag values
	temp  bool // this is a temporary entry holding early tags, and awaiting connections

	conns map[network.Stream]time.Time // start time of each connection

	firstSeen time.Time // timestamp when we began tracking this peer.
}

type peerInfos []peerInfo

func (p peerInfos) SortByValue() {
	sort.Slice(p, func(i, j int) bool {
		left, right := p[i], p[j]
		// temporary peers are preferred for pruning.
		if left.temp != right.temp {
			return left.temp
		}
		// otherwise, compare by value.
		return left.value < right.value
	})
}

// TrimOpenConns closes the connections of as many peers as needed to make the peer count
// equal the low watermark. Peers are sorted in ascending order based on their total value,
// pruning those peers with the lowest scores first, as long as they are not within their
// grace period.
//
// This function blocks until a trim is completed. If a trim is underway, a new
// one won't be started, and instead it'll wait until that one is completed before
// returning.
func (cm *Manager) TrimOpenConns(_ context.Context) {
	// TODO: error return value so we can cleanly signal we are aborting because:
	// (a) there's another trim in progress, or (b) the silence period is in effect.

	cm.doTrim()
}

func (cm *Manager) background() {
	defer cm.refCount.Done()

	interval := cm.cfg.gracePeriod / 2
	if cm.cfg.silencePeriod != 0 {
		interval = cm.cfg.silencePeriod
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&cm.connCount) < int32(cm.cfg.highWater) {
				// Below high water, skip.
				continue
			}
		case <-cm.ctx.Done():
			return
		}
		cm.trim()
	}
}

func (cm *Manager) doTrim() {
	// This logic is mimicking the implementation of sync.Once in the standard library.
	count := atomic.LoadUint64(&cm.trimCount)
	cm.trimMutex.Lock()
	defer cm.trimMutex.Unlock()
	if count == atomic.LoadUint64(&cm.trimCount) {
		cm.trim()
		cm.lastTrimMu.Lock()
		cm.lastTrim = time.Now()
		cm.lastTrimMu.Unlock()
		atomic.AddUint64(&cm.trimCount, 1)
	}
}

// trim starts the trim, if the last trim happened before the configured silence period.
func (cm *Manager) trim() {
	// do the actual trim.
	for _, c := range cm.getConnsToClose() {
		c.Close()
	}
}

// getConnsToClose runs the heuristics described in TrimOpenConns and returns the
// connections to close.
func (cm *Manager) getConnsToClose() []network.Stream {
	if cm.cfg.lowWater == 0 || cm.cfg.highWater == 0 {
		// disabled
		return nil
	}

	if int(atomic.LoadInt32(&cm.connCount)) <= cm.cfg.lowWater {
		log.Info("open connection count below limit")
		return nil
	}

	candidates := make(peerInfos, 0, cm.segments.countPeers())
	var ncandidates int
	gracePeriodStart := time.Now().Add(-cm.cfg.gracePeriod)

	cm.plk.RLock()
	for _, s := range cm.segments {
		s.Lock()
		for id, inf := range s.peers {
			if _, ok := cm.protected[id]; ok {
				// skip over protected peer.
				continue
			}
			if inf.firstSeen.After(gracePeriodStart) {
				// skip peers in the grace period.
				continue
			}
			// note that we're copying the entry here,
			// but since inf.conns is a map, it will still point to the original object
			candidates = append(candidates, *inf)
			ncandidates += len(inf.conns)
		}
		s.Unlock()
	}
	cm.plk.RUnlock()

	if ncandidates < cm.cfg.lowWater {
		log.Info("open connection count above limit but too many are in the grace period")
		// We have too many connections but fewer than lowWater
		// connections out of the grace period.
		//
		// If we trimmed now, we'd kill potentially useful connections.
		return nil
	}

	// Sort peers according to their value.
	candidates.SortByValue()

	target := ncandidates - cm.cfg.lowWater

	// slightly overallocate because we may have more than one conns per peer
	selected := make([]network.Stream, 0, target+10)

	for _, inf := range candidates {
		if target <= 0 {
			break
		}

		// lock this to protect from concurrent modifications from connect/disconnect events
		s := cm.segments.get(inf.id)
		s.Lock()
		if len(inf.conns) == 0 && inf.temp {
			// handle temporary entries for early tags -- this entry has gone past the grace period
			// and still holds no connections, so prune it.
			delete(s.peers, inf.id)
		} else {
			for c := range inf.conns {
				selected = append(selected, c)
			}
			target -= len(inf.conns)
		}
		s.Unlock()
	}

	return selected
}

// GetTagInfo is called to fetch the tag information associated with a given
// peer, nil is returned if p refers to an unknown peer.
func (cm *Manager) GetTagInfo(p peer.ID) *connmgr.TagInfo {
	s := cm.segments.get(p)
	s.Lock()
	defer s.Unlock()

	pi, ok := s.peers[p]
	if !ok {
		return nil
	}

	out := &connmgr.TagInfo{
		FirstSeen: pi.firstSeen,
		Value:     pi.value,
		Tags:      make(map[string]int),
		Conns:     make(map[string]time.Time),
	}

	for t, v := range pi.tags {
		out.Tags[t] = v
	}
	for t, v := range pi.decaying {
		out.Tags[t.name] = v.Value
	}
	for c, t := range pi.conns {
		out.Conns[c.ID()] = t
	}

	return out
}

// TagPeer is called to associate a string and integer with a given peer.
func (cm *Manager) TagPeer(p peer.ID, tag string, val int) {
	s := cm.segments.get(p)
	s.Lock()
	defer s.Unlock()

	pi := s.tagInfoFor(p)

	// Update the total value of the peer.
	pi.value += val - pi.tags[tag]
	pi.tags[tag] = val
}

// UntagPeer is called to disassociate a string and integer from a given peer.
func (cm *Manager) UntagPeer(p peer.ID, tag string) {
	s := cm.segments.get(p)
	s.Lock()
	defer s.Unlock()

	pi, ok := s.peers[p]
	if !ok {
		log.Info("tried to remove tag from untracked peer: ", p)
		return
	}

	// Update the total value of the peer.
	pi.value -= pi.tags[tag]
	delete(pi.tags, tag)
}

// UpsertTag is called to insert/update a peer tag
func (cm *Manager) UpsertTag(p peer.ID, tag string, upsert func(int) int) {
	s := cm.segments.get(p)
	s.Lock()
	defer s.Unlock()

	pi := s.tagInfoFor(p)

	oldval := pi.tags[tag]
	newval := upsert(oldval)
	pi.value += newval - oldval
	pi.tags[tag] = newval
}

// CMInfo holds the configuration for Manager, as well as status data.
type CMInfo struct {
	// The low watermark, as described in NewConnManager.
	LowWater int

	// The high watermark, as described in NewConnManager.
	HighWater int

	// The timestamp when the last trim was triggered.
	LastTrim time.Time

	// The configured grace period, as described in NewConnManager.
	GracePeriod time.Duration

	// The current connection count.
	ConnCount int
}

// GetInfo returns the configuration and status data for this connection manager.
func (cm *Manager) GetInfo() CMInfo {
	cm.lastTrimMu.RLock()
	lastTrim := cm.lastTrim
	cm.lastTrimMu.RUnlock()

	return CMInfo{
		HighWater:   cm.cfg.highWater,
		LowWater:    cm.cfg.lowWater,
		LastTrim:    lastTrim,
		GracePeriod: cm.cfg.gracePeriod,
		ConnCount:   int(atomic.LoadInt32(&cm.connCount)),
	}
}

// HasStream is called to retrieve a stream if it does exist for the pid
func (cm *Manager) HasStream(n network.Network, pid peer.ID) (network.Stream, error) {
	s := cm.segments.get(pid)
	s.Lock()
	defer s.Unlock()

	pinfo, ok := s.peers[pid]
	if !ok {
		return nil, errors.New("no stream available for pid")
	}

	for c, _ := range pinfo.conns {
		pinfo.conns[c] = time.Now()
		return c, nil
	}

	return nil, errors.New("no stream available")
}

// Connected is called by notifiers to inform that a new connection has been established.
// The notifee updates the Manager to start tracking the connection. If the new connection
// count exceeds the high watermark, a trim may be triggered.
func (cm *Manager) Connected(n network.Network, c network.Stream) {

	p := c.Conn().RemotePeer()
	s := cm.segments.get(p)
	s.Lock()
	defer s.Unlock()

	pinfo, ok := s.peers[p]
	if !ok {
		pinfo = &peerInfo{
			id:        p,
			firstSeen: time.Now(),
			tags:      make(map[string]int),
			decaying:  make(map[*decayingTag]*connmgr.DecayingValue),
			conns:     make(map[network.Stream]time.Time),
		}
		s.peers[p] = pinfo
	} else if pinfo.temp {
		// we had created a temporary entry for this peer to buffer early tags before the
		// Connected notification arrived: flip the temporary flag, and update the firstSeen
		// timestamp to the real one.
		pinfo.temp = false
		pinfo.firstSeen = time.Now()
	}

	// cache one stream for peer
	pinfo.conns = map[network.Stream]time.Time{c: time.Now()}
	// _, ok = pinfo.conns[c]
	// if ok {
	// 	log.Error("received connected notification for conn we are already tracking: ", p)
	// 	return
	// }

	// pinfo.conns[c] = time.Now()
	atomic.AddInt32(&cm.connCount, 1)
}

// Disconnected is called by notifiers to inform that an existing connection has been closed or terminated.
// The notifee updates the Manager accordingly to stop tracking the connection, and performs housekeeping.
func (cm *Manager) Disconnected(n network.Network, c network.Stream) {

	p := c.Conn().RemotePeer()
	s := cm.segments.get(p)
	s.Lock()
	defer s.Unlock()

	cinf, ok := s.peers[p]
	if !ok {
		log.Error("received disconnected notification for peer we are not tracking: ", p)
		return
	}

	_, ok = cinf.conns[c]
	if !ok {
		log.Error("received disconnected notification for conn we are not tracking: ", p)
		return
	}

	delete(cinf.conns, c)
	if len(cinf.conns) == 0 {
		delete(s.peers, p)
	}
	atomic.AddInt32(&cm.connCount, -1)
}
