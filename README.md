# :sailboat: EdgeVPN

Fully Decentralized. Immutable. Portable. Easy to use Statically compiled VPN

EdgeVPN uses libp2p to build an immutable trusted blockchain addressable p2p network.

It connect and creates a small blockchain between nodes. It keeps the routing table stored in the ledger, while connections are dynamically established via p2p.


## Screenshots

Connected machines             |  Blockchain index
:-------------------------:|:-------------------------:
![edgevpn1](https://user-images.githubusercontent.com/2420543/139510781-18bc083e-34db-4b9c-b9ea-6250979d61cc.png)|  ![edgevpn2](https://user-images.githubusercontent.com/2420543/139510466-b8518d78-57c9-4325-b365-6439a8d130c3.png)

## Usage

Generate a config, and send it over all the nodes you wish to connect:

```bash
edgevpn -g > config.yaml
```

Run edgevpn on multiple hosts:

```bash
# on Node A
EDGEVPNCONFIG=config.yaml IFACE=edgevpn0 ADDRESS=10.1.0.11/24 ./edgevpn
# on Node B
EDGEVPNCONFIG=config.yaml IFACE=edgevpn0 ADDRESS=10.1.0.12/24 ./edgevpn
# on Node C ...
EDGEVPNCONFIG=config.yaml IFACE=edgevpn0 ADDRESS=10.1.0.13/24 ./edgevpn
...
```

... and that's it! the `ADDRESS` is a _virtual_ unique IP for each node, and it is actually the ip where the node will be reachable to from the vpn, while `IFACE` is the interface name.

You can also encode the config in base64, and pass it to edgevpn with `EDGEVPNTOKEN` instead:

```bash
EDGEVPNTOKEN=$(edgevpn -g | base64 -w0)
IFACE=edgevpn0 ADDRESS=10.1.0.13/24 ./edgevpn
```

*Note*: It might take up time to build the connection between nodes. Wait at least 5 mins, it depends on the network behind the hosts.

## Web interface

To access the web interface, run 

```bash
edgevpn api
```

with the same `EDGEVPNCONFIG` or `EDGEVPNTOKEN`. It will connect to the network without routing any packet. 

By default edgevpn will listen on the `8080` port. See `edgevpn api --help` for the available options

### API endpoint

#### `/api/data`

Returns the latest available data stored in the blockchain

#### `/api/blockchain`

Returns the latest available blockchain

## Architecture

- Simple (KISS) interface to display network data from the blockchain
- p2p encryption between peers with libp2p
- randezvous points dynamically generated from OTP keys
- extra AES symmetric encryption on top. In case randezvous point is compromised
- blockchain is used as a sealed encrypted store for the routing table
- connections are created host to host

## Is it for me?

EdgeVPN makes VPN decentralization a first strong requirement. 

Its mainly use is for edge and low-end devices and especially for development.

The decentralized approach has few cons:

- The underlaying network is chatty. It uses a Gossip protocol for syncronizing the routing table and p2p. Every blockchain message is broadcasted to all peers, while the traffic is to the host only.
- Might be not suited for low latency workload.

Keep that in mind before using it for your prod networks!

But it has a strong pro: it just works everywhere libp2p works!

### Example use case: network-decentralized [k3s](https://github.com/k3s-io/k3s) test cluster

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

## As a library

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

## Credits

- The awesome [libp2p](https://github.com/libp2p) library
- [https://github.com/songgao/water](https://github.com/songgao/water) for tun/tap devices in golang
- [Room example](https://github.com/libp2p/go-libp2p/tree/master/examples/chat-with-rendezvous) (shamelessly parts are copied by)

## Disclaimers

I'm not a security expert, and this software didn't went through a full security audit, so don't use and rely it for sensible traffic! I did this mostly for fun while I was experimenting with libp2p. 

## LICENSE

GNU GPLv3.

```
edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.
```
