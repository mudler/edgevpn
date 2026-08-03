---
title: "Addressing and DHCP"
linkTitle: "Addressing and DHCP"
weight: 20
description: >
  Assign virtual addresses by hand, let peers negotiate them, or pin a static peer table.
---

Every VPN node needs a virtual address on the `edgevpn0` interface. There are
three ways to get one, and two extra flags that change how packets are routed
once you have it.

## Static addresses with `--address`

`--address` takes a CIDR and defaults to `10.1.0.1/24`. It is the address the
node is reachable at from inside the VPN, and it must be unique across the
network — nothing checks this for you.

```bash
# on Node A
$ EDGEVPNTOKEN=.. edgevpn --address 10.1.0.11/24
# on Node B
$ EDGEVPNTOKEN=.. edgevpn --address 10.1.0.12/24
```

The interface name defaults to `edgevpn0` and can be changed with
`--interface` (or `IFACE`).

## Automatic addresses with `--dhcp`

{{% pageinfo color="warning"%}}
Experimental feature!
{{% /pageinfo %}}

`--dhcp` lets peers negotiate addresses among themselves over the ledger, so
you do not have to keep a list of who has which IP. There is no DHCP server:
the allocation is agreed on through the shared ledger like every other piece of
network metadata.

```bash
$ EDGEVPNTOKEN=.. edgevpn --dhcp
```

With `--dhcp` enabled, `--address` can be omitted. If an address *is* given, it
is the base the allocator counts up from when picking the next free IP, not a
reservation for this node. The allocated subnet is `/24`.

Once a node has an address it writes a lease under `--lease-dir` (default
`$HOME/.edgevpn/leases`) and reuses it on the next start. Allocation needs at
least two nodes visible on the ledger, so a single node started with `--dhcp`
waits until a peer shows up.

## Sending everything to one node with `--router`

`--router` takes the virtual address of another node in the network:

```bash
$ EDGEVPNTOKEN=.. edgevpn --address 10.1.0.11/24 --router 10.1.0.1
```

When a packet originates on this node and its destination is *not* a machine
announced on the ledger, the packet is sent to the router node instead of being
dropped. Packets addressed to nodes that are on the ledger still go directly to
that peer — `--router` is a fallback for unknown destinations, not a blanket
redirect.

The router node is a normal EdgeVPN node. What it does with the traffic it
receives — forwarding it to a LAN, to the internet, or nowhere — is up to that
host's own kernel routing and NAT configuration; EdgeVPN itself only delivers
the packet to it.

## Pinning the routing table with `--static-peertable`

`--static-peertable` takes one or more `ip:peerid` pairs and can be repeated:

```bash
$ edgevpn --address 10.1.0.11/24 \
    --static-peertable 10.1.0.12:12D3KooW... \
    --static-peertable 10.1.0.13:12D3KooW...
```

The value must contain exactly one `:` separating the virtual IP from the
libp2p peer ID; anything else is a startup error.

Setting it changes the node's behaviour in two ways:

- **Outbound**, the ledger's machine table is not consulted at all. A
  destination that is not in the static table is unroutable, even if the peer
  is announcing itself on the ledger.
- **Inbound**, only peers whose IDs appear in the static table may open a VPN
  stream to this node. Streams from anyone else are reset.

That makes it a way to run the VPN with a fixed, hand-maintained topology
instead of a discovered one. It is per-node configuration: a node with a static
peer table and a node without can coexist in the same network, and each applies
its own rule.

The equivalent environment variable is `EDGEVPNSTATICPEERTABLE`.
