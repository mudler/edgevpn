---
title: "Use EdgeVPN as a library"
linkTitle: "Use as a library"
weight: 100
description: >
  Embed a node in your own Go program — join a network from a token, or bring up the VPN.
---

EdgeVPN can be used as a library. It is very portable and offers a functional
interface.

To join a node in a network from a token, without starting the vpn:

```golang
import (
    "github.com/ipfs/go-log"
    "github.com/mudler/edgevpn/pkg/discovery"
    "github.com/mudler/edgevpn/pkg/logger"
    node "github.com/mudler/edgevpn/pkg/node"
)

d := discovery.NewDHT()
m := &discovery.MDNS{}

e, err := node.New(
    node.Logger(logger.New(log.LevelInfo)),
    node.MaxMessageSize(2 << 20),
    node.FromBase64(mDNSEnabled, DHTEnabled, token, d, m),
    // ....
)
if err != nil {
    return err
}

if err := e.Start(ctx); err != nil {
    return err
}
```

`node.FromBase64` decodes the same base64 token the CLI uses (`edgevpn -g -b`)
and wires the discovery services into the node, which is why it takes the
`*discovery.DHT` and `*discovery.MDNS` values you built above. The two booleans
enable mDNS and DHT discovery respectively.

or to start a VPN:

```golang
import (
    node "github.com/mudler/edgevpn/pkg/node"
    vpn "github.com/mudler/edgevpn/pkg/vpn"
)

opts, err := vpn.Register(vpnOpts...)
if err != nil {
    return err
}

e, err := node.New(append(o, opts...)...)
if err != nil {
    return err
}

if err := e.Start(ctx); err != nil {
    return err
}
```

`vpn.Register` turns a set of `vpn.Option` values into node options — it
registers the VPN as a network service on the node, so there is no separate VPN
start call. `o` is your own `[]node.Option` slice, built the same way as in the
first example.

Bringing up the TUN interface needs `CAP_NET_ADMIN` and access to
`/dev/net/tun`, exactly as the `edgevpn` binary does.
