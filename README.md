<h1 align="center">
  <br>
	<img src="https://user-images.githubusercontent.com/2420543/144679248-1f6e4c10-a558-424c-b6f5-b3695269c906.png" width=128
         alt="logo"><br>
    EdgeVPN

<br>
</h1>

<h3 align="center">Create Decentralized private networks </h3>
<p align="center">
  <a href="https://opensource.org/licenses/">
    <img src="https://img.shields.io/badge/licence-GPL3-brightgreen"
         alt="license">
  </a>
  <a href="https://github.com/mudler/edgevpn/issues"><img src="https://img.shields.io/github/issues/mudler/edgevpn"></a>
  <img src="https://img.shields.io/badge/made%20with-Go-blue">
  <img src="https://goreportcard.com/badge/github.com/mudler/edgevpn" alt="go report card" />
</p>

<p align="center">
	 <br>
    Fully Decentralized. Immutable. Portable. Easy to use Statically compiled VPN and a reverse proxy over p2p.<br>
    <b>VPN</b> -  <b>Reverse Proxy</b> - <b>Send files securely over p2p</b> -  <b>Blockchain</b>
</p>


EdgeVPN uses libp2p to build private decentralized networks that can be accessed via shared secrets.

It can:

- **Create a VPN** :  Secure VPN between p2p peers
  - Automatically assign IPs to nodes
  - Embedded tiny DNS server to resolve internal/external IPs
  - Create trusted zones to restrict which peers may join (experimental: it does not currently stop a token holder from entering the zone — see the [security model](https://mudler.github.io/edgevpn/docs/explanation/security-model/))
  - For example, the [Kairos](https://github.com/kairos-io/kairos) CNCF project uses it as a layer for creating decentralized clusters with Kubernetes

- **Act as a reverse Proxy** : Share a tcp service like you would do with `ngrok`. EdgeVPN let expose TCP services to the p2p network nodes without establishing a VPN connection: creates reverse proxy and tunnels traffic into the p2p network.

- **Send files via p2p** : Send files over p2p between nodes without establishing a VPN connection.

- **Be used as a library**: Plug a distributed p2p ledger easily in your golang code! For example EdgeVPN powers [LocalAI](https://github.com/mudler/LocalAI)'s P2P features (you can learn more about it [here](https://localai.io/features/distribute/)).

See the [documentation](https://mudler.github.io/edgevpn).

# :camera: Screenshots

Dashboard (Dark mode)            |  Dashboard (Light mode)
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 00-12-16 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/163020448-8e9238c1-3b6d-435d-9b25-7729d8779ebd.png) | ![Screenshot 2021-10-31 at 23-03-26 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/163020460-e18c07d7-8426-4992-aab3-0b2fd90279ae.png)

DNS            |  Machine index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-03-44 EdgeVPN - Services index](https://user-images.githubusercontent.com/2420543/163020465-3d481da4-4912-445e-afc0-2614966dcadf.png) | ![Screenshot 2021-10-31 at 23-03-59 EdgeVPN - Files index](https://user-images.githubusercontent.com/2420543/163020462-7821a622-8c13-4971-8abe-9c5b6b491ae8.png)

Services            |  Blockchain index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-04-12 EdgeVPN - Users connected](https://user-images.githubusercontent.com/2420543/163021285-3c5a980d-2562-4c10-b266-7e99f19d8a87.png) | ![Screenshot 2021-10-31 at 23-04-20 EdgeVPN - Blockchain index](https://user-images.githubusercontent.com/2420543/163020457-77ef6e50-40a6-4e3b-83c4-a81db729bd7d.png)


# :new: GUI

A Desktop GUI application (alpha) for Linux is available [here](https://github.com/mudler/edgevpn-gui)

Dashboard            |  Connections index
:-------------------------:|:-------------------------:
![edgevpn-gui-2](https://user-images.githubusercontent.com/2420543/147854909-a223a7c1-5caa-4e90-b0ac-0ae04dc0949d.png) | ![edgevpn-3](https://user-images.githubusercontent.com/2420543/147854904-09d96991-8752-421a-a301-8f0bdd9d5542.png)
![edgevpn-gui](https://user-images.githubusercontent.com/2420543/147854907-1e4a4715-3181-4dc2-8bc0-d052b3bf46d3.png) | 

# Kubernetes 

Check out [Kairos](https://github.com/kairos-io/kairos) for seeing EdgeVPN in action with Kubernetes!

# :running: Installation

Download the precompiled static release in the [releases page](https://github.com/mudler/edgevpn/releases). You can either install it in your system or just run it.

Or install the latest release with the one-liner:

```bash
curl -sfL https://raw.githubusercontent.com/mudler/edgevpn/master/install.sh | sh
```

Every installation route — install script, release archives, Homebrew, the container image and building from source — is covered in [Install EdgeVPN](https://mudler.github.io/edgevpn/docs/tutorials/install/).

# :computer: Usage

EdgeVPN works by generating tokens (or a configuration file) that can be shared between different machines, hosts or peers to access to a decentralized secured network between them.

Every token is unique and identifies the network,  no central server setup, or specifying hosts ip is required.

To generate a config run:

```bash
# Generate a new config file and use it later as EDGEVPNCONFIG
$ edgevpn -g > config.yaml
```

OR to generate a portable token:

```bash
$ EDGEVPNTOKEN=$(edgevpn -g -b)
```

Note, tokens are config merely encoded in base64, so this is equivalent:

```bash
$ EDGEVPNTOKEN=$(edgevpn -g | tee config.yaml | base64 -w0)
```

All edgevpn commands implies that you either specify a `EDGEVPNTOKEN` (or `--token` as parameter) or a `EDGEVPNCONFIG` as this is the way for `edgevpn` to establish a network between the nodes. 

The configuration file is the network definition and allows you to connect over to your peers securely.

**Warning** Exposing this file or passing-it by is equivalent to give full control to the network.

## :satellite: As a VPN

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


# :question: Is it for me?

The decentralized approach is chatty and might not suit low-latency workloads, and this software has not been security audited. Read [when not to use EdgeVPN](https://mudler.github.io/edgevpn/docs/explanation/when-not-to-use-edgevpn/) before you rely on it.

# :question: Why? 

First of all it's my first experiment with libp2p. Second, I always wanted a more "open" `ngrok` alternative, but I always prefer to have "less infra" as possible to maintain. That's why building something like this on top of `libp2p` makes sense.

# :warning: Warning!

This software has not been through a full security audit — don't rely on it for sensitive traffic or production environments. The full caveats are in [when not to use EdgeVPN](https://mudler.github.io/edgevpn/docs/explanation/when-not-to-use-edgevpn/).

# :books: Examples

- [A network-decentralized k3s test cluster](https://mudler.github.io/edgevpn/docs/tutorials/decentralized-k3s-cluster/) — a multi-node Kubernetes development cluster across machines behind NAT.
- [Use EdgeVPN as a library](https://mudler.github.io/edgevpn/docs/how-to/use-as-a-library/) — embed a node in your own Go program.

# 🧑‍💻 Projects using EdgeVPN

- [Kairos](https://github.com/kairos-io/kairos) - creates Kubernetes clusters with K3s automatically using EdgeVPN networks


# 🐜 Contribution

You can improve this project by contributing in following ways:

- report bugs
- fix issues
- request features
- asking questions (just open an issue)

and any other way if not mentioned here.

# :notebook: Credits

- The awesome [libp2p](https://github.com/libp2p) library
- [https://github.com/songgao/water](https://github.com/songgao/water) for tun/tap devices in golang
- [Room example](https://github.com/libp2p/go-libp2p/tree/master/examples/chat-with-rendezvous) (shamelessly parts are copied by)
- Logo originally made by [Uniconlabs](https://www.flaticon.com/authors/uniconlabs) from [www.flaticon.com](https://www.flaticon.com/), modified by me

# :notebook: Troubleshooting

Bootstrap failures, receive-buffer warnings and poor network performance are covered in [Troubleshooting](https://mudler.github.io/edgevpn/docs/troubleshooting/).

# :notebook: TODO

- [x] VPN
- [x] Send and receive files via p2p
- [x] Expose remote/local services via p2p tunnelling
- [x] Store arbitrary data on the blockchain
- [x] Allow to persist blockchain on disk

# :notebook: LICENSE

Apache License v2.

```
edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.
```
