---
title: "Tunnel connections"
linkTitle: "Tunnelling"
weight: 1
description: >
  EdgeVPN network services for tunnelling TCP services
---

## Forwarding a local connection

EdgeVPN can also be used to expose local(or remote) services without establishing a VPN and allocating a local tun/tap device, similarly to `ngrok`.

### Exposing a service

If you are used to how Local SSH forwarding works (e.g. `ssh -L 9090:something:remote <my_node>`), EdgeVPN takes a similar approach.

A Service is a generalized TCP service running in a host (also outside the network). For example, let's say that we want to expose a SSH server inside a LAN.

To expose a service to your EdgeVPN network then:

```bash
$ edgevpn service-add "MyCoolService" "127.0.0.1:22"
```

To reach the service, EdgeVPN will setup a local port and bind to it, it will tunnel the traffic to the service over the VPN, for e.g. to bind locally to `9090`:

```bash
$ edgevpn service-connect "MyCoolService" "127.0.0.1:9090"
```

with the example above, 'sshing into `9090` locally would forward to `22`.
