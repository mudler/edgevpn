---
title: "IPv6"
linkTitle: "IPv6"
weight: 30
description: >
  Run the VPN over IPv6 with static addresses. Experimental, single-stack only.
---

{{% pageinfo color="warning"%}}
Experimental feature. IPv6 support is provisional and has known gaps — see
below before relying on it.
{{% /pageinfo %}}

IPv6 works with static addresses only. One address per interface; dual stack is
not supported, so a node is either IPv4 or IPv6, not both.

```bash
$ EDGEVPNTOKEN=.. edgevpn --address fd:ed4e::11/64 --mtu 1500
```

Two things to get right:

- **The address must be static.** `--dhcp` allocates IPv4 addresses only, so it
  cannot be combined with an IPv6 `--address`.
- **`--mtu` must be above 1280**, the IPv6 minimum link MTU. EdgeVPN's default
  is `1200`, which is below it, so you have to set `--mtu` explicitly.

Tracking issue [#15](https://github.com/mudler/edgevpn/issues/15) is still open
at the time of writing; it is the place to check for the current state of IPv6
support.
