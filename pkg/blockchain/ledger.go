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

package blockchain

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/utils"

	"github.com/pkg/errors"
)

type Ledger struct {
	sync.Mutex
	blockchain Store

	channel io.Writer

	// Authentication / ownership (see docs/design/authenticated-ledger.md).
	// When signer is set, writes are signed; the mode controls whether Update
	// runs the authorized merge and whether violations are dropped or logged.
	signer   Signer
	mode     OwnershipMode
	registry Registry
	ttl      time.Duration
	clock    func() time.Time
	warn     func(string, ...interface{})
}

// OwnershipMode selects how the ledger handles authenticated buckets.
type OwnershipMode int

const (
	// OwnershipOff: legacy height-wins replace; entries unsigned (default).
	OwnershipOff OwnershipMode = iota
	// OwnershipObserve: run the authorized merge but accept-and-warn on
	// violations instead of dropping (safe rollout / observation).
	OwnershipObserve
	// OwnershipEnforce: run the authorized merge and drop invalid writes.
	OwnershipEnforce
)

type Store interface {
	Add(Block)
	Len() int
	Last() Block
}

// LedgerOption configures optional ledger behaviour.
type LedgerOption func(*Ledger)

// WithSigner makes the ledger sign every write with s. Without enforcement this
// only annotates entries (so a network can pre-sign before flipping enforcement
// on); with enforcement it is required for the local node to author entries.
func WithSigner(s Signer) LedgerOption { return func(l *Ledger) { l.signer = s } }

// WithOwnership switches Update to the per-key authorized merge using the given
// policy registry, liveness window and mode (observe = log-only, enforce = drop).
func WithOwnership(mode OwnershipMode, r Registry, ttl time.Duration) LedgerOption {
	return func(l *Ledger) {
		l.mode = mode
		l.registry = r
		l.ttl = ttl
	}
}

// WithEnforcedOwnership is shorthand for WithOwnership(OwnershipEnforce, ...).
func WithEnforcedOwnership(r Registry, ttl time.Duration) LedgerOption {
	return WithOwnership(OwnershipEnforce, r, ttl)
}

// WithViolationLogger sets the sink for ownership-violation warnings.
func WithViolationLogger(f func(string, ...interface{})) LedgerOption {
	return func(l *Ledger) { l.warn = f }
}

// WithClock overrides the time source (tests).
func WithClock(f func() time.Time) LedgerOption { return func(l *Ledger) { l.clock = f } }

// SetSigner installs the signing key (called once the host identity exists).
func (l *Ledger) SetSigner(s Signer) {
	l.Lock()
	l.signer = s
	l.Unlock()
}

// SetOwnership configures the merge mode/registry/ttl at runtime (called during
// node startup before the message hub is running).
func (l *Ledger) SetOwnership(mode OwnershipMode, r Registry, ttl time.Duration) {
	l.Lock()
	l.mode, l.registry, l.ttl = mode, r, ttl
	l.Unlock()
}

// New returns a new ledger which writes to the writer
func New(w io.Writer, s Store, opts ...LedgerOption) *Ledger {
	c := &Ledger{channel: w, blockchain: s, clock: time.Now, warn: log.Printf}
	for _, o := range opts {
		o(c)
	}
	if c.clock == nil {
		c.clock = time.Now
	}
	if c.warn == nil {
		c.warn = log.Printf
	}
	if s.Len() == 0 {
		c.newGenesis()
	}
	return c
}

func (l *Ledger) newGenesis() {
	t := l.clock()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), map[string]map[string]SignedData{}, genesisBlock.Checksum(), ""}
	l.blockchain.Add(genesisBlock)
}

