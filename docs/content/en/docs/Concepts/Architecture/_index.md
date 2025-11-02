---
title: "Architecture"
linkTitle: "Architecture"
weight: 2
description: >
  EdgeVPN internal architecture
resources:
- src: "**edgevpn_*.png"
---
 
## Introduction

EdgeVPN uses [libp2p](https://github.com/libp2p/go-libp2p) to establish a decentralized, asymmetrically encrypted gossip network which propagate a (symmetrically encrypted) blockchain states between nodes.

The blockchain is lightweight as:
- There is no PoW mechanism
- It is in memory only, no DAG, CARv2, or GraphSync protocol - the usage is restricted to hold metadata, and not real addressable content

EdgeVPN uses the blockchain to store Services UUID, Files UUID, VPN and other metadata (such as DNS records, IP, etc.) and co-ordinate events between the nodes of the network. Besides, it is used as a mechanism of protection: if nodes are not part of the blockchain, they can't talk to each other.

The blockchain is ephemeral and on-memory, optionally can be stored on disk. 

Each node keeps broadcasting it's state until it is reconciled in the blockchain. If the blockchain would get start from scratch, the hosts would re-announce and try to fill the blockchain with their data.


- Simple (KISS) interface to display network data from the blockchain
- asymmetric p2p encryption between peers with libp2p
- randezvous points dynamically generated from OTP keys
- extra AES symmetric encryption on top. In case rendezvous point is compromised
- blockchain is used as a sealed encrypted store for the routing table
- connections are created host to host and encrypted asymmetrically

### Connection bootstrap

Network is bootstrapped with libp2p and is composed of 3 phases:

{{< imgproc edevpn_bootstrap.png Fit "1200x550" >}}
{{< /imgproc >}}

In the first phase, nodes do discover each others via DHT and a rendezvous secret which is automatically generated via OTP.

Once peers know about each other a gossip network is established, where the nodes exchange a blockchain over an p2p e2e encrypted channel. The blockchain is sealed with a symmetric key which is rotated via OTP that is shared between the nodes. 

At that point a blockchain and an API is established between the nodes, and optionally start the VPN binding on the tun/tap device.
