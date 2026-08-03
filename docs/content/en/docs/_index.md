---
title: "Documentation"
linkTitle: "Documentation"
weight: 20
menu:
  main:
    weight: 20
---

EdgeVPN uses libp2p to build private, decentralized networks that are accessed
with a shared secret (a *token*). There is no VPN server and no central
coordinator: every peer that holds the token joins the same network and
discovers the others over p2p.

A single statically compiled binary can:

- **Create a VPN** between peers. Each node takes its virtual address from
  `--address`; an experimental `--dhcp` mode lets peers negotiate addresses
  among themselves instead.
- **Serve DNS** for the network, if you enable it with `--dns`. It answers from
  the records peers announce on the shared ledger, and forwards anything it
  does not have to an upstream resolver.
- **Act as a reverse proxy**, exposing a TCP service to the network the way
  `ngrok` would, without bringing up a VPN interface.
- **Send files** directly between peers, again without a VPN interface.
- **Be used as a Go library**, so you can embed the same distributed ledger and
  p2p connectivity in your own program.

## Where to go next

- **[Tutorials](tutorials/)** — start here if you are new. End-to-end
  walkthroughs that get you from nothing to a working network.
- **[How-to guides](how-to/)** — task-oriented recipes: expose a service, proxy
  traffic through another peer, lock a network down.
- **[Reference](reference/)** — every command, flag, environment variable and
  API endpoint.
- **[Explanation](explanation/)** — how EdgeVPN works and why, including the
  security model you should read before deploying it.

There is also a web UI and HTTP API for inspecting a running network
([WebUI/API](reference/api/)), and an alpha
[desktop GUI](tools/desktop-gui/) for Linux.
