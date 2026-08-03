---
title: "Send and receive files"
linkTitle: "Send and receive files"
weight: 50
aliases:
  - /docs/concepts/overview/files/
description: >
  Transfer files directly between peers, without bringing up a VPN interface.
---


## Sending and receiving files

EdgeVPN can be used to send and receive files between hosts via p2p with the  `file-send` and `file-receive` subcommand.

Sending and receiving files, as services, don't establish a VPN connection.

### Sending

```bash
$ edgevpn file-send --name unique-id --path /src/path
```

### Receiving

```bash
$ edgevpn file-receive --name unique-id --path /dst/path
```
