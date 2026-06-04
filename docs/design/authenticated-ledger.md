# Authenticated, per-owner ledger entries

Status: **proposed** · Target: single PR · The `edgevpn` binary defaults to `--ownership=enforce`; operators opt out with `--ownership=off` (`EDGEVPNOWNERSHIP`). The library default (`node.New` without the option) stays off so embedders opt in deliberately.

## 1. Problem

EdgeVPN's only trust boundary today is *knowledge of the network token*. Every peer
that holds the token is fully, equally and permanently trusted. The shared ledger
(`pkg/blockchain`) is the control plane for the whole network — it stores IP→peer
mappings, services, files and DNS records — yet:

- Blocks are **unsigned**. `Block.Checksum()` hashes only `Index|Timestamp|Storage|PrevHash`;
  there is no notion of *who* authored an entry.
- `Ledger.Update` adopts any incoming block whose `Index` is higher (hash breaks ties)
  and **whole-replaces** local state. There is no per-key authorisation.

Consequences (see the security audit): any in-network peer can overwrite *any* key —
hijack another node's IP (MITM), poison DNS (`.*` record), hijack services/files, or
take over global state. Every "ownership" check in `pkg/vpn`/`pkg/services` reads its
authority from the same ledger the attacker rewrites, so those checks are self-referential.

A second, operational problem: entries owned by a node that has gone away linger
forever (the ledger is "pestered by old entries"), because deletion is best-effort and
not tied to liveness.

## 2. Goals

1. A write to a peer-owned key is only accepted if it is **signed by the owning peer**.
2. An entry cannot be **rolled back or replayed** to an older value.
3. Entries owned by an **inactive** node expire and are **reaped**, bounding ledger size.
4. **Extensible**: adding a new bucket with ownership + TTL is a one-line registry entry.
5. **Backwards compatible by default**: ships behind a flag; existing networks keep working.

Non-goals: changing the libp2p transport, the seal-key/OTP scheme, or per-peer message
encryption. This work is purely about *authorising writes to the ledger* and *bounding
its lifetime*.

## 3. Core idea

The ledger already behaves as a single shared snapshot (`MemoryStore` keeps only
`Last()`; the chain is never validated). We lean into that:

> **State is a map of independently *signed*, *versioned* entries. A "block" is just the
> transport envelope carrying a snapshot of them.** Security moves from the block (height)
> to the entry (owner signature + monotonic version).

Forging another peer's entry then requires that peer's private key. Convergence becomes
per-entry (highest version wins, deterministic tie-break) instead of a global height race,
which also removes the split-brain hack the current `Update` comment describes.

## 4. Data model

```go
// pkg/blockchain/data.go
type SignedData struct {
    Value     Data   // existing JSON payload — unchanged, readers still Unmarshal this
    Owner     string // peer.ID (base58) of the author
    Version   uint64 // monotonic per (bucket,key); only bumps when Value changes
    UpdatedAt int64  // unix seconds — signed; drives Absolute TTL & lease renewal
    Deleted   bool   // signed tombstone
    Sig       []byte // Ed25519 over canonical(bucket,key,Owner,Version,UpdatedAt,Deleted,Value)
}
```

`Block.Storage` becomes `map[string]map[string]SignedData`. The inner `Data` type is kept,
so every reader in `pkg/vpn`/`pkg/services` keeps calling `entry.Value.Unmarshal(&x)`.

`Block.Checksum()` stays for transport integrity only; it is **no longer** the security
boundary.

### Canonical encoding

Signing input is a length-prefixed concatenation of the fields (never Go `fmt`/map
ordering) so the signed bytes are identical on every node:

```
canonical = len|bucket ‖ len|key ‖ len|owner ‖ u64|version ‖ i64|updatedAt ‖ b|deleted ‖ len|value
```

## 5. Signing & verification (`pkg/blockchain/sign.go`)

- **Key = the libp2p host identity key** (Ed25519, already generated in `genHost`). No new
  keys, no PKI.
