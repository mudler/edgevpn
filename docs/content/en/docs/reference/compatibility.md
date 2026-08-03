---
title: "Version and wire-format compatibility"
linkTitle: "Compatibility"
weight: 50
description: >
  Which EdgeVPN versions and which --ownership modes can share a network, and how to move between them.
---

Two nodes holding the same token are on the same network only if they also agree
on the format of what they gossip. Almost every EdgeVPN setting is a local
choice; a small number are not. This page is the list of the ones that are not.

## How to tell what a node is running

Release binaries carry their tag:

```bash
edgevpn --version
```

```
edgevpn version v0.35.3
```

Every command also logs it at startup, alongside the ownership mode:

```
Version: v0.35.3 commit: a9f4e17af58d565a0a2caee26be8ea0311d0a7e9
ledger ownership enforcement: mode=2 ttl=2m0s
```

Released binaries print the mode as an **integer**: `1` is `observe`, `2` is
`enforce`. There is no `mode=0` — the line is only logged when enforcement is
on, so `off` prints nothing at all (and so does a node whose host private key
was unavailable, which logs `ownership enforcement requested but host private
key is unavailable` instead and runs unsigned). The window is `2m0s` on every
released version. Builds newer than v0.35.3 print the name (`mode=enforce`) and
derive the window from the heartbeat, so they show `ttl=8m0s` on stock
settings — a quick way to tell a released binary from a build off `master`.

A binary built from source without the release `-ldflags` reports an empty
version (`Version:  commit:`) and has no `--version` flag at all. If you build
your own, record the commit yourself — a network of self-built binaries has no
other way to answer "what are these nodes running".

## The only wire format that changes: `--ownership`

`--ownership` selects how ledger entries are encoded, so nodes that disagree do
not merely behave differently — they cannot read each other.

- **Unsigned (legacy) entries** are bare JSON values. Every EdgeVPN release up to
  and including **v0.34.0** speaks only this, and so does any newer node running
  `--ownership off`.
- **Signed entries** are JSON objects carrying the author's peer ID, a version, a
  timestamp and a signature. `observe` and `enforce` emit these.

The decoder used by `observe` and `enforce` accepts *both* shapes. The decoder in
releases up to v0.34.0 accepts only the first, and a single signed entry
anywhere in an incoming block makes the **whole block** undecodable to it — not
just that entry.

See [ledger ownership](../../how-to/ledger-ownership/) for what the modes do and
[the authenticated ledger](../../explanation/authenticated-ledger/) for the
design.

### Which release introduced it

Ownership landed in **v0.35.0**, in commit `1c969bd` — which is the commit
v0.35.0 tags, so there is no intermediate release that has part of it. It has
been present in every release since (v0.35.0, v0.35.1, v0.35.2, v0.35.3), and it
has defaulted to `enforce` from the first of them: **a v0.35.x node started with
no ownership flags will not interoperate with a v0.34.0 node.**

That is the upgrade trap. Nothing about the upgrade announces it.

## The matrix

Rows write, columns read. "≤ v0.34.0" means any release before ownership existed;
those binaries have no `--ownership` flag.

| Writer → Reader | ≤ v0.34.0 | v0.35+ `off` | v0.35+ `observe` | v0.35+ `enforce` |
|---|---|---|---|---|
| **≤ v0.34.0** | works | works | works (accepted, logged) | **dropped** (silent to the writer) |
| **v0.35+ `off`** | works (see below) | works | works (accepted, logged) | **dropped** (silent to the writer) |
| **v0.35+ `observe`** | **whole blocks dropped** | works | works | works |
| **v0.35+ `enforce`** | **whole blocks dropped** | works | works | works |

The `off` → `≤ v0.34.0` cell has an exception. An `off` node adopts incoming
blocks wholesale, so in a mixed network it stores and re-broadcasts signed
entries authored by others; its own blocks then contain signed entries too, which
a pre-v0.35 node cannot decode either. `off` is reliably legacy-compatible only
in a network that contains no signing nodes at all.

Reading the rest of it:

- **`off` ↔ `enforce` fails in one direction and says nothing on the other.** The
  `enforce` node logs each dropped write; the `off` node logs nothing, because
  from its point of view everything it receives decodes and everything it sends
  is sent.
- **`observe` is compatible with both `off` and `enforce`.** It signs (so
  `enforce` accepts it) and it accepts unsigned writes (so `off` reaches it).
  That is the whole reason for the `off → observe → enforce` sequence.
- **`observe` is *not* compatible with pre-v0.35 binaries.** It signs, and old
  binaries cannot decode signed entries. Against a v0.34 node, `observe` is as
  incompatible as `enforce`. Only `off` is.

### What each failure looks like

