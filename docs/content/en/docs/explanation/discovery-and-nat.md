---
title: "Discovery and NAT traversal"
linkTitle: "Discovery and NAT"
weight: 35
description: >
  Placeholder — how peers find each other and get a connection through NAT. Not written yet.
---

{{% pageinfo color="warning" %}}
**This page has not been written.** [Architecture](../architecture/) sketches
the three bootstrap phases in a few paragraphs, and
[relays and hop nodes](../../how-to/relays-and-hop-nodes/) covers the relay case
from the operator's side. Nothing explains the mechanism as a whole, or what to
expect when a given piece of it fails.

What is missing, and where the source is:

- **The OTP rendezvous.** `pkg/discovery/dht.go` derives the DHT rendezvous
  string from a TOTP over the token's OTP key (`Rendezvous()`: TOTP-SHA256, then
  MD5), so the point peers meet at rotates on the token's `otp.dht.interval`
  (see [network config](../../reference/network-config/)). A two-entry ring
  (`rendezvousHistory`, `pkg/discovery/ring.go`) keeps the previous rendezvous
  announced across a rotation so nodes do not lose each other at the boundary.
  The consequences — clock skew between peers, and what a node sees when it
  drifts — are undocumented.
- **DHT versus mDNS.** `--dht` and `--mdns` are both on by default
  (`cmd/util.go`), and they solve different problems: `pkg/discovery/mdns.go`
  finds peers on the same LAN and dials them directly, while the DHT
  (`pkg/discovery/dht.go`) is the internet-wide path and needs bootstrap peers.
  What happens with only one of them enabled is not written down.
- **Hole punching and reachability.** `--holepunch`, `--natservice` and
  `--natmap` (all default on) map onto libp2p's DCUtR, AutoNAT and UPnP
  respectively — see the wiring in `pkg/config/config.go`. Hole punching needs a
  third party both peers can already reach, which is why it interacts with the
  relay settings.
- **Relay fallback.** `--autorelay`, `--autorelay-static-peer`,
  `--autorelay-static-only`, `--autorelay-discovery-interval` and
  `--relay-service`, and the order in which a node tries direct, hole-punched
  and relayed connections.
- **Diagnosing it.** Which of the above a stuck "0 peers" state actually points
  at. See [troubleshooting](../../troubleshooting/) for what exists today.

Contributions welcome — see [contributing](../../contributing/).
{{% /pageinfo %}}
