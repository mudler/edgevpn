---
title: "Architecture"
linkTitle: "Architecture"
weight: 10
aliases:
  - /docs/concepts/architecture/
description: >
  How EdgeVPN's p2p network, encryption and ledger fit together.
resources:
- src: "**edgevpn_*.png"
---

 
## Introduction

EdgeVPN uses [libp2p](https://github.com/libp2p/go-libp2p) to establish a
decentralized, asymmetrically encrypted gossip network which propagates a
(symmetrically encrypted) ledger state between nodes.

The ledger is a hash-linked chain of blocks, and it is deliberately minimal:

- There is no proof of work and no consensus protocol. A block is just
  `Index`, `Timestamp`, `Storage`, `Hash` and `PrevHash`, where `Hash` is a
  SHA256 over the other four — `Index`, `Timestamp`, `Storage` and `PrevHash`.
- There is no DAG, CARv2 or GraphSync. The chain holds metadata only —
  service and file names, machine records, DNS entries, IP allocations,
  heartbeats — never addressable content.

Because there is no consensus round, "blockchain" here means *hash-chained
gossiped state*, not a distributed ledger with agreement guarantees. Nodes
converge because every node keeps re-announcing its own entries and merges what
it receives; a node that has just joined, or one that has restarted with an
empty chain, is refilled by those re-announcements.

EdgeVPN uses the ledger to store Services UUID, Files UUID, VPN and other
metadata (such as DNS records, IP, etc.) and to co-ordinate events between the
nodes of the network.

## Where the state lives

By default the chain is kept in memory only, and a node that restarts starts
from an empty chain and refills it from the network. Starting a node with
`--ledger-state <dir>` swaps the in-memory store for a disk-backed one, so the
chain survives restarts. Persisting it is recommended when running
[trusted networks](../../how-to/trusted-networks/), so that authorization keys
do not have to be re-seeded on every restart.

## Ownership of ledger entries

Entries are not anonymous. Every write a node makes is signed with that node's
libp2p private key, and carries the author's peer ID, a monotonic version, an
`UpdatedAt` timestamp and a signature over a canonical encoding of all of them.
Verification needs no key distribution: the public key is recovered from the
owner's peer ID, which embeds it.

When merging an incoming block, a node applies a per-key policy rather than
replacing the whole chain:

- Buckets such as machines, services, files, users, DNS, heartbeats and egress
  are *owned*. A peer cannot overwrite a live entry that belongs to another
  peer, replay an older version over a newer one, or forge an entry whose value
  claims a different peer as its author.
- Buckets that are not registered — including anything you create yourself
  through the API — are open: the highest version wins and anyone may write
  them.
- Owned entries have a lease. Most of them are alive only while their owner's
  heartbeat is fresh (an eight minute window by default, `--ownership-ttl`); the
  heartbeat itself expires on an absolute TTL. The elected leader periodically
  reaps expired entries by writing signed tombstones, and prunes old tombstones
  once every node has had time to see them.

This is controlled by `--ownership`, which defaults to `enforce`. `observe`
runs the same merge but accepts and logs violations instead of dropping them,
and `off` restores the legacy behaviour: unsigned entries and a whole-block
replace where the higher block index wins. All nodes of a network must agree on
the mode, because the wire format differs.

For the details, see
[the authenticated ledger](../authenticated-ledger/).

## What this does and does not protect

The ledger authenticates *authorship*, not *membership*. Anything that holds the
network token can join the gossip network, announce itself, and write to any
open bucket. Ownership stops a member from impersonating or evicting another
member; it does not stop a token holder from being a member in the first place.

The VPN data plane does check the ledger: an inbound VPN stream is reset unless
the remote peer appears in the machines bucket (or in a configured static peer
table), and packets are only routed to a destination IP that resolves to a live
machine entry. So a node that is not in the ledger cannot exchange VPN traffic —
but since a peer announces its own machine entry, this is a routing table, not
an access control list.

Restricting *which* peers may join is the job of PeerGuardian and peergating,
which are opt-in (`--peerguard`, `--peergate`); see
[trusted networks](../../how-to/trusted-networks/) and the
[security model](../security-model/).

## Layers

- Simple (KISS) interface to display network data from the ledger
- asymmetric p2p encryption between peers with libp2p
- rendezvous points dynamically generated from OTP keys
- extra AES symmetric encryption on top, in case the rendezvous point is
  compromised
- the ledger acts as a sealed encrypted store for the routing table
- connections are created host to host and encrypted asymmetrically

### Connection bootstrap

Network is bootstrapped with libp2p and is composed of 3 phases:

{{< imgproc edevpn_bootstrap.png Fit "1200x550" >}}
{{< /imgproc >}}

In the first phase, nodes do discover each others via DHT and a rendezvous
secret which is automatically generated via OTP.

Once peers know about each other a gossip network is established, where the
nodes exchange ledger blocks over a p2p e2e encrypted channel. The messages are
sealed with a symmetric AES key which is rotated via OTP and shared between the
nodes.

At that point a ledger and an API is established between the nodes, and
optionally start the VPN binding on the tun/tap device.
