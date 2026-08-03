---
title: "Share a service between two hosts"
linkTitle: "Share a service"
weight: 30
description: >
  Walk through exposing a TCP service on one host and reaching it from another, with no VPN interface and no root.
---

In this tutorial you will take a TCP service running on one machine — we will
use an SSH server, but anything that speaks TCP works — and reach it from a
second machine somewhere else, over EdgeVPN.

Nothing here brings up a VPN interface, so you do not need root on either host.

You will need:

- Two hosts, **A** and **B**, that can both reach the internet. They do not
  need to see each other, and neither needs a public IP.
- `edgevpn` installed on both. See
  [your first network](../your-first-network/) if you have not installed it yet.
- Something listening on a TCP port on host A. We assume `sshd` on
  `127.0.0.1:22`.

If you already know the shape of this and just want the commands, read
[Tunnel TCP services](../../how-to/tunnel-tcp-services/) instead.

## 1. Generate a token

Do this once, on either host. The token *is* the network — anyone holding it
joins it, so treat it like a password.

```bash
$ edgevpn -g -b
b3RwOgogIGRodDoKICAgIGludGVydmFsOiA5MDAwCiAgICBrZXk6IDRPNk5aUUMyTzVRNzdKRlJJT1BCWDVWRUkzRUlKSFdECiAgICBsZW5ndGg6IDMyCiAgY3J5cHRvOgogICAgaW50ZXJ2YWw6IDkwMDAKICAgIGtleTogN1hTUUNZN0NaT0haVkxQR0VWTVFRTFZTWE5ORzNOUUgKICAgIGxlbmd0aDogMzIKcm9vbTogWUhmWXlkSUpJRlBieGZDbklLVlNmcGxFa3BhVFFzUk0KcmVuZGV6dm91czoga1hxc2VEcnNqbmFEbFJsclJCU2R0UHZGV0RPZGpXd0cKbWRuczogZ0NzelJqZk5XZEFPdHhubm1mZ3RlSWx6Zk1BRHRiZGEKbWF4X21lc3NhZ2Vfc2l6ZTogMjA5NzE1MjAK
```

Copy that string to both hosts and export it, so you do not have to repeat it
on every command:

```bash
# on host A, and again on host B
$ export EDGEVPNTOKEN=b3RwOgogIGRodDoK...
```

## 2. Expose the service on host A

`service-add` takes a unique name for the service and the address EdgeVPN
should connect to on your behalf:

```bash
$ edgevpn service-add "MyCoolService" "127.0.0.1:22"
 INFO           edgevpn  Copyright (C) 2021-2022 Ettore Di Giacinto
       This program comes with ABSOLUTELY NO WARRANTY.
       This is free software, and you are welcome to redistribute it
       under certain conditions.
 INFO   Version: ... commit: ...
 INFO   Starting EdgeVPN network
 INFO   Node ID: 12D3KooWRW4RXSMAh7CTRsTjX7iEjU6DEU8QKJZvFjSosv7zCCeZ
 INFO   Bootstrapping DHT
```

The name — `MyCoolService` here — is how host B will ask for this service. It
is announced on the shared ledger, so it must be unique within the network.

Leave this running. The process stays in the foreground and acts as the proxy
between the network and `127.0.0.1:22`.

## 3. Connect from host B

`service-connect` takes the same name, and a local address to bind:

```bash
$ edgevpn service-connect "MyCoolService" "127.0.0.1:9090"
 INFO           edgevpn  Copyright (C) 2021-2022 Ettore Di Giacinto
       This program comes with ABSOLUTELY NO WARRANTY.
       This is free software, and you are welcome to redistribute it
       under certain conditions.
 INFO   Version: ... commit: ...
 INFO   Starting EdgeVPN network
 INFO   Node ID: 12D3KooWEyoppNCUx8Yx1oQ4rALCcs2i1p1sc6Bqm2S8LzRvbAcM
 INFO   Bootstrapping DHT
```

Leave this running too. Host B now has a listener on `127.0.0.1:9090` that
forwards to `127.0.0.1:22` on host A.

Peers find each other over the DHT, which is not instant. It is normal for the
first connection to take a few minutes; on a bad network it can take longer.

## 4. Use it

In a third terminal, on host B:

```bash
$ ssh -p 9090 youruser@127.0.0.1
```

You are now logged into host A. Anything that speaks TCP to `127.0.0.1:9090`
on B reaches port 22 on A.

If the connection hangs, the two nodes have most likely not discovered each
other yet — wait, and check that both processes are still running and using the
same token.

## What you did

- Created a network with a token, with no server and no configuration to
  coordinate.
- Published a TCP endpoint on the ledger under a name, from host A.
- Bound a local port on host B that tunnels to it.

Neither host allocated a `tun` device or needed root, and neither needed to
know the other's address.

## Next

- [Tunnel TCP services](../../how-to/tunnel-tcp-services/) — the terse version
  of this, for when you come back to it.
- [Your first network](../your-first-network/) — the full VPN, where peers get
  virtual IPs and can reach each other directly.