| Where | Log line | Level |
|---|---|---|
| `enforce` node receiving an unsigned entry | `ownership violation (rejected): machines/10.1.0.9 from : invalid signature` | warn |
| `observe` node receiving an unsigned entry | `ownership violation (observe, accepting): machines/10.1.0.9 from : invalid signature` | warn |
| pre-v0.35 node receiving a signed block | `handler error: failed unmarshalling blockchain data: json: cannot unmarshal object into Go struct field Block.Storage of type blockchain.Data` | warn |
| `off` node in a mixed network | *nothing* | — |

The empty owner between `from` and `:` marks a legacy write, as opposed to a
genuine ownership violation, which names the offending peer.

## Upgrading a network across the boundary

The binary upgrade and the mode change are two separate migrations, and they have
to happen in that order.

1. **Upgrade the binaries, pinning the old wire format.** Roll v0.35.x out one
   node at a time with `--ownership off` (or `EDGEVPNOWNERSHIP=off`). A v0.35
   node in `off` mode is wire-identical to a v0.34 node in both directions, so
   the network keeps working throughout with any mixture of old and new binaries.

   Do **not** skip this by letting the new nodes take their `enforce` default:
   every node you upgrade would then vanish from the ones you have not.

2. **Once every node is on v0.35.x, change the mode.** Follow
   [changing the mode on a live network](../../how-to/ledger-ownership/#changing-the-mode-on-a-live-network):
   `off` → `observe` everywhere, wait for the `ownership violation (observe,
   accepting)` lines to stop, then `observe` → `enforce` everywhere. Each step is
   safe one node at a time.

Downgrading reverses it: `enforce` → `observe` → `off` on every node first, and
only then replace binaries with a pre-v0.35 release.

## What is *not* a compatibility concern

These differ freely between nodes on one network:

- **All relay settings.** `--relay-service*` and `--autorelay*` are per-node
  resource and policy choices. See
  [relays and hop nodes](../../how-to/relays-and-hop-nodes/).
- **Resource limits, connection watermarks, log levels, API settings.**
- **`--privkey-cache`.** Identity persistence is per-node.

The ledger protocol identifiers (`/edgevpn/0.1` and the service, file and egress
protocols) have not changed across the history of those files, and neither has
the block structure apart from the entry encoding described above.

## `--ownership-ttl` is not a wire format, but it still has to match

The TTL is a local judgement about when a peer counts as dead, so nodes that
disagree can still read each other. That is the whole of the good news. Under
`observe`/`enforce` the merge is per-key and nothing reconciles whole blocks
afterwards, so two nodes running different windows durably disagree on ledger
*state*: the node with the shorter window declares an owner dead, stops routing
to it, and lets its addresses, services and DNS names be reclaimed — or reaps
them outright if it is the leader — while the node with the longer window still
holds the original entry and still routes to the original owner. Nothing
resolves the split until the owner re-announces.

Set the same value on every node. Leaving it at the default `0` does that for
you: the window is derived from `--aliveness-healthcheck-interval` (4×, so 8
minutes on the stock 120-second heartbeat), so nodes stay in agreement as long
as the heartbeat interval matches too.

## What this page does not establish

Stated plainly, because guessing here is worse than a gap:

- **Interoperability *among* pre-v0.35 releases was not audited.** The claim
  above is only that they share one entry encoding with `--ownership off`.
- **Transport-level compatibility across large libp2p version gaps was not
  tested.** EdgeVPN v0.35.x builds against go-libp2p v0.48.0. Multistream
  negotiation is designed to be backward compatible, but very old EdgeVPN
  releases have not been run against current ones here.
- **Token and network-configuration format** compatibility across releases was
  not tested beyond noting that the fields have not changed.

## Fixes that are not in any release yet

Three ownership defects are fixed in the source tree but appear in **no released
version**. If you are running v0.35.0 through v0.35.3, they are all present:

| Defect | Symptom on a released binary | Workaround until a release ships |
|---|---|---|
| An unrecognised `--ownership` value fell through to `off` | A typo (`--ownership enabled`, `--ownership true`) silently disabled ledger authentication, with no error and no warning | Confirm the mode in the startup log (`ledger ownership enforcement: mode=…`, where `1` is `observe` and `2` is `enforce`) rather than trusting the flag. The line's absence means the node is unsigned — either `off`, or enforcement was requested but the host private key was unavailable, which logs a warning of its own |
| The liveness window was a fixed 2 minutes | Shorter than the 180 s a healthy node can take between jittered heartbeats, so live nodes were intermittently treated as inactive: packets dropped, addresses reclaimable, entries tombstoned by a leader scrub | Set `--ownership-ttl 480` explicitly on every node |
| A persisted unsigned entry could not be replaced or deleted by its own owner | With `--ledger-state`, entries written under `off`/`observe` were frozen after a restart into `enforce`; deleting one succeeded locally and was rejected everywhere else, diverging the network with nothing logged on the node that issued it | Clear the state directory before restarting a node into `enforce` |

All three are described in more detail on
[ledger ownership](../../how-to/ledger-ownership/).