// Syncronizer starts a goroutine which
// writes the blockchain to the  periodically
func (l *Ledger) Syncronizer(ctx context.Context, t time.Duration) {
	go func() {
		t := utils.NewBackoffTicker(utils.BackoffMaxInterval(t))
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.Lock()

				bytes, err := json.Marshal(l.blockchain.Last())
				if err != nil {
					log.Println(err)
				}

				l.channel.Write(compress(bytes).Bytes())

				l.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func compress(b []byte) *bytes.Buffer {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(b)
	gz.Close()
	return &buf
}

func deCompress(b []byte) (*bytes.Buffer, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	result, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(result), nil
}

// Update the blockchain from a message
func (l *Ledger) Update(f *Ledger, h *hub.Message, c chan *hub.Message) (err error) {
	block := &Block{}

	b, err := deCompress([]byte(h.Message))
	if err != nil {
		err = errors.Wrap(err, "failed decompressing")
		return
	}

	err = json.Unmarshal(b.Bytes(), block)
	if err != nil {
		err = errors.Wrap(err, "failed unmarshalling blockchain data")
		return
	}

	if l.mode == OwnershipOff {
		l.Lock()
		// Legacy path: adopt the incoming block when it is higher (height wins),
		// or — on an exact height tie — when its hash sorts higher. This is a
		// whole-block replace and is the behaviour when ownership enforcement is
		// disabled (the default), keeping existing networks unchanged.
		last := l.blockchain.Last()
		if block.Index > last.Index ||
			(block.Index == last.Index && block.Hash > last.Hash) {
			l.blockchain.Add(*block)
		}
		l.Unlock()
		return
	}

	// Enforced path: authenticated per-key merge, applied atomically. We do not
	// re-broadcast on adoption (the Syncronizer propagates state) to avoid gossip
	// amplification.
	l.commit(false, func(cur map[string]map[string]SignedData) bool {
		_, changed := l.merge(cur, block, l.clock())
		return changed
	})
	return
}

// merge applies each entry of an incoming block into cur, enforcing the
// per-bucket ownership policy. Returns the merged storage and whether anything
// changed.
func (l *Ledger) merge(cur map[string]map[string]SignedData, incoming *Block, now time.Time) (map[string]map[string]SignedData, bool) {
	health := projectValues(cur[protocol.HealthCheckKey])
	changed := false

	for bucket, kv := range incoming.Storage {
		pol := l.registry.Policy(bucket)
		for key, in := range kv {
			if cur[bucket] == nil {
				cur[bucket] = map[string]SignedData{}
			}
			ex, ok := cur[bucket][key]

			// Idle networks re-broadcast the same block every sync interval, so a
			// byte-identical incoming entry is the common case: skip it silently
			// (no new block, no warning).
			if ok && sameEntry(ex, in) {
				continue
			}

			if !pol.Owned {
				// Open/legacy bucket: take the strictly higher version, else keep.
				if !ok || in.Version > ex.Version {
					cur[bucket][key] = in
					changed = true
				}
				continue
			}

			if reason := l.accept(bucket, key, in, ex, ok, pol, health, now); reason != "" {
				// Rejected by policy. In observe mode we log and accept anyway so
				// operators can see violations without breaking a live network.
				if l.mode == OwnershipObserve {
					l.warn("ownership violation (observe, accepting): %s/%s from %s: %s", bucket, key, in.Owner, reason)
					cur[bucket][key] = in
					changed = true
				} else {
					l.warn("ownership violation (rejected): %s/%s from %s: %s", bucket, key, in.Owner, reason)
				}
				continue
			}
			cur[bucket][key] = in
			changed = true
		}
	}
	return cur, changed
}

// accept decides whether an authenticated entry may overwrite the existing one.
// It returns "" to accept, or a short reason describing the rejection.
func (l *Ledger) accept(bucket, key string, in, ex SignedData, exists bool, pol BucketPolicy, health map[string]Data, now time.Time) string {
	// Signature must be valid and bound to the claimed owner.
	if err := Verify(bucket, key, in); err != nil {
		return "invalid signature"
	}

	if in.Deleted {
		// A tombstone may be authored by the current owner, or by anyone once
		// the current owner's lease has expired (the reaper). It must out-version
		// what it deletes.
		if !exists {
			return "tombstone for unknown key"
		}
		if in.Owner != ex.Owner && !l.expired(bucket, key, ex, pol, health, now) {
			return "tombstone by non-owner of a live entry"
		}
		if in.Version <= ex.Version {
			return "stale tombstone"
		}
		return ""
	}

	// For buckets whose value declares an owner, it must match the signer. A
	// nil OwnerOf marks a self-owned bucket (e.g. dns): the value carries no
	// owner, so the signer is the owner and the key is claimed first-come.
	if pol.OwnerOf != nil && pol.OwnerOf(key, in.Value) != in.Owner {
		return "value owner does not match signer"
	}

	if !exists {
		return "" // first claim of a free key
	}

	// An existing tombstone is a cleared slot, not a live entry: any valid owner
	// (incl. the original owner returning after a reap) may re-claim it, subject
	// only to version monotonicity. Without this, a leader-authored tombstone
	// would lock the returning owner out until tombstone GC.
	if ex.Deleted {
		if in.Version <= ex.Version {
			return "stale re-claim over tombstone"
		}
		return ""
	}

	// A different owner may only take over an expired (or reclaimable-free) slot.
	if in.Owner != ex.Owner && !l.expired(bucket, key, ex, pol, health, now) {
		return "overwrite of a live entry owned by another peer"
	}

	if in.Version < ex.Version {
		return "rollback to an older version"
	}
	if in.Version == ex.Version {
		// Deterministic tie-break so every node converges on the same winner.
		if in.Owner > ex.Owner || (in.Owner == ex.Owner && string(in.Sig) > string(ex.Sig)) {
			return ""
		}
		return "lost deterministic tie-break"
	}
	return ""
}

// expired reports whether the existing entry is past its lease and may be taken
// over by another owner.
func (l *Ledger) expired(bucket, key string, ex SignedData, pol BucketPolicy, health map[string]Data, now time.Time) bool {
	switch pol.Expiry {
	case Absolute:
		return time.Unix(ex.UpdatedAt, 0).Add(pol.TTL).Before(now)
	case Liveness:
		owner := ex.Owner
		if owner == "" && pol.OwnerOf != nil {
			owner = pol.OwnerOf(key, ex.Value)
		}
		return !IsLive(health, owner, l.ttl, now)
	default:
		return false
	}
}

// Announce keeps updating async data to the blockchain.
// Sends a broadcast at the specified interval
// by making sure the async retrieved value is written to the
// blockchain
func (l *Ledger) Announce(ctx context.Context, d time.Duration, async func()) {
	go func() {
		t := utils.NewBackoffTicker(utils.BackoffMaxInterval(d))
		defer t.Stop()
		for {
			select {
			case <-t.C:
				async()

			case <-ctx.Done():
				return
			}
		}
	}()
}

// AnnounceDeleteBucket Announce a deletion of a bucket. It stops when the bucket is deleted
// It takes an interval time and a max timeout.
// It is best effort, and the timeout is necessary, or we might flood network with requests
// if more writers are attempting to write to the same resource
func (l *Ledger) AnnounceDeleteBucket(ctx context.Context, interval, timeout time.Duration, bucket string) {
	del, cancel := context.WithTimeout(ctx, timeout)

	l.Announce(del, interval, func() {
		_, exists := l.CurrentData()[bucket]
		if exists {
			l.DeleteBucket(bucket)
		} else {
			cancel()
		}
	})
}

// AnnounceDeleteBucketKey Announce a deletion of a key from a bucket. It stops when the key is deleted
func (l *Ledger) AnnounceDeleteBucketKey(ctx context.Context, interval, timeout time.Duration, bucket, key string) {
	del, cancel := context.WithTimeout(ctx, timeout)

	l.Announce(del, interval, func() {
		_, exists := l.CurrentData()[bucket][key]
		if exists {
			l.Delete(bucket, key)
		} else {
			cancel()
		}
	})
}

// AnnounceUpdate Keeps announcing something into the blockchain if state is differing
func (l *Ledger) AnnounceUpdate(ctx context.Context, interval time.Duration, bucket, key string, value interface{}) {
	l.Announce(ctx, interval, func() {
		v, exists := l.CurrentData()[bucket][key]
		realv, _ := json.Marshal(value)
		switch {
		case !exists || string(v) != string(realv):
			l.Add(bucket, map[string]interface{}{key: value})
		}
	})
}

// Persist Keeps announcing something into the blockchain until it is reconciled
func (l *Ledger) Persist(ctx context.Context, interval, timeout time.Duration, bucket, key string, value interface{}) {
	put, cancel := context.WithTimeout(ctx, timeout)

	l.Announce(put, interval, func() {
		v, exists := l.CurrentData()[bucket][key]
		realv, _ := json.Marshal(value)
		switch {
		case !exists || string(v) != string(realv):
			l.Add(bucket, map[string]interface{}{key: value})
		case exists && string(v) == string(realv):
			cancel()
		}
	})
}

// GetKey retrieve the current key from the blockchain
func (l *Ledger) GetKey(b, s string) (value Data, exists bool) {
	l.Lock()
	defer l.Unlock()

	if l.blockchain.Len() > 0 {
		last := l.blockchain.Last()
		bkt, ok := last.Storage[b]
		if !ok {
			return
		}
		e, ok := bkt[s]
		if !ok || e.Deleted {
			return
		}
		return e.Value, true
	}
	return
}

// Exists returns true if there is one element with a matching value
func (l *Ledger) Exists(b string, f func(Data) bool) (exists bool) {
	l.Lock()
	defer l.Unlock()
	if l.blockchain.Len() > 0 {
		for _, bv := range l.blockchain.Last().Storage[b] {
			if bv.Deleted {
				continue
			}
			if f(bv.Value) {
				exists = true
				return
			}
		}
	}

	return
}

// CurrentData returns the current ledger data (locking). Tombstoned entries are
// hidden and only the inner Value is exposed, so existing readers are unchanged.
func (l *Ledger) CurrentData() map[string]map[string]Data {
	l.Lock()
	defer l.Unlock()

	out := map[string]map[string]Data{}
	for b, kv := range l.blockchain.Last().Storage {
		out[b] = projectValues(kv)
	}
	return out
}

// CurrentStorage returns a deep copy of the raw signed storage (locking). Used
// by the reaper, which needs owner/version metadata.
func (l *Ledger) CurrentStorage() map[string]map[string]SignedData {
	l.Lock()
	defer l.Unlock()
	return copyStorage(l.blockchain.Last().Storage)
}

// LastBlock returns the last block in the blockchain
func (l *Ledger) LastBlock() Block {
	l.Lock()
	defer l.Unlock()
	return l.blockchain.Last()
}

func copyStorage(s map[string]map[string]SignedData) map[string]map[string]SignedData {
	out := map[string]map[string]SignedData{}
	for b, kv := range s {
		nb := make(map[string]SignedData, len(kv))
		for k, v := range kv {
			nb[k] = v
		}
		out[b] = nb
	}
	return out
}

// sameEntry reports whether two entries are byte-identical (the common case for
// re-broadcast/relay), so the merge can skip them without churn or warnings.
func sameEntry(a, b SignedData) bool {
	return a.Owner == b.Owner &&
		a.Version == b.Version &&
		a.UpdatedAt == b.UpdatedAt &&
		a.Deleted == b.Deleted &&
		a.Value == b.Value &&
		bytes.Equal(a.Sig, b.Sig)
}

// versionAfter returns the next version for a write. It is monotonic per key
// (prev+1) but also floored to the wall clock, so an owner that restarts with a
// fresh ledger (version counter reset) still out-versions any stale entry or
// tombstone other peers retained, rather than being rejected as a rollback.
func versionAfter(prev uint64, now time.Time) uint64 {
	v := prev + 1
	if n := uint64(now.UnixNano()); n > v {
		return n
	}
	return v
}

// projectValues exposes a bucket's live (non-tombstoned) values.
func projectValues(kv map[string]SignedData) map[string]Data {
	out := map[string]Data{}
	for k, v := range kv {
		if v.Deleted {
			continue
		}
		out[k] = v.Value
	}
	return out
}

// makeEntry builds the SignedData to store for a write. Without a signer it
// produces a legacy unsigned/unversioned entry (so the wire format is unchanged
// for default networks). With a signer it bumps the version and re-signs only
// when the value actually changes (keeping re-announce idempotent).
func (l *Ledger) makeEntry(bucket, key string, value Data, prev SignedData, now time.Time) SignedData {
	if l.signer == nil {
		return SignedData{Value: value}
	}
	if prev.Owner == l.signer.ID() && !prev.Deleted && prev.Value == value && prev.Sig != nil {
		return prev
	}
	d := SignedData{Value: value, Version: versionAfter(prev.Version, now), UpdatedAt: now.Unix(), Owner: l.signer.ID()}
	d.Sig, _ = l.signer.Sign(canonical(bucket, key, d))
	return d
}

func (l *Ledger) makeTombstone(bucket, key string, prev SignedData, now time.Time) SignedData {
	d := SignedData{Owner: l.signer.ID(), Version: versionAfter(prev.Version, now), UpdatedAt: now.Unix(), Deleted: true}
	d.Sig, _ = l.signer.Sign(canonical(bucket, key, d))
	return d
}

// Add data to the blockchain
func (l *Ledger) Add(b string, s map[string]interface{}) {
	l.commit(true, func(cur map[string]map[string]SignedData) bool {
		now := l.clock()
		if cur[b] == nil {
			cur[b] = make(map[string]SignedData)
		}
		changed := false
		for key, val := range s {
			dat, _ := json.Marshal(val)
			ne := l.makeEntry(b, key, Data(string(dat)), cur[b][key], now)
			if prev, ok := cur[b][key]; !ok || !sameEntry(prev, ne) {
				cur[b][key] = ne
				changed = true
			}
		}
		return changed
	})
}

// Delete data from the ledger. With a signer this writes a signed tombstone so
// the deletion survives gossip reconciliation; without one it removes the key
// (legacy behaviour).
func (l *Ledger) Delete(b string, k string) {
	l.commit(true, func(cur map[string]map[string]SignedData) bool {
		now := l.clock()
		if l.signer != nil {
			if bkt, ok := cur[b]; ok {
				if prev, ok := bkt[k]; ok && !prev.Deleted {
					cur[b][k] = l.makeTombstone(b, k, prev, now)
					return true
				}
			}
			return false
		}
		if _, ok := cur[b][k]; ok {
			delete(cur[b], k)
			return true
		}
		return false
	})
}

// DeleteBucket deletes a bucket from the ledger.
func (l *Ledger) DeleteBucket(b string) {
	l.commit(true, func(cur map[string]map[string]SignedData) bool {
		now := l.clock()
		if l.signer != nil {
			changed := false
			for k, prev := range cur[b] {
				if !prev.Deleted {
					cur[b][k] = l.makeTombstone(b, k, prev, now)
					changed = true
				}
			}
			return changed
		}
		if _, ok := cur[b]; ok {
			delete(cur, b)
			return true
		}
		return false
	})
}

// String returns the blockchain as string
func (l *Ledger) String() string {
	bytes, _ := json.MarshalIndent(l.blockchain, "", "  ")
	return string(bytes)
}

// Index returns last known blockchain index
func (l *Ledger) Index() int {
	return l.blockchain.Len()
}

// commit runs mutate under the lock against a copy of the current storage and,
// iff mutate reports a change, installs the result as a new block. The whole
// read-modify-write is atomic (holding the lock across read, mutate and append),
// so concurrent writers — local Add, the network merge and the reaper — cannot
// clobber each other. The broadcast happens after the lock is released (writing
// to the message channel under the lock could deadlock against the hub
// consumer, which drives ledger.Update).
func (l *Ledger) commit(broadcast bool, mutate func(cur map[string]map[string]SignedData) bool) {
	l.Lock()
	cur := copyStorage(l.blockchain.Last().Storage)
	changed := mutate(cur)
	if changed {
		newBlock := l.blockchain.Last().NewBlock(cur)
		if newBlock.IsValid(l.blockchain.Last()) {
			l.blockchain.Add(newBlock)
		}
	}
	var payload []byte
	if broadcast && changed {
		b, err := json.Marshal(l.blockchain.Last())
		if err != nil {
			log.Println(err)
		} else {
			payload = compress(b).Bytes()
		}
	}
	l.Unlock()

	if payload != nil {
		l.channel.Write(payload)
	}
}
