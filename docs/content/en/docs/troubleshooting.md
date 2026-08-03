---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
weight: 50
description: >
  Common bootstrap failures and poor network performance, and what to do about them.
---

## Slow bootstrap or poor network performance

If during bootstrap you see messages like:

```
edgevpn[3679]:             * [/ip4/104.131.131.82/tcp/4001] failed to negotiate stream multiplexer: context deadline exceeded
```

or

```
edgevpn[9971]: 2021/12/16 20:56:34 failed to sufficiently increase receive buffer size (was: 208 kiB, wanted: 2048 kiB, got: 416 kiB). See https://github.com/lucas-clemente/quic-go/wiki/UDP-Receive-Buffer-Size for details.
```

or generally experiencing poor network performance, it is recommended to
increase the maximum buffer size by running:

```
sysctl -w net.core.rmem_max=2500000
```

That setting does not survive a reboot. To make it permanent, drop it into
`/etc/sysctl.d/`:

```bash
echo "net.core.rmem_max=2500000" | sudo tee /etc/sysctl.d/99-edgevpn.conf
sudo sysctl --system
```

The systemd unit written by the [install script](../tutorials/install/) already applies
it on every start, via `ExecStartPre=-/bin/sh -c "sysctl -w
net.core.rmem_max=2500000"`.

## Nodes never see each other

It might take up time to build the connection between nodes. Wait at least
5 mins, it depends on the network behind the hosts.

If they still do not connect, check that every node is using the *same* token
or configuration file — a token identifies the network, and two different
tokens are two different networks that will never meet.
