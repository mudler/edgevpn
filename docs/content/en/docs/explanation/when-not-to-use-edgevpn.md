---
title: "When not to use EdgeVPN"
linkTitle: "When not to use it"
weight: 40
description: >
  What the decentralized design costs you, and the cases it is not a good fit for.
---

## Is it for me?

EdgeVPN makes VPN decentralization a first strong requirement.

Its main use is for edge and low-end devices and especially for development.

The decentralized approach has few cons:

- The underlying network is chatty. It uses a Gossip protocol for synchronizing
  the routing table and p2p. Every blockchain message is broadcasted to all
  peers, while the traffic is to the host only.
- Might be not suited for low latency workload.

Keep that in mind before using it for your prod networks!

But it has a strong pro: it just works everywhere libp2p works!

## Warning

{{% pageinfo color="warning" %}}
I'm not a security expert, and this software didn't went through a full
security audit, so don't use and rely on it for sensible traffic and not even
for production environment! I did this mostly for fun while I was experimenting
with libp2p.
{{% /pageinfo %}}
