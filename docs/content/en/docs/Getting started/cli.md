---
title: "CLI"
linkTitle: "CLI"
weight: 1
description: >
  Command line interface
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

The VPN takes several options, below you will find a reference for the most important features:


## Generate a network token

EdgeVPN works by generating tokens (or network configuration files) that are shared between different machines.

Every token is unique and identifies the network itself: there is no central server setup, and no IP address is specified in config files.

To generate a new network token, just run `edgevpn -g -b`:

```bash
$ edgevpn -g -b
b3RwOgogIGRodDoKICAgIGludGVydmFsOiA5MDAwCiAgICBrZXk6IDRPNk5aUUMyTzVRNzdKRlJJT1BCWDVWRUkzRUlKSFdECiAgICBsZW5ndGg6IDMyCiAgY3J5cHRvOgogICAgaW50ZXJ2YWw6IDkwMDAKICAgIGtleTogN1hTUUNZN0NaT0haVkxQR0VWTVFRTFZTWE5ORzNOUUgKICAgIGxlbmd0aDogMzIKcm9vbTogWUhmWXlkSUpJRlBieGZDbklLVlNmcGxFa3BhVFFzUk0KcmVuZGV6dm91czoga1hxc2VEcnNqbmFEbFJsclJCU2R0UHZGV0RPZGpXd0cKbWRuczogZ0NzelJqZk5XZEFPdHhubm1mZ3RlSWx6Zk1BRHRiZGEKbWF4X21lc3NhZ2Vfc2l6ZTogMjA5NzE1MjAK
```

A network token needs to be specified for all later interactions with edgevpn, in order to connect and establish a network connection between peers.

For example, to start `edgevpn` in API mode:

```bash
$ edgevpn api --token <token> # or alternatively using $EDGEVPNTOKEN
 INFO           edgevpn  Copyright (C) 2021-2022 Ettore Di Giacinto
       This program comes with ABSOLUTELY NO WARRANTY.
       This is free software, and you are welcome to redistribute it
       under certain conditions.
 INFO  Version: v0.8.4 commit:
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

While starting in VPN mode, it is possible _also_ to start in API mode by specifying `--api`.

## DHCP

Note: Experimental feature!

Automatic IP negotiation is available since version `0.8.1`.

DHCP can be enabled with `--dhcp` and `--address` can be omitted. If an IP is specfied with `--address` it will be the default IP.

## IPv6 (experimental)

Node: Very experimental feature! Highly unstable!

Very provisional support for IPv6 is available using static addresses only. Currently only one address is supported per interface, dual stack is not available.
For more information, checkout [issue #15](https://github.com/mudler/edgevpn/issues/15)

IPv6 can be enabled with `--address fd:ed4e::<IP>/64` and `--mtu >1280`.
