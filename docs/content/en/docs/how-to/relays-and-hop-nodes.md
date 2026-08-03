---
title: "Relays and hop nodes"
linkTitle: "Relays and hop nodes"
weight: 75
description: >
  Run a node that carries no VPN traffic of its own but helps everyone else connect.
---

Two nodes behind NAT cannot always dial each other. EdgeVPN tries hole punching
first (`--holepunch`, on by default), but hole punching needs a third party that
both peers can already reach, and it does not work through every NAT. A **relay**
is that third party: a node on a reachable address that other peers connect
through.

A relay does not need a VPN interface, an IP on the virtual network, or root.
`edgevpn start` is the command for it:

```bash
edgevpn start --token "$TOKEN"
```

That joins the p2p network — gossip, ledger, discovery, relay service — and
stops there. No TUN device is created, so unlike `edgevpn` it runs unprivileged.

## What a relay actually carries

This is worth being precise about, because "relay" suggests more than EdgeVPN
currently does with it.

- **Gossip and the ledger travel over relays.** The pubsub layer explicitly opts
  into relayed (*limited*) connections, so a peer that can only be reached
  through a relay still sees the ledger, still announces itself, and is still
  discovered by everyone else.
- **Relays are the rendezvous for hole punching.** Once two peers are connected
  through a circuit, libp2p's DCUtR runs over it and tries to upgrade to a direct
  connection. When that succeeds, the relay drops out of the path and the VPN
  works over the direct link.
- **VPN frames do not travel over relays.** The data plane opens its streams from
  a context that does not permit limited connections (`pkg/vpn/vpn.go`), so a
  packet destined for a peer reachable *only* through a circuit fails with:

  ```
  could not open stream to 12D3KooW…: limited connection to peer
  ```

  If you see that line, the relay is doing its job and hole punching is not.
  `--transient-conn` on the root command does not change it: it marks the node's
  own start context, which the frame path does not use.

So a relay fixes discovery and gives hole punching a chance. It is not a fallback
data path for the VPN.

## Running one

Put the relay somewhere with an address other peers can reach — a public IP, or
a port-forwarded host — and pin the port so the address is stable:

```bash
edgevpn start \
  --token "$TOKEN" \
  --listen-maddrs /ip4/0.0.0.0/tcp/4501 \
  --privkey-cache --privkey-cache-dir /var/lib/edgevpn/relay
```

`--privkey-cache` matters more here than on an ordinary node: peers that pin this
relay do so **by peer ID**, and without a persisted key the node comes back with a
different identity after every restart, invalidating every `--autorelay-static-peer`
entry pointing at it. Give each process its own directory — the default is shared
per user, and two processes sharing a key join with the same peer ID.

The two lines you need are printed at startup:

```
Node ID: 12D3KooWRCbBhhGGowBmt1kMEoSqTzEE8o5mMPNPo8WbwCuqsqJz
Node Addresses: [/ip4/10.9.0.23/tcp/4501 /ip4/127.0.0.1/tcp/4501 …]
```

Combine the reachable address with the ID to get the multiaddr other nodes will
use: `/ip4/198.51.100.7/tcp/4501/p2p/12D3KooWRCbB…`.

If the relay is behind a static port-forward, the addresses it discovers locally
are private ones. `--dht-announce-maddrs` replaces what it publishes on the DHT:

```bash
edgevpn start --token "$TOKEN" \
  --listen-maddrs /ip4/0.0.0.0/tcp/4501 \
  --dht-announce-maddrs /ip4/198.51.100.7/tcp/4501
```

{{% alert title="A relay must run the network's ownership mode" color="warning" %}}
A relay holds and gossips the ledger like any other node, so `--ownership` has to
match the rest of the network. A relay left on the wrong mode drops or is dropped
by everyone else's writes. See [ledger ownership](../ledger-ownership/) and the
[compatibility matrix](../../reference/compatibility/).
{{% /alert %}}

`edgevpn start` takes the common flags only — there is no `--api` on it, so a
relay cannot expose the inspection API. To watch a network's state, run
`edgevpn api` on a node that has it.

## Choosing relays from the client side

Every node is an AutoRelay client by default (`--autorelay`, default on). It
finds candidate relays by asking the DHT for the peers closest to itself and
offering those to libp2p, which reserves a slot on the ones that accept.

To pin specific relays instead, list them as multiaddrs:

```bash
sudo edgevpn --token "$TOKEN" \
  --autorelay-static-peer /ip4/198.51.100.7/tcp/4501/p2p/12D3KooWRCbB… \
  --autorelay-static-peer /ip4/198.51.100.8/tcp/4501/p2p/12D3KooWabcd…
```

Static peers are *added* to the DHT-discovered ones. `--autorelay-static-only`
drops the DHT lookup and offers only the peers you listed — the setting to use
when you want traffic to leave through hosts you control.

{{% alert title="Two things to know about the autorelay flags" color="warning" %}}
- **`--autorelay-discovery-interval` currently does nothing.** It is parsed and
  stored, but the value is never handed to libp2p — the autorelay option that
  once took it no longer exists in the version EdgeVPN builds against. Setting it
  has no effect in either direction.