- **Verification needs no key distribution**: `peer.Decode(Owner).ExtractPublicKey()`
  recovers the public key directly from the peer ID (works because EdgeVPN forces Ed25519).
- A `Signer` is injected into the ledger:

```go
type Signer interface {
    Sign(msg []byte) ([]byte, error)
    ID() string // base58 peer.ID of the signer
}
```

In tests this is a fake key; in production it is built from the host key after `genHost`.

## 6. Ownership & TTL registry (`pkg/blockchain/policy.go`)

The single source of truth for "what is signed and what is under TTL". Adding a feature
bucket = adding one row. The merge engine, the reaper and the readers all consult it.

```go
type ExpiryKind int
const (
    NoExpiry ExpiryKind = iota // never expires (default for unregistered buckets)
    Liveness                   // alive iff the OWNER's heartbeat is fresh
    Absolute                   // alive iff now-UpdatedAt < TTL
)

type BucketPolicy struct {
    Owned       bool                            // signed + owner-enforced?
    OwnerOf     func(key string, v Data) string // who owns this entry
    Expiry      ExpiryKind
    TTL         time.Duration                   // only for Absolute
    Reclaimable bool                            // may a non-owner claim after expiry?
}

type Registry map[string]BucketPolicy
```

Default registry:

| Bucket          | Owned | OwnerOf            | Expiry          | Reclaimable |
|-----------------|-------|--------------------|-----------------|-------------|
| `machines`      | yes   | `PeerID` field     | Liveness        | yes         |
| `services`      | yes   | `PeerID` field     | Liveness        | yes         |
| `files`         | yes   | `PeerID` field     | Liveness        | yes         |
| `users`         | yes   | key == peer.ID     | Liveness        | no          |
| `healthcheck`   | yes   | key == peer.ID     | Absolute(maxTime) | no        |
| `dns`           | yes   | self-owned (nil OwnerOf) | Liveness  | yes (first-claim) |
| *(unregistered)*| no    | —                  | NoExpiry        | —           |

A `nil` `OwnerOf` marks a **self-owned** bucket: the value carries no owner field,
so the first signer to claim a key owns it (first-claim), and the normal
cross-owner/expiry rules then protect it. `dns` uses this so it needs no change to
the `types.DNS` record shape.

An unregistered bucket is the zero value: open + never expires → unchanged behaviour, which
is what keeps the change backwards compatible for any bucket we have not opted in.

### Two expiry kinds

- **`Liveness`** — the entry is alive iff its owner's heartbeat (`HealthCheckKey`) is fresh
  (`owner ∈ AvailableNodes(maxTime)`). One heartbeat governs *all* of a node's entries: when
  a node goes inactive, its IP, services, files and DNS names expire together. This is the
  answer to "a node becoming inactive" — we never track entries individually.
- **`Absolute(d)`** — expires `d` after the entry's own signed `UpdatedAt`, independent of
  owner liveness (used for the heartbeat bucket itself and any future ephemeral key).

Lease renewal is free: the existing `Persist`/`AnnounceUpdate` loops already re-announce
each entry periodically; under the new model that re-sign bumps `UpdatedAt`.

## 7. The merge — `Ledger.Update` rewrite

`Update` stops doing height-wins-whole-replace and merges entry by entry into current state:

```
for each (bucket, key, incoming) in block.Storage:
    pol = registry[bucket]
    if !pol.Owned:                      accept (legacy/open bucket); continue
    if !verifySig(bucket, key, incoming):                       drop   // forged / corrupt
    if pol.OwnerOf(key, incoming.Value) != incoming.Owner:      drop   // e.g. Owner != Machine.PeerID
    cur = state[bucket][key]
    if cur exists:
        if incoming.Owner != cur.Owner && !expired(pol, cur):   drop   // hijack attempt
        if incoming.Version <  cur.Version:                     drop   // rollback / replay
        if incoming.Version == cur.Version && incoming.Sig != cur.Sig:
            keep deterministic winner (Owner, then Sig bytes)          // anti-flap
    accept incoming
```

