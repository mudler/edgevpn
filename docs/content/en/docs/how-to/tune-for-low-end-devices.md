---
title: "Tune for low-end devices"
linkTitle: "Tune for low-end devices"
weight: 140
description: >
  Placeholder — cutting memory, connection and file-descriptor usage on small hardware. Not written yet.
---

{{% pageinfo color="warning" %}}
**This page has not been written.** EdgeVPN has a full set of resource-limiting
flags and none of them are explained beyond their `--help` line. There is no
guidance on which to reach for on a Raspberry Pi, a router or a container with a
tight memory cgroup.

What is missing, and where the source is:

- **`--low-profile`** (env `EDGEVPNLOWPROFILE`) is **on by default**, which is
  itself undocumented, and its name promises more than it does: in
  `pkg/config/config.go` the flag's only effect is `dht.BucketSize(20)`. A
  second, unrelated `vpn.LowProfile` library option in `pkg/vpn/config.go`
  swaps in a bounded stream manager (`pkg/vpn/vpn.go`) and is *not* wired to the
  CLI flag. The difference needs writing up.
- **The ten `limit-*` flags** in `cmd/util.go`, which configure the libp2p
  resource manager: `--limit-enable` (off by default — the others do nothing
  until it is on), `--limit-file`, `--limit-scope`, `--limit-config-streams`,
  `--limit-config-streams-inbound`, `--limit-config-streams-outbound`,
  `--limit-config-conn`, `--limit-config-conn-inbound`,
  `--limit-config-conn-outbound` and `--limit-config-fd`. Their defaults
  (200/30/30, 200/30/30, 30) are not documented and their relationship to
  `--limit-scope` is not explained.
- **Connection water marks.** `--connection-high-water` and
  `--connection-low-water` (env `EDGEVPN_CONNECTION_HIGH_WATER` /
  `EDGEVPN_CONNECTION_LOW_WATER`) both default to `0`, and what `0` means is not
  stated anywhere. `--max-connections` (env `EDGEVPNMAXCONNS`) is a third knob
  in the same area.
- **What to turn off.** `--dht`, `--mdns`, `--natservice`, `--natmap`,
  `--autorelay` and `--relay-service` are all on by default and all cost
  something; `--relay-service=false` in particular stops the node carrying other
  peers' traffic. See [relays and hop nodes](../relays-and-hop-nodes/).

Contributions welcome — see [contributing](../../contributing/).
{{% /pageinfo %}}