- **Do not combine `--dht=false` with dynamic autorelay.** Relay candidate
  discovery queries the DHT, and with the DHT disabled that lookup dereferences a
  nil pointer and takes the process down when libp2p first asks for candidates.
  If you turn the DHT off, either pass `--autorelay-static-only` together with
  `--autorelay-static-peer`, or turn autorelay off with `--autorelay=false`.
{{% /alert %}}

## Serving as a relay: the resource limits

`--relay-service` (default on) is what makes a node accept reservations and carry
other peers' traffic. Turning it off does **not** stop the node from *using* other
relays — that is `--autorelay` — so a resource-constrained edge device can keep
its own NAT traversal while refusing to carry anyone else's traffic:

```bash
sudo edgevpn --token "$TOKEN" --relay-service=false
```

EdgeVPN deliberately runs wider limits than libp2p's stock circuit-v2 defaults,
because a cluster peer relaying for another cluster peer is a different threat
model from a public relay serving the open internet. The *direction* is the part
worth knowing; for the values, read the
[`start` reference](../../reference/cli/start/) or
[environment variables](../../reference/environment-variables/), which are
generated from the binary and cannot drift from it.

| Knob | What it bounds | libp2p stock | EdgeVPN | Why |
|---|---|---|---|---|
| `--relay-service-max-data` | bytes per direction on one circuit | 128 KiB | raised | 128 KiB resets a circuit almost immediately; cluster transfers (container images, model files) need room |
| `--relay-service-max-duration` | lifetime of one circuit | 2 minutes | raised | long-running relayed sessions instead of forced resets |
| `--relay-service-max-circuits` | concurrent circuits **per peer** | 16 | raised | one peer may hold more simultaneous circuits through this node — it does not change how many peers may use it |
| `--relay-service-buffer-size` | buffer held per circuit | 2 KiB | raised | throughput on large relayed transfers |
| `--relay-service-reservation-ttl` | how long a reservation lasts | 1 hour | unchanged | reservation churn is already tolerable at an hour |

Every one of these costs memory per circuit, so a node that raises them and then
carries many circuits pays for all of them at once. Lower them on small nodes;
`--relay-service=false` is the blunt version.

**How many peers** a relay serves is a different limit, and it is not tunable.
The reservation caps are not exposed as flags and stay at libp2p's defaults: 128
reservations in total, one per peer, 8 per IP and 32 per ASN. A single relay
therefore serves at most 128 peers no matter how the flags above are set —
raising `--relay-service-max-circuits` buys each of those peers more concurrent
circuits, not more peers.

## Restricting who may reserve

`--relay-service-network-only` (default **on**) only lets peers already seen in
the local ledger's alive bucket reserve a slot. Strangers who found the node
through the public DHT are refused, so the relay's bandwidth is spent on the
cluster rather than on the internet at large.

Membership is re-snapshotted from the ledger every
`--relay-service-acl-refresh` (30s by default; keep it at or below the aliveness
announce interval so churn is picked up within a tick or two).

Two behaviours follow from how that set is built, and both matter in practice:

- **The ACL is open until the set is non-empty.** A fresh relay that has not yet
  seen anyone's heartbeat admits everyone; that bootstrap window is deliberate,
  since a node cannot prove membership before it has joined. If the alive bucket
  never fills — nobody running the aliveness service — the ACL stays open forever
  and logs a line per refresh at debug level.
- **An `edgevpn start` node never appears in that set.** `start` runs no
  aliveness service and publishes no heartbeat, so it is invisible to every
  network-only ACL on the network. That is harmless for a relay, which *accepts*
  reservations rather than making them, but it means a `start` node behind NAT
  cannot reserve a slot on someone else's network-only relay. Relays belong on
  reachable hosts anyway.

To open a relay to peers outside the cluster, pass
`--relay-service-network-only=false` — and read
[the security model](../../explanation/security-model/#relay-acls-protect-your-bandwidth)
first. Note that this is a bandwidth-abuse control, not access control: everyone
it admits already holds the token.

## Checking it works

On the relay, at the default `--log-level info`, startup prints the node ID, the
listen addresses, and the ownership mode. Two more lines are worth recognising
because they look like problems and are not:

```
connmanager disabled
 go-libp2p resource manager protection disabled
```

Both are the defaults: connection watermarks are off until you set
`--connection-low-water` *and* `--connection-high-water` (both, to non-zero), and
the libp2p resource manager is off until `--limit-enable` — and stays off on
macOS regardless. On a relay carrying other peers' traffic, both are worth
turning on.

Reservation activity is logged by libp2p itself, not by EdgeVPN, so raise
`--libp2p-log-level` (default `fatal`) to see it:

```bash
edgevpn start --token "$TOKEN" --libp2p-log-level info
```

On a client, the sign that relaying is in use is a peer address containing
`/p2p-circuit`. The sign that it is *stuck* there — that hole punching never
completed — is the `limited connection to peer` line above.

## Where next

- [`edgevpn start` reference](../../reference/cli/start/) — the generated flag
  list, always in sync with the binary.
- [Version and wire-format compatibility](../../reference/compatibility/) — before
  you upgrade a relay separately from the rest of the network.
- [Ledger ownership](../ledger-ownership/) — the one setting every node,
  relays included, must agree on.
- [The security model](../../explanation/security-model/) — what the relay ACL
  does and does not protect.
- [Troubleshooting](../../troubleshooting/) — nodes that never see each other.
