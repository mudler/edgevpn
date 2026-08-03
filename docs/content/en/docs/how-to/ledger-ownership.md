---
title: "Ledger ownership"
linkTitle: "Ledger ownership"
weight: 80
description: >
  Sign and authorise ledger writes — and change the mode on a live network without splitting it.
---

{{% pageinfo color="warning"%}}
`--ownership` is the one EdgeVPN setting that **every node on a network must
agree on**. Mixing `off` with `enforce` produces a network that looks up but
silently drops half its ledger. If you are upgrading an existing network, read
[Changing the mode on a live network](#changing-the-mode-on-a-live-network)
first.
{{% /pageinfo %}}

Every peer that holds the network token can write to the
[ledger](../../explanation/the-ledger/). Without ownership, it can write to
*anyone's* entries: overwrite the `machines` record that maps an IP to a peer,
claim someone else's DNS name, or replace a service announcement. Ownership
closes that gap by binding each entry to the libp2p identity that wrote it.

Under ownership each ledger entry carries the author's peer ID, a monotonic
version, a timestamp and an Ed25519 signature. Peers verify the signature using
the public key embedded in the author's peer ID — there is no PKI and no key to
distribute. A write to a key owned by a live peer is only accepted if it is
signed by that peer.

This is **write integrity, not admission control**. The token is still the only
thing keeping outsiders out, and identities are free, so a token holder can
still create many identities and claim *unclaimed* names or addresses.
Ownership stops it from taking over entries that already belong to someone
else. See [the security model](../../explanation/security-model/) for the whole
picture, and [the authenticated ledger](../../explanation/authenticated-ledger/)
for the design.

## The three modes

```bash
sudo edgevpn --ownership enforce --token "$TOKEN"    # the default
```

| Mode | Signs its own writes | Incoming unauthorised write | Use it for |
|---|---|---|---|
| `enforce` | yes | **rejected** and logged | normal operation (the default) |
| `observe` | yes | accepted, logged | migrating a live network |
| `off` | no | accepted (legacy whole-block replace) | staying compatible with pre-ownership nodes |

| Flag | Default | Environment | Description |
|---|---|---|---|
| `--ownership` | `enforce` | `EDGEVPNOWNERSHIP` | `enforce`, `observe` or `off` |
| `--ownership-ttl` | `0` (derived: 8 minutes) | `EDGEVPNOWNERSHIPTTL` | Liveness window in seconds, after which an inactive owner's entries may be reclaimed or reaped. `0` derives it from the heartbeat interval |

Values are matched case-insensitively, and each mode has aliases: `enforce`
and `on`; `observe`, `log` and `log-only`; `off`. Anything else is rejected at
startup, before the node joins a network:

```
invalid ownership mode "enabled": must be one of "off" (legacy, no
authentication), "observe" (sign and log violations; aliases "log", "log-only")
or "enforce" (sign and reject unauthorized writes; alias "on")
```

{{% alert title="Older releases disabled ownership on a typo" color="warning" %}}
Before this validation existed, an unrecognised value — `true`, `enabled`, or a
misspelling — fell through to `off` with no error and no warning, silently
turning ledger authentication off on a node the operator believed was enforcing
it. If you are on an older build, confirm the mode in the startup log rather
than trusting the flag.
{{% /alert %}}

When ownership is active the node logs one line at startup:

```
ledger ownership enforcement: mode=enforce ttl=8m0s
```

If that line is absent, the node is running unsigned. Usually that means `off`,
but it is also what you see when enforcement was requested and the host private
key turned out to be unavailable — that case logs `ownership enforcement
requested but host private key is unavailable; running unsigned` instead, so
check for it before concluding the flag did not take.

### Which entries are owned

Ownership is per-bucket. These buckets are signed and owner-enforced:

| Bucket | Owner | Expiry |
|---|---|---|
| `machines` | the `PeerID` in the value | while the owner's heartbeat is fresh |
| `services` | the `PeerID` in the value | while the owner's heartbeat is fresh |
| `files` | the `PeerID` in the value | while the owner's heartbeat is fresh |
| `users` | the key (a peer ID) | while the owner's heartbeat is fresh |
| `egress` | the key (a peer ID) | while the owner's heartbeat is fresh |
| `dns` | the first peer to claim the name | while the owner's heartbeat is fresh |
| `healthcheck` | the key (a peer ID) | `--ownership-ttl` after the entry's own timestamp |

`Reclaimable` appears on the policy struct but the merge does not consult it, so
every bucket above becomes claimable once its owner's lease has lapsed.

The `dhcp` bucket (IP-lease leader election) is deliberately left open: its
single `leader` key changes owner on every handoff, and readers already
cross-check it against the deterministic leader election. Any bucket not listed
here — including buckets your own application writes through the API — takes the
zero policy: no owner, no expiry, writable by any peer. Its entries are still
signed like every other write, but nothing verifies them; the only constraint
the merge applies is the version check, which accepts an incoming entry when its
version is strictly higher than the stored one.

Ownership decides liveness from heartbeats, so the **alive service must be
running**. The VPN (`edgevpn`), `api`, `service-add`/`service-connect` and
`file-send`/`file-receive` all start it. Two commands do not:

- `edgevpn start` only joins the network as a relay. It publishes no heartbeat,
  and it claims nothing in an owned bucket either, so the omission costs it
  nothing.
- `edgevpn proxy` publishes no heartbeat but *does* claim an owned entry. See
  the known issue below.

Embedders using the library must run the alive service themselves.

{{% alert title="Known issue: `edgevpn proxy` claims an entry it cannot keep alive" color="warning" %}}
`services.Proxy` bundles the proxy service alone, with no alive service — yet
the proxy announces its own peer ID into `users`, a bucket that is owned with a
liveness expiry. With no heartbeat behind it, every other node reads that entry
as belonging to an inactive owner from the moment it appears, and the elected
leader tombstones it on each scrub. The proxy's next announce re-adds it, so the
entry churns for as long as the node runs.

Nothing breaks outright — `users` gates *dialing* a service, file or egress, and
the proxy is the side doing the dialing — but the ledger carries an entry that is
permanently expired and permanently rewritten, and the tombstone traffic is real.
Until this is fixed, run `edgevpn proxy` on a host that also runs a command
which starts the alive service if the churn matters to you. This is a behaviour
question rather than a documentation one and is tracked separately.
{{% /alert %}}

## Changing the mode on a live network

Modes are not freely mixable. What one node accepts from another depends on
both nodes' modes:

| Writer → Reader | `off` | `observe` | `enforce` |
|---|---|---|---|
| **`off`** | works | works (accepted, logged) | **silently dropped** |
| **`observe`** | works | works | works |
| **`enforce`** | works | works | works |

An `off` node writes entries in the legacy bare-value format with no signature.
An `enforce` node rejects every one of them. Nothing errors, nothing exits — the
writes just never land.

`observe` is compatible with **both** neighbours: it signs its own writes (so
`enforce` nodes accept them) and it accepts unsigned writes (so `off` nodes
still get through). That is what makes it the bridge.

### The safe sequence

1. **Move the whole network to `observe`.** Restart every node with
   `--ownership observe`. This is safe to do one node at a time: an `observe`
   node interoperates with the `off` nodes that have not been restarted yet.
2. **Let it run and watch the logs.** Expect a burst of
   `ownership violation (observe, accepting): …` lines while `off` nodes are
   still writing unsigned entries. Once every node is on `observe`, those lines
   must stop.
3. **Only when the violation lines have stopped, move to `enforce`.** Restart
   every node with `--ownership enforce`. Also safe one node at a time, because
   `observe` and `enforce` nodes both sign.

Going the other way (`enforce` → `off`) has the same hazard in reverse and wants
the same route: `enforce` → `observe` everywhere, then `off` everywhere.

Two things make step 2 a stage to pass through rather than to camp in.

- While legacy nodes remain, an `observe` node that is elected leader will
  tombstone their heartbeats on every scrub. A legacy heartbeat carries no
  signed timestamp, so it reads as infinitely old and is always past its
  absolute expiry. The legacy node re-announces and reappears, but it flickers
  in and out of the live set — and while it is out, its address, services and
  DNS names are eligible for reaping and reclaim.
- If you persist the ledger with `--ledger-state`, entries written under `off`
  are unsigned and survive the restart into `enforce`. (`observe` is not
  affected: it installs a signer and signs every write, exactly as `enforce`
  does — the difference is only whether violations are rejected or logged.)
  Each unsigned entry is re-signed by the peer it names on that peer's next
  announce, and until then the merge resolves the owner from the value, so
  nobody else can take it. The exception is `dns`, whose values carry no peer
  ID: with no owner to resolve, an unsigned DNS entry reads as unclaimed and
  any peer can take the name before its rightful owner re-announces. Nodes on
  the default in-memory ledger start clean and skip this entirely.

  {{% alert title="Older releases froze those entries" color="warning" %}}
  A persisted unsigned entry used to be unreplaceable even by its own rightful
  owner — the stored entry named no owner, the owner was still live, and the
  signed update was rejected as `overwrite of a live entry owned by another
  peer`, permanently. Deleting one behaved worse still: the delete took effect
  on the owner's own node and was rejected everywhere else
  (`tombstone by non-owner of a live entry`), so the network diverged with no
  error shown on the node that issued it. If you are on an older build, clear
  the state directory before restarting a node into `enforce`.
  {{% /alert %}}

### What it looks like when you get it wrong

Suppose one node is left on `off` while the rest are on `enforce`.

**On the `enforce` nodes**, every write from the `off` node is dropped and
logged at `warn` — visible at the default `--log-level info`:

```
ownership violation (rejected): machines/10.1.0.9 from : invalid signature
ownership violation (rejected): healthcheck/12D3KooWQCErhoGPk64ST3Cs6pz8kS1s7Eyd77SS7mLLaqaf8VYG from : invalid signature
```

The empty owner between `from` and `:` is the tell — a legacy entry has no owner
field at all. This is the signature of a mixed-mode network, as opposed to a
genuine ownership violation, which names the offending peer.

Because the `healthcheck` write is rejected too, the `off` node never joins the
live set on those nodes. It is not merely unroutable, it is invisible. Start an
`enforce` node with `--api` and look for it:

```bash
curl -s http://127.0.0.1:8080/api/ledger/machines     # the off node's IP is absent
curl -s http://127.0.0.1:8080/api/ledger/healthcheck  # the off node's peer ID is absent
```

Traffic to that address is dropped at the routing lookup, and any service, file
or DNS name it announces is never seen.

**On the `off` node**, the failure looks completely different — and much more
confusing, because it logs nothing at all. An `off` node still adopts whole
blocks from the `enforce` nodes, so it *can* see them and their entries decode
fine. But adopting a block **replaces its entire local state**, wiping its own
entries; its announce loop then re-adds them, and the next incoming block wipes
them again. The result is a node that shows a plausible peer list, cannot be
reached by anyone, and produces no error.

If you are diagnosing a network where "some nodes can't see each other", check
the ownership mode on every node before anything else.

## Identity and restarts

Ownership binds entries to a node's **libp2p identity**, which is not the
network token. By default that identity is **ephemeral**: it is regenerated from
`crypto/rand` on every start, so a restarted node comes back as a *different
owner*. Its old entries are orphaned, and it can only reclaim its address,
services and DNS names once the previous owner's lease expires — up to
`--ownership-ttl`.

With ownership on and no persisted key, the node warns once at startup:

```
ownership enforcement is on with an ephemeral identity: this node's ledger
entries will be reclaimed after the liveness TTL on each restart. Use
--privkey-cache with a per-node --privkey-cache-dir for a stable identity.
```

For a long-lived node, persist the identity:

```bash
sudo edgevpn --ownership enforce --privkey-cache \
        --privkey-cache-dir /var/lib/edgevpn/node1 --token "$TOKEN"
```

| Flag | Default | Environment | Description |
|---|---|---|---|
| `--privkey-cache` | off | `EDGEVPNPRIVKEYCACHE` | Persist the libp2p identity to disk and reuse it |
| `--privkey-cache-dir` | `$HOME/.edgevpn` | `EDGEVPNPRIVKEYCACHEDIR` | Where the key is stored (file `privkey`, mode `0600`) |

{{% alert title="Give every process its own cache directory" color="warning" %}}
The default cache directory is shared per user. Two EdgeVPN processes on the
same machine — say `edgevpn` for the VPN and `edgevpn api` beside it — would
load the *same* key and join with the *same* peer ID, which breaks the network
far more thoroughly than an ephemeral identity does. This is why
`--privkey-cache` is never enabled automatically. Set a distinct
`--privkey-cache-dir` per process.
{{% /alert %}}

Ephemeral identities are still fine for short-lived clients (`file-send`,
`file-receive`, `service-connect`) — they claim a slot, use it and leave.

## The liveness window and reaping

`--ownership-ttl` is the liveness window: a node is considered live for that
long after its last heartbeat. It drives three things.

- **Routing.** Packets are not routed to an address whose owner's heartbeat has
  gone stale. This takes effect immediately, without waiting for any cleanup.
- **Reclaim.** Once an owner's lease has expired, another peer may take over its
  keys. This is how an address is recycled after a node leaves for good.
- **Reaping.** The elected leader periodically walks the ledger and writes
  signed tombstones over entries whose owner is no longer live, then physically
  prunes tombstones once they are old enough for everyone to have seen them.
  This is what stops a long-running network from accumulating entries from
  nodes that never came back. It replaces the older, blunter behaviour of
  wiping the whole `healthcheck` bucket on a timer, and it runs only on the
  leader, so there is no tombstone storm.

A node that goes offline therefore disappears in stages: it stops being routed
within a TTL, its entries are tombstoned on the next leader scrub, and the
tombstones are pruned later. A node that comes back with the *same* identity
re-claims its own keys immediately, even over a tombstone.

### Choosing a value

Leave it at `0`. A node is declared dead purely on the age of its last
heartbeat, so the window has to cover the worst case gap between two heartbeats
— and `0` derives exactly that, as four times
`--aliveness-healthcheck-interval` (8 minutes on the stock 120-second
heartbeat). Retune the heartbeat and the window follows.

Four intervals rather than one because the heartbeat runs on a jittered ticker:
each tick lands anywhere between 0.5× and 1.5× the configured interval, so a
perfectly healthy node can go 180 seconds between heartbeats on a 120-second
setting. Surviving one lost heartbeat therefore means covering 360 seconds of
silence — and the window has to be strictly longer than that rather than equal
to it, because a node whose last heartbeat is exactly one window old is already
expired. Four nominal intervals clears it with a full interval to spare.

{{% alert title="Releases before this derived the window used 2 minutes" color="warning" %}}
That was exactly the nominal heartbeat interval and therefore *shorter* than the
180 seconds a healthy node can actually take. Live nodes were periodically
treated as inactive: packets to them dropped, their addresses became claimable
by other peers, and a leader scrub landing in the gap tombstoned their entries.
It self-corrected on the next announce, so it showed up as unexplained blips
rather than an outage. If you are on an older build, set `--ownership-ttl 480`
explicitly.
{{% /alert %}}

Set an explicit value only if you have a reason to, and then set it on **every**
node. Unlike `--ownership`, the TTL is a local judgement about when a peer is
dead rather than a wire format, so nodes that disagree can still read each
other — but they will not agree on what the ledger says. The merge is per-key
and nothing reconciles whole blocks afterwards, so a node with a shorter window
declares an owner dead, stops routing to it and lets its entries be reclaimed or
reaped, while a node with a longer window still routes to that owner. The two
stay split until the owner re-announces. A longer window also means dead nodes
linger; a shorter one risks evicting live ones, and anything at or below three
heartbeat intervals will.

## Turning it off

```bash
sudo edgevpn --ownership off --token "$TOKEN"
```

You want this only to interoperate with nodes that predate ownership, and only
after moving the *entire* network through `observe` as described above. With
ownership off, any token holder can overwrite any entry.

Embedders are unaffected by default: `node.New` without `WithOwnership` stays
`off` and emits the exact legacy encoding, so a library user opts in
deliberately.

## Where next

- [The authenticated ledger](../../explanation/authenticated-ledger/) — the
  design: canonical signing bytes, the merge rules, the policy registry and the
  reaper.
- [The ledger](../../explanation/the-ledger/) — what the ledger stores.
- [The security model](../../explanation/security-model/) — what the token
  protects and what it does not.
