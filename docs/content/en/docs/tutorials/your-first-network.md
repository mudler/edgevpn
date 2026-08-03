---
title: "Your first network"
linkTitle: "Your first network"
weight: 20
aliases:
  - /docs/getting-started/
description: >
  Install EdgeVPN, generate a token, and bring up a two-node VPN.
---


## Get EdgeVPN  

Prerequisites: No dependencies. EdgeVPN releases are statically compiled.

On Linux, the quickest way is the install script:

```bash
curl -sfL https://raw.githubusercontent.com/mudler/edgevpn/master/install.sh | sh
```

Release archives, Homebrew, the container image and building from source are
all covered in [Install EdgeVPN](../install/).

## Creating Your First VPN

Let's create our first vpn now and start it:

```bash
$> EDGEVPNTOKEN=$(edgevpn -b -g)
$> edgevpn --dhcp --api
```

That's it!

You can now access the web interface on [http://localhost:8080](http://localhost:8080).

To join new nodes in the network, simply copy the `EDGEVPNTOKEN` and use it to start edgevpn in other nodes:

```bash
$> EDGEVPNTOKEN=<token_generated_before> edgevpn --dhcp
```
