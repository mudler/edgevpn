---
title: "WebUI and API"
linkTitle: "WebUI and API"
weight: 40
aliases:
  - /docs/getting-started/api/
description: >
  Query the network status and operate the ledger with the built-in HTTP API.
---


EdgeVPN embeds an HTTP API with a small web UI on top of it. The API exposes the
shared ledger, the peers a node knows about and libp2p bandwidth counters.

To start a node in API-only mode, run:

```bash
$ edgevpn api
```

with either a `EDGEVPNCONFIG` or `EDGEVPNTOKEN` set (or `--config <file>`).

In API mode, EdgeVPN will connect to the network without routing any packet, and
without setting up a VPN interface.

By default the API listens on `127.0.0.1:8080`. Use `--listen` to change it, and
see `edgevpn api --help` for the other options.

The API can also be started together with the VPN with `--api`; in that case the
address is set with `--api-listen` instead (see
[Binding to a socket](#binding-to-a-socket) below).

## The API has no authentication

{{% alert title="The API is a full control plane, and it is unauthenticated" color="warning" %}}
There is no authentication, authorization or CSRF protection on any route.
Anything that can open a TCP connection to the API port can read the whole
ledger and write to it, for example with:

```bash
$ curl -X PUT 'http://localhost:8080/api/ledger/<bucket>/<key>/<value>'
```

Writes are announced to the rest of the network, so an unprotected API port is
enough to inject DNS records, service and file announcements into a network
you are not otherwise a member of.

Keep the listener on loopback (the default) or on a unix socket, and treat
exposing it on a routable address as equivalent to handing out the network
token.
{{% /alert %}}

Ledger writes authored by a node are signed with that node's libp2p identity and
merged under the per-bucket ownership rules, so a peer cannot silently overwrite
entries owned by another live peer. That constrains *what* an API caller can
forge on other nodes, but it does not authenticate the caller: whatever the API
writes is signed as the node running it. See the
[security model](../../explanation/security-model/) and
[the ledger](../../explanation/the-ledger/).

## Response format

Responses are JSON encoded straight from the Go types, which carry no `json`
struct tags. Field names are therefore **PascalCase** exactly as declared in the
source: `PeerID`, `RateIn`, `BlockChain`, `NodeID`, and so on.

Ledger reads (`/api/ledger...`) return only the *values* of the entries. The
signature envelope (`Owner`, `Version`, `UpdatedAt`, `Deleted`, `Sig`) is visible
through `/api/blockchain`, which returns the raw last block.

## API endpoints

### GET

#### `/api/summary`

Counters for the current node: number of files, machines, users and services in
the ledger, the current block index, the number of nodes seen on the gossip
topic, the size of the libp2p peerstore, and this node's peer ID.

```bash
$ curl -s http://localhost:8080/api/summary
{"Files":0,"Machines":0,"Users":0,"Services":0,"BlockChain":0,"OnChainNodes":0,"Peers":1,"NodeID":"12D3KooW..."}
```

#### `/api/users`

Returns the users connected to services in the blockchain

#### `/api/services`

Returns the services running in the blockchain

#### `/api/files`

Returns the files announced to the ledger (`PeerID`, `Name`)

#### `/api/dns`

Returns the domains registered in the blockchain

#### `/api/machines`

Returns the machines connected to the VPN. Each entry is the ledger `Machine`
record plus `Connected` (a live libp2p connection exists), `OnChain` (the peer
is on the gossip topic) and `Online` (the peer announced itself recently).

#### `/api/nodes`

Returns the peers currently considered online: the union of the gossip topic
members and the peers whose healthcheck entry in the ledger is younger than 10
minutes.

#### `/api/peerstore`

Returns every peer ID in the local libp2p peerstore — peers this node has
discovered, whether or not they are part of the network.

The two lists overlap but neither contains the other. `/api/nodes` is read
partly from the ledger, so it can name a peer that announced a healthcheck but
that this node has never met and so is absent from its peerstore; conversely the
peerstore holds peers discovered over the DHT that never announced themselves.

#### `/api/blockchain`

Returns the latest available block, including the full signature envelope of
every entry

#### `/api/ledger`

Returns the current data in the ledger. For what the buckets are called and what
their keys mean, see [ledger buckets](../ledger-buckets/).

#### `/api/ledger/:bucket`

Returns the current data in the ledger inside the `:bucket`

#### `/api/ledger/:bucket/:key`

Returns the current data in the ledger inside the `:bucket` at given `:key`

#### `/api/peergate`

Returns peergater status.

Registered only when the node runs with `--peerguard`; otherwise it returns
`404`. See [Trusted networks](../../how-to/trusted-networks/).

### Metrics

The metrics endpoints report the libp2p bandwidth counters. They are registered
only when the node was given a bandwidth reporter. Every CLI entry point that
serves the API (`edgevpn api`, `edgevpn --api`, `edgevpn proxy`) attaches one, so
in practice they are always present; a `404` here means the API was started
programmatically without a reporter, not that something is broken.

All of them return a `metrics.Stats` object:

```bash
$ curl -s http://localhost:8080/api/metrics
{"TotalIn":0,"TotalOut":0,"RateIn":0,"RateOut":0}
```

#### `/api/metrics`

Aggregate bandwidth totals and rates for the node

#### `/api/metrics/protocol`

Bandwidth broken down by libp2p protocol ID

#### `/api/metrics/protocol/:protocol`

Bandwidth for a single protocol. The protocol ID contains slashes, so it has to
be URL-encoded:

```bash
$ curl -s 'http://localhost:8080/api/metrics/protocol/%2Fedgevpn%2F0.1'
```

#### `/api/metrics/peer`

Bandwidth broken down by peer ID

#### `/api/metrics/peer/:peer`

Bandwidth for a single peer, by peer ID

### PUT

#### `/api/ledger/:bucket/:key/:value`

Puts `:value` in the ledger inside the `:bucket` at given `:key`. Returns
`{"State":"Announcing"}` — the write is queued for announcement, so it is not
visible to a subsequent `GET` until it has been committed to a block.

#### `/api/peergate/:state`

Enables/disables peergating (only present with `--peerguard`):

```bash
# enable
$ curl -X PUT 'http://localhost:8080/api/peergate/enable'
# disable
$ curl -X PUT 'http://localhost:8080/api/peergate/disable'
```

### POST

#### `/api/dns`

The endpoint accept a JSON payload of the following form:

```json
{ "Regex": "<regex>", 
  "Records": { 
     "A": "2.2.2.2",
     "AAAA": "...",
  },
}
```

Takes a regex and a set of records and registers them to the blockchain.

The DNS table in the ledger will be used by the embedded DNS server to handle requests locally.

To create a new entry, for example:

```bash
$ curl -X POST http://localhost:8080/api/dns --header "Content-Type: application/json" -d '{ "Regex": "foo.bar", "Records": { "A": "2.2.2.2" } }'
```

### DELETE

#### `/api/ledger/:bucket/:key`

Deletes the `:key` into `:bucket` inside the ledger

#### `/api/ledger/:bucket`

Deletes the `:bucket` from the ledger

### Debug endpoints

#### `/debug/pprof/*`

When the node is started with `--debug`, the Go `net/http/pprof` handlers are
mounted under `/debug/pprof/`. They are not mounted otherwise.

Like the rest of the API these are unauthenticated, and they expose goroutine
stacks and heap profiles of the process — do not enable `--debug` on a node
whose API port is reachable by anything you do not trust.

### Web UI

Every other path is served from the assets embedded in the binary: the single
page UI at `/`, with sections for nodes, DNS, the blockchain, services and
peers. It is a read/write front-end for the endpoints above and inherits the
same lack of authentication.

## Binding to a socket

The API can also be bound to a unix socket, for instance:

```bash
$ edgevpn api --listen "unix://<path/to/socket>"
```

or as well while running the vpn, where the flag is `--api-listen`:

```bash
$ edgevpn --api --api-listen "unix://<path/to/socket>"
```

The socket is created with mode `0660`, which can be overridden with the
`APILISTENUNIXMODE` environment variable. systemd socket activation is honoured:
if a `.socket` unit passes a listener, EdgeVPN inherits it and leaves the socket
file's ownership and permissions alone.

A unix socket is the recommended way to run the API on a shared host, because
filesystem permissions are the only access control the API has.
