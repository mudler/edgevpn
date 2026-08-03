---
title: "Ledger buckets"
linkTitle: "Ledger buckets"
weight: 45
description: >
  The buckets EdgeVPN keeps in the shared ledger, what their keys mean, and who writes and reads each one.
---

The ledger is a two-level map: **bucket → key → value**
(`map[string]map[string]SignedData` in `pkg/blockchain/block.go`). A *bucket* is
just the first level — a namespace. There is no schema, no registration step and
no fixed list: a bucket springs into existence the moment something writes a key
into it, including a `PUT /api/ledger/<bucket>/<key>/<value>` from the
[API](../api/) with a name nobody has ever used.

What follows is the set of buckets **EdgeVPN itself uses**. The names are
declared in `pkg/protocol/protocol.go`; the key semantics below are what the
code that writes and reads them actually does, and they are the part that trips
people up — `machines` is keyed by **IP address**, and `dns` by **regular
expression**, not by hostname.

## Summary

| Bucket | Key | Value | Written by | Read by |
|---|---|---|---|---|
| `machines` | VPN IP address (`10.1.0.11`) | `types.Machine` | the VPN service, on every announce | packet routing, DHCP, `/api/machines` |
| `users` | peer ID | `types.User` | a peer before it dials a service, file or egress | the service/file/egress stream handlers, `/api/users` |
| `services` | service name (`--name` / `service-add`) | `types.Service` | the node exposing the service | `service-connect`, `/api/services` |
| `files` | file name (`--name` / `file-send`) | `types.File` | the node sharing the file | `file-receive`, `/api/files` |
| `healthcheck` | peer ID | RFC3339 UTC timestamp, as a string | the alive service, every heartbeat | liveness for every other bucket, `/api/nodes`, relay ACLs |
| `dns` | a **regular expression** | `types.DNS` (`map[dns.Type]string`) | `edgevpn dns`, `POST /api/dns` | the embedded DNS server, `/api/dns` |
| `egress` | peer ID | the literal string `ok` | a node started with the egress service | the HTTP proxy when picking an egress |
| `trustzone` | peer ID | empty string | PeerGuardian, after a peer passes a challenge | PeerGater, when gating gossip |
| `trustzoneAuth` | provider-prefixed name (`ecdsa_1`) | provider data (an ECDSA public key) | **you**, by hand, via the API | the auth providers, when validating challenges |
| `dhcp` | the literal key `leader` | peer ID of the current lease leader | the DHCP service during leader election | the DHCP service |

`dhcp` is the one bucket with no constant in `pkg/protocol/protocol.go` — it is
written as a bare string literal in `pkg/vpn/dhcp.go`. It still shows up in
`/api/ledger` like any other.

