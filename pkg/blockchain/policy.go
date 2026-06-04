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
	"time"

	"github.com/mudler/edgevpn/pkg/protocol"
)

// ExpiryKind selects how a bucket's entries are aged out. See
// docs/design/authenticated-ledger.md.
type ExpiryKind int

const (
	// NoExpiry: entries live forever (default for unregistered/legacy buckets).
	NoExpiry ExpiryKind = iota
	// Liveness: an entry is alive iff its owner's heartbeat is fresh.
	Liveness
	// Absolute: an entry expires TTL after its own signed UpdatedAt.
	Absolute
)

// BucketPolicy is the single source of truth for whether a bucket is
// authenticated and how its entries expire. Adding a new authenticated bucket
// is a one-line registry entry.
type BucketPolicy struct {
	Owned       bool                            // signed + owner-enforced?
	OwnerOf     func(key string, v Data) string // who owns this entry
	Expiry      ExpiryKind
	TTL         time.Duration // only meaningful for Absolute
	Reclaimable bool          // may a non-owner claim the key after expiry?
}

// Registry maps bucket name -> policy. The zero BucketPolicy (returned for any
// unregistered bucket) is unowned/open and never expires, preserving legacy
// behaviour for buckets we have not opted in.
type Registry map[string]BucketPolicy

// Policy returns the policy for a bucket, or the open/legacy zero value.
func (r Registry) Policy(bucket string) BucketPolicy { return r[bucket] }

// ownerIsKey is the OwnerOf for buckets where the key itself is the owner
// peer.ID (users, healthcheck).
func ownerIsKey(key string, _ Data) string { return key }

// ownerFromPeerIDField is the OwnerOf for buckets whose value carries a PeerID
// field (machines, services, files, dns).
func ownerFromPeerIDField(_ string, v Data) string {
	var s struct{ PeerID string }
	_ = v.Unmarshal(&s)
	return s.PeerID
}

// DefaultRegistry is the built-in policy set. ttl is the liveness window
// (reused as the Absolute TTL for the heartbeat bucket itself).
func DefaultRegistry(ttl time.Duration) Registry {
	return Registry{
		protocol.MachinesLedgerKey: {Owned: true, OwnerOf: ownerFromPeerIDField, Expiry: Liveness, Reclaimable: true},
		protocol.ServicesLedgerKey: {Owned: true, OwnerOf: ownerFromPeerIDField, Expiry: Liveness, Reclaimable: true},
		protocol.FilesLedgerKey:    {Owned: true, OwnerOf: ownerFromPeerIDField, Expiry: Liveness, Reclaimable: true},
		protocol.UsersLedgerKey:    {Owned: true, OwnerOf: ownerIsKey, Expiry: Liveness},
		protocol.HealthCheckKey:    {Owned: true, OwnerOf: ownerIsKey, Expiry: Absolute, TTL: ttl},
		// dns is self-owned: the value (types.DNS) carries no owner field, so a
		// nil OwnerOf means the first signer to claim a name owns it (first-claim
		// + lease). This blocks hijacking an existing name; constraining which
		// names/targets a peer may register (e.g. rejecting ".*" catch-alls) is
		// further hardening tracked separately.
		protocol.DNSKey: {Owned: true, OwnerOf: nil, Expiry: Liveness, Reclaimable: true},
	}
}

// IsLive reports whether owner has a heartbeat in healthData (a map of
// peer.ID -> RFC3339 timestamp, as written by the alive service) that is newer
// than ttl relative to now.
func IsLive(healthData map[string]Data, owner string, ttl time.Duration, now time.Time) bool {
	t, ok := healthData[owner]
	if !ok {
		return false
	}
	var s string
	if err := t.Unmarshal(&s); err != nil {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	return parsed.Add(ttl).After(now)
}