- **Per-key versions** make rollback/replay impossible without out-versioning the owner —
  which only the owner can sign.
- **Cross-owner writes are allowed iff the current owner's lease has expired.** This single
  rule powers both *reclaim* (peer B takes a dead peer's IP) and *reaping* (the leader
  tombstones a dead peer's entry).
- **Modes**: `off` skips this merge entirely (legacy height-wins path, no signing). `observe`
  runs the merge but turns every `drop` above into a `WARN` log + accept (observe violations on
  a live network without breaking it). `enforce` (the binary default) drops invalid writes.

## 8. Write path

`Add(bucket, {key: value})`:
1. read current entry; if `Value` unchanged → keep `Version` (idempotent under the
   `AnnounceUpdate`/`Persist` re-announce loops), else `Version = cur.Version + 1`;
2. set `Owner = signer.ID()`, `UpdatedAt = now`, `Deleted = false`;
3. sign canonical bytes → `Sig`; store.

`Delete`/`DeleteBucket`/`AnnounceDeleteBucket*` emit **signed tombstones** (`Deleted=true`,
`Version` bumped, empty `Value`) instead of dropping keys, so a delete propagates and an old
value cannot be replayed over it.

## 9. Reaping inactive nodes

Two layers, both built on existing primitives (`HealthCheckKey`, `AvailableNodes`,
`utils.Leader`).

1. **Reader-side lazy expiry (free, immediate).** Every reader (`vpn` routing, `services`
   dial, `dns` resolve) wraps lookups with `policy.IsLive(owner)`. The instant a node's
   heartbeat goes stale, traffic stops routing to it and its services/DNS stop resolving —
   *functionally* clean before anything is deleted.

2. **Leader-elected reaper (bounded storage).** This generalises the alive service's existing
   leader-scrub. The elected leader (`utils.Leader(AvailableNodes(...))`) periodically walks
   the registry and, for each `Liveness` bucket, writes signed tombstones for entries whose
   owner left the live set. The merge accepts these cross-owner tombstones precisely because
   the owner's lease is expired (same rule as reclaim). Only the leader writes ⇒ no tombstone
   storm; if the leader dies, the next election picks a live one.

3. **Tombstone GC.** Tombstones carry an `Absolute` TTL (e.g. 2–3× `maxTime`, wider than any
   expected partition); past that the leader physically drops the key. A node partitioned
   longer than that may briefly re-inject an old entry — but its heartbeat is stale too, so
   readers ignore it and the reaper re-tombstones it next cycle. Self-healing; storage stays
   bounded.

This replaces the current blunt `b.DeleteBucket(HealthCheckKey)` scrub in `alive.go` with
targeted, registry-driven reaping across all TTL buckets.

> Implementation note: layers (2) and (3) are implemented (`Ledger.Reap`, wired into the
> alive service's leader path), and tombstoned entries already read as absent via
> `GetKey`/`CurrentData`. Layer (1) is implemented for VPN routing via
> `Ledger.IsOwnerLive` (a no-op when ownership is off); applying the same guard at the
> service/file dial sites is a small further addition.

## 10. Wiring

The ledger is currently created lazily (`node.Ledger()`) before `genHost`. We build the
`Signer` from the host key right after `genHost` and inject it (`ledger.SetSigner(...)`)
before any network service runs. The host private key is read from
`host.Peerstore().PrivKey(host.ID())`, covering both supplied and auto-generated keys.

## 11. Configuration

| Flag / field             | Default   | Meaning                                               |
|--------------------------|-----------|-------------------------------------------------------|
| `--ownership` (`EDGEVPNOWNERSHIP`) | `enforce` | `enforce` = sign + reject; `observe` = sign + log-only; `off` = legacy/opt-out |
| `--ownership-ttl`        | node default (2m) | liveness window after which an inactive owner's entries may be reclaimed/reaped |

Reaping cadence reuses the existing alive `scrub-interval`, and tombstones are retained for
`3×maxtime` before pruning (see `pkg/services/alive.go`).

## 11a. Identity, the token, and restarts

Ownership binds every entry to a peer's **libp2p identity** — which is *not* the network token.
The two are independent secrets:

- The **token** carries only the OTP material (DHT key, crypto exchange key, room/rendezvous).
  It is shared by every node and contains no private key. It is *admission* control.
- The **identity** is a per-node Ed25519 keypair (`genHost`, `pkg/node/connection.go`). It is the
  thing ownership is bound to. A libp2p Ed25519 peer ID **embeds its public key**, which is what
  lets `Verify` recover the verifying key from the `Owner` peer.ID with no PKI.

By default the identity is **ephemeral**: with no configured key, `GenPrivKey(0)` draws from
`crypto/rand`, so the peer ID is fresh on every start. Persistence is via `--privkey-cache`
(binary) or `node.WithPrivKey` (library).

**Why this matters under enforcement.** Because entries are owned by the identity:

- *Stable identity* → a restart is the **same owner**; the node updates its own entries
  immediately (same-owner write, version kept monotonic by the wall-clock floor in
  `versionAfter`). No orphans, no reclaim delay.
- *Ephemeral identity* → a restart is a **new owner**; the node's previous entries are orphaned,
  reaped after the liveness TTL, and the node re-establishes its footprint under the new identity
  via cross-owner reclaim (allowed only once the old owner expires). This works but adds churn and
  a reclaim delay of up to the TTL on every restart.

For long-lived nodes a stable identity is therefore recommended under enforcement
(`--privkey-cache`). The binary does **not** auto-enable it: the default cache dir is shared
per-user (`$HOME/.edgevpn`), so auto-persisting would make co-located processes (e.g. `edgevpn
api` next to the VPN, or `file-send`/`file-receive`) load the *same* key and boot with a
duplicate peer ID. Instead, `cmd/util.go` emits a one-line warning when ownership is on with an
ephemeral identity, pointing operators at `--privkey-cache` (with a distinct `--privkey-cache-dir`
per process). Ephemeral identities still work — restarts just churn (orphan → reclaim after the
liveness TTL).

**Security boundary.** Token = admission (keeps outsiders out); identity = per-write integrity
(keeps insiders from impersonating each other — a leaked token still cannot let peer A write peer
B's entries, since that needs B's private key, which is never shared and never in the token).
The flip side: identities are free and unlimited, so ownership gives **write-integrity, not
anti-sybil**. A token-holding attacker can still spin up many identities and *first-claim* free
IPs / service names / DNS names (ownership stops hijacking *existing* entries, not squatting
*unclaimed* ones). Anti-sybil needs an identity cost/allowlist — which is what the
`PeerGuard`/trustzone ECDSA mechanism is for.

## 12. Backwards compatibility

The signed wire format differs from the legacy bare-value one, so observe/enforce nodes do not
interoperate with pre-authentication nodes on the owned buckets. Because the binary now
defaults to `enforce`, **a network must be upgraded together** (all nodes on the same version
and mode). Notes:

- The library default (`node.New` without `WithOwnership`) stays `off`, emitting the exact
  legacy bare-value encoding, so embedders (e.g. LocalAI, Kairos) are unaffected until they
  opt in — and they must ensure the alive service is running, since cross-owner protection
  relies on heartbeats.
- `observe` is the safe rollout step: it signs and logs violations without dropping, so a
  network can move legacy → observe → enforce while watching the warnings.
- Operators opt out with `--ownership=off` (`EDGEVPNOWNERSHIP=off`).

## 13. Testing

- `sign_test.go`: sign/verify round-trip; pubkey-from-peerID extraction; any tampered field → reject.
- `policy_test.go`: registry lookups; `IsLive` against `HealthCheckKey`; `expired` for both kinds.
- `ledger_merge_test.go`:
  - legit owner update accepted;
  - **cross-owner overwrite of a live entry rejected** (the IP-hijack test);
  - rollback (lower version) rejected;
  - tombstone not resurrected by replay of the old value;
  - concurrent first-claim of one IP converges deterministically on all nodes;
  - reclaim after lease expiry accepted;
  - log-only mode accepts-but-warns; enforce mode drops.
- `reaper_test.go`: dead-owner entries get tombstoned by the leader only; tombstones pruned
  after `tombstone-ttl`.
- `go test -race ./pkg/blockchain/... ./pkg/node/... ./pkg/services/...`.

## 14. Files touched

`pkg/blockchain/{data,block,ledger,sign(new),policy(new),store_disk,store_memory}.go`,
`pkg/node/{node,options}.go`, `pkg/config/config.go`, `pkg/services/alive.go` (reaper),
`pkg/vpn/vpn.go` + `pkg/services/{services,files,dns}.go` (one-line `IsLive` guards on reads),
`pkg/protocol/protocol.go` (version bump), plus tests.

## 14a. Post-review hardening

A review of the first cut found and fixed several issues:

- **No re-broadcast churn / false warnings.** The merge now skips byte-identical
  incoming entries (`sameEntry`), so an idle network neither mints a block every
  sync nor logs spurious ownership-violation warnings; open buckets require a
  *strictly* higher version to adopt.
- **Restart-safe versioning.** Versions are floored to the wall clock
  (`versionAfter`), and an existing tombstone is treated as a reclaimable slot,
  so an owner that restarts (or returns after being reaped) is not locked out of
  its own key by a higher-versioned tombstone until GC.
- **Atomic writes.** `commit` holds the lock across the whole read-modify-write
  (read → mutate copy → append block), so the concurrent writers — local `Add`,
  the network merge, and the reaper — can no longer clobber one another (a lost
  heartbeat could otherwise trigger false reaping). The network merge installs
  without re-broadcasting (the Syncronizer propagates state) to avoid gossip
  amplification.
- **More gated reads.** `IsOwnerLive` guards VPN routing (high-frequency, where
  acting on a just-departed owner matters and reaper lag is visible). It is
  deliberately *not* applied to the one-shot service/file dials: those are
  short-lived transfers where the peer has just announced itself and is actively
  serving, so gating on heartbeat propagation only adds latency/fragility — the
  reaper still removes a dead owner's entry within a scrub interval. The `egress`
  bucket is registered (self-owned) so egress
  advertisements can't be forged for other peers. The `dhcp` leader bucket is
  deliberately left open (its shared key changes owner on handoff; the
  deterministic `utils.Leader` cross-check already bounds a forged value to a
  self-correcting stall, and the assigned IPs in `machines` are signed).

## 15. Known limitations / follow-ups

- **DNS ownership.** Implemented as a self-owned bucket (first-claim + lease), so
  hijacking an *existing* DNS name owned by a live peer is blocked and dead owners'
  records are reaped. Still open as further hardening: constraining *which* names a
  peer may register (e.g. rejecting `.*` catch-alls) and binding a record's targets to
  the owner's own address.
- **Reader-side `IsLive` guards.** Implemented for VPN routing (`Ledger.IsOwnerLive`),
  in addition to reaping + tombstone hiding. The same guard at the service/file dial
  sites is a small further addition.
- **Wire format.** Observe/enforce modes change the on-wire entry encoding to the signed
  object form; a network running observe/enforce must have all nodes on this version.
  The default (`off`) keeps the exact legacy bare-value encoding, so mixed old/new nodes
  interoperate only while enforcement is off.
- **`--enforce-ownership` UX.** Exposed as `--ownership=off|observe|enforce` with
  `--ownership-ttl`; the recommended rollout is `observe` (logs violations) before
  `enforce`.