Values are Go structs serialised with no `json` struct tags, so field names come
back **PascalCase** (`PeerID`, `Hostname`) — see
[response format](../api/#response-format).

## machines

Keyed by the node's **VPN IP address without the mask** — `10.1.0.11`, not
`10.1.0.11/24` and not a peer ID. `pkg/vpn/vpn.go` parses `--address` and uses
the resulting IP as the key, re-announcing whenever the entry is missing or
names a different peer.

The value is `types.Machine`: `PeerID`, `Hostname`, `OS`, `Arch`, `Address`,
`Version`.

This bucket is the routing table. When the VPN has a packet for `10.1.0.12` it
looks that address up here to find the peer ID to open a stream to; if the
lookup misses, the packet is dropped. It is also what
[DHCP](../../how-to/addressing-and-dhcp/) reads to work out which addresses are
already taken, and what `/api/machines` returns.

## users

Keyed by **peer ID**, value `types.User` (`PeerID`, `Timestamp`).

This is not a user directory, despite the name. It is a "I am about to connect
to you" announcement: `file-receive`, `service-connect` and the egress client
each add their own peer ID here before dialling, and the corresponding stream
handlers on the serving side reset any incoming stream whose remote peer is not
present in this bucket. A node that never consumes anything never appears in it.

## services

Keyed by the **service name** you chose (`edgevpn service-add --name mysvc`, or
the `serviceID` argument in the library API), value `types.Service` (`PeerID`,
`Name`).

The exposing node re-announces the entry whenever it is missing or points at a
different peer, so two nodes exposing the same name will fight over the key —
under `--ownership enforce` the peer that claimed it first keeps it for as long
as it stays alive. The connecting side looks the name up to find which peer to
dial. See
[tunnel TCP services](../../how-to/tunnel-tcp-services/).

## files

Keyed by the **file name** you chose (`edgevpn file-send --name myfile`), value
`types.File` (`PeerID`, `Name`). Exactly the same shape as `services`: the
sharing node announces, `file-receive` polls the bucket until the name appears
and then opens a stream to the peer named in the value. The file contents never
enter the ledger. See
[send and receive files](../../how-to/send-and-receive-files/).

## healthcheck

Keyed by **peer ID**, value the peer's own clock as an RFC3339 UTC timestamp
string (`pkg/services/alive.go`).

This is the heartbeat, and it is the bucket every other bucket depends on: a
peer is "alive" if its timestamp here is newer than the liveness window, and
under `--ownership enforce` an entry in `machines`, `services`, `files`, `users`,
`dns` or `egress` is only honoured while its owner is alive. Its own entries age
out on an absolute TTL rather than on liveness, for the obvious reason.
`/api/nodes` and the [relay ACL](../../how-to/relays-and-hop-nodes/) read it too.

## dns

Keyed by a **regular expression**, not a hostname. The value is `types.DNS`, a
map from DNS record type to value — `{"A": "10.1.0.11"}`.

The resolver in `pkg/services/dns.go` walks every key in the bucket, compiles it
as a Go regexp and returns the first entry that matches the queried name. Two
consequences worth internalising:

- A key of `foo.bar` matches any name *containing* `foo.bar`, because the
  pattern is unanchored. Anchor it (`^foo\.bar\.$`) if you want an exact name.
  Queried names arrive with the trailing dot, as `foo.bar.`.
- Match order across the bucket is map iteration order, so overlapping patterns
  resolve non-deterministically. Keep patterns disjoint.

See [enable the DNS server](../../how-to/enable-dns/).

## egress

Keyed by **peer ID**, value the literal string `ok` — presence is the whole
signal. A node running the egress service re-announces its own key; the HTTP
proxy intersects this bucket with the set of currently alive peers and picks one
at random to forward a request through. See
[HTTP egress and the proxy](../../how-to/http-egress-and-proxy/).

## trustzone and trustzoneAuth

These two belong to the experimental `--peerguard` machinery
([trusted networks](../../how-to/trusted-networks/)). Together with `dhcp` they
are the EdgeVPN-defined buckets absent from the policy registry in
`pkg/blockchain/policy.go`, so they take the zero policy: no owner, no expiry,
open to any writer. A node that signs still signs what it writes here — what
the zero policy skips is the *verification*, so an incoming entry is taken
whenever its version is strictly higher, whoever wrote it.

- **`trustzone`** is keyed by peer ID with an empty value. PeerGuardian adds a
  peer's ID once one of the configured auth providers accepts a challenge
  response from it; PeerGater turns the key set into the list of senders whose
  gossip is accepted. With `autocleanup`, keys for peers no longer seen in the
  message hub are deleted.
- **`trustzoneAuth`** holds the *provider* configuration — the material used to
  validate challenges, not the peers that passed them. Keys are namespaced by
  provider: the ECDSA provider picks up every key containing `ecdsa` and treats
  its value as a public key to verify against. This is the bucket the API
  reference writes to in its example:

  ```bash
  curl -X PUT 'http://localhost:8080/api/ledger/trustzoneAuth/ecdsa_1/<pubkey>'
  ```

  Because the bucket is open, anyone who can reach any node's API — or any peer
  already permitted to gossip — can add a key here and thereby authorise new
  peers. It is a distribution mechanism for trust anchors, not a protected one.
  See [the security model](../../explanation/security-model/).

## Ownership and expiry

Whether a bucket is signed, who owns an entry and when it expires is a separate
concern, defined once in `pkg/blockchain/policy.go`. The operator-facing table
is in [ledger ownership](../../how-to/ledger-ownership/); the design note is
[the authenticated ledger](../../explanation/authenticated-ledger/). In short:
`machines`, `services`, `files`, `users`, `egress`, `healthcheck` and `dns` are
owned and expiring; `trustzone`, `trustzoneAuth`, `dhcp` and any bucket you
invent yourself are open and permanent.
