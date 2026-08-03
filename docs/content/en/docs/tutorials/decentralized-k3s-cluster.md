---
title: "A network-decentralized k3s test cluster"
linkTitle: "Decentralized k3s cluster"
weight: 40
description: >
  Run a multi-node k3s development cluster across machines that are only reachable behind NAT.
---

Let's see a practical example, you are developing something for kubernetes and
you want to try a multi-node setup, but you have machines available that are
only behind NAT (pity!) and you would really like to leverage HW.

If you are not really interested in network performance (again, that's for
development purposes only!) then you could use `edgevpn` +
[k3s](https://github.com/k3s-io/k3s) in this way:

1) Generate edgevpn config: `edgevpn -g > vpn.yaml`
2) Start the vpn:

   on node A: `sudo IFACE=edgevpn0 ADDRESS=10.1.0.3/24 EDGEVPNCONFIG=vpn.yaml edgevpn`

   on node B: `sudo IFACE=edgevpn0 ADDRESS=10.1.0.4/24 EDGEVPNCONFIG=vpn.yaml edgevpn`
3) Start k3s:

   on node A: `k3s server --flannel-iface=edgevpn0`

   on node B: `K3S_URL=https://10.1.0.3:6443 K3S_TOKEN=xx k3s agent --flannel-iface=edgevpn0 --node-ip 10.1.0.4`

We have used flannel here, but other CNI should work as well.

{{% pageinfo color="warning" %}}
This is a development setup. The p2p network is chatty and is not built for
low-latency workloads — see
[when not to use EdgeVPN](../../explanation/when-not-to-use-edgevpn/) before
pointing anything important at it.
{{% /pageinfo %}}

For a managed, batteries-included version of the same idea, see
[Kairos](https://github.com/kairos-io/kairos), which creates Kubernetes
clusters with k3s automatically using EdgeVPN networks.
