
---
title: "Documentation"
linkTitle: "Documentation"
weight: 20
menu:
  main:
    weight: 20
---


EdgeVPN uses libp2p to build private decentralized networks that can be accessed via shared secrets.

It can:

- **Create a VPN** :  
  - Secure VPN between p2p peers
  - Automatically assign IPs to nodes
  - Embedded tiny DNS server to resolve internal/external IPs

- **Act as a reverse Proxy**
  - Share a tcp service like you would do with `ngrok` to the p2p network nodes without establishing a VPN connection

- **Send files via p2p**
  - Send files over p2p between nodes without establishing a VPN connection.

- **Be used as a library**
  - Plug a distributed p2p ledger easily in your golang code!

Check out the docs below for further example and reference, have a look at our [getting started guide]({{< relref "/docs">}}/getting-started), the [cli interface]({{< relref "/docs">}}/getting-started/cli), [gui desktop app]({{< relref "/docs">}}/getting-started/gui), and the embedde [WebUI/API]({{< relref "/docs">}}/getting-started/api/).


| WebUI            | [Desktop](https://github.com/mudler/edgevpn-gui)                                          |
| ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| ![img](https://user-images.githubusercontent.com/2420543/163020448-8e9238c1-3b6d-435d-9b25-7729d8779ebd.png) | ![](https://user-images.githubusercontent.com/2420543/147854909-a223a7c1-5caa-4e90-b0ac-0ae04dc0949d.png) |
