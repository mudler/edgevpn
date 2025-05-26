---
title: "Getting Started"
linkTitle: "Getting Started"
weight: 1
description: >
  First steps with EdgeVPN
---

## Get EdgeVPN  

Prerequisites: No dependencies. EdgeVPN releases are statically compiled.

### From release

Just grab a release from [the release page on GitHub](https://github.com/mudler/edgevpn/releases). The binaries are statically compiled.

### Via Homebrew on Macos

If you're using homebrew in MacOS, you can use the [edgevpn formula](https://formulae.brew.sh/formula/edgevpn)

```
brew install edgevpn
```


### Building EdgeVPN from source

Requirements:

- [Golang](https://golang.org/) installed in your system.
- make

```bash
$> git clone https://github.com/mudler/edgevpn
$> cd edgevpn
$> go build
```

### Using Docker Compose

Using docker is still experimental as setups can vary wildly.
An example [docker-compose.yml](https://github.com/mudler/edgevpn/blob/master/docker-compose.yml) file is provided for convenience but you'll likely need to edit it.

```bash
$> git clone https://github.com/mudler/edgevpn
$> cd edgevpn
$> sudo docker compose up --detach
```

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
