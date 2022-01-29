---
title: "Sending and receiving files"
linkTitle: "File transfer"
weight: 20
date: 2017-01-05
description: >
  Send and receive files between p2p nodes
---

## Sending and receiving files

EdgeVPN can be used to send and receive files between hosts via p2p with the  `file-send` and `file-receive` subcommand.

Sending and receiving files, as services, don't establish a VPN connection.

### Sending

```bash
$ edgevpn file-send 'unique-id' '/src/path'
```

### Receiving

```bash
$ edgevpn file-receive 'unique-id' '/dst/path'
```
