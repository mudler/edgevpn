<h1 align="center">
  <br>
 :sailboat: EdgeVPN
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
    Fully Decentralized. Immutable. Portable. Easy to use Statically compiled VPN and a reverse proxy over p2p.
</p>




# :camera: Screenshots

Dashboard            |  Machine index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 00-12-16 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/139602703-f04ac4cb-b949-498c-a23a-0ce8deb036f9.png) | ![Screenshot 2021-10-31 at 23-03-26 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/139602704-15bd342f-2db2-4a3b-b1c7-4dc7be27c0f4.png)

Services            |  File index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-03-44 EdgeVPN - Services index](https://user-images.githubusercontent.com/2420543/139602706-6050dfb7-2ef1-45b2-a768-a00b9de60ba1.png) | ![Screenshot 2021-10-31 at 23-03-59 EdgeVPN - Files index](https://user-images.githubusercontent.com/2420543/139602707-1d29f9b4-972c-490f-8015-067fbf5580f2.png)

Users            |  Blockchain index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-04-12 EdgeVPN - Users connected](https://user-images.githubusercontent.com/2420543/139602708-d102ae09-12f2-4c4c-bcc2-d8f4366355e0.png) | ![Screenshot 2021-10-31 at 23-04-20 EdgeVPN - Blockchain index](https://user-images.githubusercontent.com/2420543/139602709-244960bb-ea1d-413b-8c3e-8959133427ae.png)

EdgeVPN uses libp2p to build an immutable trusted blockchain addressable p2p network.

**VPN** Creates a vpn between p2p peers

**Reverse Proxy** You can now share a tcp service like you would do with `ngrok`. Expose services to the p2p network. Creates reverse proxy and tunnels traffic into the p2p network.

**Send files via p2p** Send files over p2p between nodes.

At implementation detail, EdgeVPN uses a blockchain to store *Services UUID*, *Files UUID*, *VPN Data* into the shared ledger.

It connect and creates a small blockchain between nodes. 

**The blockchain is ephemeral and on-memory**. Each node keeps broadcasting it's state until it is reconciled in the blockchain. If the blockchain would get start from scratch, the hosts would re-announce and try to fill the blockchain with their data.  

# :question: Why? 

First of all it's my first experiment with libp2p. Second, I always wanted a more "open" `ngrok` alternative, but I always prefer to have "less infra" as possible to maintain. That's why building something like this on top of `libp2p` makes sense.

# :warning: Warning!

I'm not a security expert, and this software didn't went through a full security audit, so don't use and rely it for sensible traffic and not even for production environment! I did this mostly for fun while I was experimenting with libp2p. 

# :running: Installation

Download the precompiled static release in the [releases page](https://github.com/mudler/edgevpn/releases). You can either install it in your system or just run it.

# :computer: Usage

EdgeVPN needs only a config, or a token to connect machines to a network.

To generate a config, do:

```bash
# Generate a new config file and use it later as EDGEVPNCONFIG
$ edgevpn -g > config.yaml
```

OR for a token:

```bash
$ EDGEVPNTOKEN=$(edgevpn -g -b)
```

The commands below emplies that you either specify a `EDGEVPNTOKEN` (or `--token` as parameter) or a `EDGEVPNCONFIG`. The configuration file is the network definition and allows you to connect over to your peers securely.

**Warning** Exposing this file or passing-it by is equivalent to give full control to the network.

## :satellite: As a VPN

Run edgevpn on multiple hosts:

```bash
# on Node A
$ EDGEVPNTOKEN=.. IFACE=edgevpn0 ADDRESS=10.1.0.11/24 edgevpn
# on Node B
$ EDGEVPNTOKEN=.. IFACE=edgevpn0 ADDRESS=10.1.0.12/24 edgevpn
# on Node C ...
$ EDGEVPNTOKEN=.. IFACE=edgevpn0 ADDRESS=10.1.0.13/24 edgevpn
...
```

... and that's it! the `ADDRESS` is a _virtual_ unique IP for each node, and it is actually the ip where the node will be reachable to from the vpn, while `IFACE` is the interface name.

*Note*: It might take up time to build the connection between nodes. Wait at least 5 mins, it depends on the network behind the hosts.

## :loop: Forwarding a local connection

EdgeVPN can also be used to expose local(or remote) services without establishing a VPN and allocating a local tun/tap device, similarly to `ngrok`.

### Exposing a service

If you are used to how Local SSH forwarding works (e.g. `ssh -L 9090:something:remote <my_node>`), EdgeVPN takes a similar approach.

A Service is a generalized TCP service running in a host (also outside the network). For example, let's say that we want to expose a SSH server inside a LAN.

To expose a service to your EdgeVPN network then:

```bash
$ edgevpn service-add --name "MyCoolService" --remoteaddress "127.0.0.1:22"
```

To reach the service, EdgeVPN will setup a local port and bind to it, it will tunnel the traffic to the service over the VPN, for e.g. to bind locally to `9090`:

```bash
$ edgevpn service-connect --name "MyCoolService" --srcaddress "127.0.0.1:9090"
```

with the example above, 'sshing into `9090` locally would forward to `22`.

## :globe_with_meridians: Web interface

To access the web interface, run 

```bash
$ edgevpn api
```

with the same `EDGEVPNCONFIG` or `EDGEVPNTOKEN`. It will connect to the network without routing any packet. 

By default edgevpn will listen on the `8080` port. See `edgevpn api --help` for the available options

## :mag: API endpoint

### GET

#### `/api/users`

Returns the users connected to services in the blockchain

#### `/api/services`

Returns the services running in the blockchain

#### `/api/machines`

Returns the machines connected to the VPN

#### `/api/blockchain`

Returns the latest available blockchain

#### `/api/ledger`

Returns the current data in the ledger

#### `/api/ledger/:bucket`

Returns the current data in the ledger inside the `:bucket`

#### `/api/ledger/:bucket/:key`

Returns the current data in the ledger inside the `:bucket` at given `:key`

### PUT

#### `/api/ledger/:bucket/:key/:value`

Puts `:value` in the ledger inside the `:bucket` at given `:key`

### DELETE

#### `/api/ledger/:bucket/:key`

Deletes the `:key` into `:bucket` inside the ledger

#### `/api/ledger/:bucket`

Deletes the `:bucket` from the ledger

## :mailbox: Sending and receiving files

### :outbox_tray: Sending

```bash
$ edgevpn file-send --name 'unique-id' --path '/src/path'
```

### :inbox_tray: Receiving

```bash
$ edgevpn file-receive --name 'unique-id' --path '/dst/path'
```

# Architecture

- Simple (KISS) interface to display network data from the blockchain
- p2p encryption between peers with libp2p
- randezvous points dynamically generated from OTP keys
- extra AES symmetric encryption on top. In case randezvous point is compromised
- blockchain is used as a sealed encrypted store for the routing table
- connections are created host to host

# :warning: Caveats

We might loose packets when the blockchain limit is reached. At that point EdgeVPN to avoid polluting memory will reset the blockchain, and the nodes will start to announce themselves again, there might be a small interval of time whereas node can't be reached. This _could_ happen if your network is having a lot of updates.

# :question: Is it for me?

EdgeVPN makes VPN decentralization a first strong requirement. 

Its mainly use is for edge and low-end devices and especially for development.

The decentralized approach has few cons:

- The underlaying network is chatty. It uses a Gossip protocol for syncronizing the routing table and p2p. Every blockchain message is broadcasted to all peers, while the traffic is to the host only.
- Might be not suited for low latency workload.

Keep that in mind before using it for your prod networks!

But it has a strong pro: it just works everywhere libp2p works!

## Example use case: network-decentralized [k3s](https://github.com/k3s-io/k3s) test cluster

Let's see a practical example, you are developing something for kubernetes and you want to try a multi-node setup, but you have machines available that are only behind NAT (pity!) and you would really like to leverage HW.

If you are not really interested in network performance (again, that's for development purposes only!) then you could use `edgevpn` + [k3s](https://github.com/k3s-io/k3s) in this way:

1) Generate edgevpn config: `edgevpn -g > vpn.yaml`
2) Start the vpn:

   on node A: `sudo IFACE=edgevpn0 ADDRESS=10.1.0.3/24 EDGEVPNCONFIG=vpn.yml edgevpn`
   
   on node B: `sudo IFACE=edgevpn0 ADDRESS=10.1.0.4/24 EDGEVPNCONFIG=vpm.yml edgevpn`
3) Start k3s:
 
   on node A: `k3s server --flannel-iface=edgevpn0`
   
   on node B: `K3S_URL=https://10.1.0.3:6443 K3S_TOKEN=xx k3s agent --flannel-iface=edgevpn0 --node-ip 10.1.0.4`

We have used flannel here, but other CNI should work as well.

# :notebook: As a library

EdgeVPN can be used as a library. It is very portable and offers a functional interface:

```golang

import (
    edgevpn "github.com/mudler/edgevpn/pkg/edgevpn"
)

e := edgevpn.New(edgevpn.Logger(l),
    edgevpn.LogLevel(log.LevelInfo),
    edgevpn.MaxMessageSize(2 << 20),
    edgevpn.WithMTU(1500),
    edgevpn.WithInterfaceMTU(1300),
    edgevpn.WithInterfaceAddress(os.Getenv("ADDRESS")),
    edgevpn.WithInterfaceName(os.Getenv("IFACE")),
    // ....
    edgevpn.WithInterfaceType(water.TAP))

e.Start()

```

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

# :notebook: LICENSE

GNU GPLv3.

```
edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.
```
