---
title: "Run as a VPN"
linkTitle: "Run as a VPN"
weight: 10
aliases:
  - /docs/getting-started/cli/
description: >
  Join a network as a VPN peer and route traffic between nodes.
---

To start the VPN, simply run `edgevpn` without any argument.

An example of running edgevpn on multiple hosts:

```bash
# on Node A
$ EDGEVPNTOKEN=.. edgevpn --address 10.1.0.11/24
# on Node B
$ EDGEVPNTOKEN=.. edgevpn --address 10.1.0.12/24
# on Node C ...
$ EDGEVPNTOKEN=.. edgevpn --address 10.1.0.13/24
...
```

... and that's it! the `--address` is a _virtual_ unique IP for each node, and it is actually the ip where the node will be reachable to from the vpn. You can assign IPs freely to the nodes of the network, while you can override the default `edgevpn0` interface with `IFACE` (or `--interface`)

*Note*: It might take up time to build the connection between nodes. Wait at least 5 mins, it depends on the network behind the hosts.

For how addresses are handed out, how to let peers negotiate them among
themselves, and how to pin a static routing table, see
[Addressing and DHCP](../addressing-and-dhcp/). For IPv6, see
[IPv6](../ipv6/).

## Generate a network token

EdgeVPN works by generating tokens (or network configuration files) that are shared between different machines.

Every token is unique and identifies the network itself: there is no central server setup, and no IP address is specified in config files.

To generate a new network token, just run `edgevpn -g -b`:

```bash
$ edgevpn -g -b
b3RwOgogIGRodDoKICAgIGludGVydmFsOiA5MDAwCiAgICBrZXk6IDRPNk5aUUMyTzVRNzdKRlJJT1BCWDVWRUkzRUlKSFdECiAgICBsZW5ndGg6IDMyCiAgY3J5cHRvOgogICAgaW50ZXJ2YWw6IDkwMDAKICAgIGtleTogN1hTUUNZN0NaT0haVkxQR0VWTVFRTFZTWE5ORzNOUUgKICAgIGxlbmd0aDogMzIKcm9vbTogWUhmWXlkSUpJRlBieGZDbklLVlNmcGxFa3BhVFFzUk0KcmVuZGV6dm91czoga1hxc2VEcnNqbmFEbFJsclJCU2R0UHZGV0RPZGpXd0cKbWRuczogZ0NzelJqZk5XZEFPdHhubm1mZ3RlSWx6Zk1BRHRiZGEKbWF4X21lc3NhZ2Vfc2l6ZTogMjA5NzE1MjAK
```

The fields of that configuration are documented in
[Network configuration and tokens](../../reference/network-config/).

A network token needs to be specified for all later interactions with edgevpn, in order to connect and establish a network connection between peers.

For example, to start `edgevpn` in API mode:

```bash
$ edgevpn api --token <token> # or alternatively using $EDGEVPNTOKEN
 INFO           edgevpn  Copyright (C) 2021-2022 Ettore Di Giacinto
       This program comes with ABSOLUTELY NO WARRANTY.
       This is free software, and you are welcome to redistribute it
       under certain conditions.
 INFO   Starting EdgeVPN network
 INFO   Node ID: 12D3KooWRW4RXSMAh7CTRsTjX7iEjU6DEU8QKJZvFjSosv7zCCeZ
 INFO   Node Addresses: [/ip6/::1/tcp/38637 /ip4/192.168.1.234/tcp/41607 /ip4/127.0.0.1/tcp/41607]
 INFO   Bootstrapping DHT
⇨ http server started on [::]:8080
```

Alternatively a network configuration file can be specified with `--config` or `EDGEVPNCONFIG`. 

As the token is a network configuration file encoded in base64, using a token or a config is equivalent:

```bash
$ EDGEVPNTOKEN=$(edgevpn -g | tee config.yaml | base64 -w0)
```

## API

While starting in VPN mode, it is possible _also_ to start in API mode by
specifying `--api`. See [WebUI and API](../../reference/api/).
