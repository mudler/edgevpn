---
title: "HTTP egress and the proxy"
linkTitle: "HTTP egress and proxy"
weight: 70
description: >
  Let one node make HTTP requests on behalf of the network, and reach it through a local HTTP proxy.
---

{{% pageinfo color="warning"%}}
Only plain HTTP is proxied. HTTPS does not work — see
[HTTPS does not work](#https-does-not-work) below.
{{% /pageinfo %}}

EdgeVPN can designate one or more nodes as **HTTP egress** nodes. Another peer
runs `edgevpn proxy`, which exposes an ordinary local HTTP proxy. Requests sent
to that proxy travel over libp2p to one of the egress nodes, which performs the
request from its own network and streams the response back.

## What this is, and what it is not

This proxies **HTTP requests**, not arbitrary IP traffic. It is not a VPN exit
node: nothing is rerouted at the IP layer, your default route is untouched, and
only the clients you explicitly point at the local proxy are affected. If you
want a real network interface between peers, that is
[run as a VPN](../run-as-a-vpn/) instead.

The distinguishing property is on the client side: the machine running
`edgevpn proxy` needs no VPN interface and no privileges — it only joins the
network and listens on a local TCP port.

The egress side is different. `--egress` is a flag of the top-level `edgevpn`
command, which always brings up a TUN interface, so an egress node does need
privileges and a VPN address even though the egress feature itself never uses
the interface. Started unprivileged it fails with:

```
error while starting network service: 'ioctl: operation not permitted'
```

## Running an egress node

```bash
sudo edgevpn --egress --token "$TOKEN"
```

| Flag | Default | Environment | Description |
|---|---|---|---|
| `--egress` | off | `EGRESS` | Announce this node as an HTTP egress |
| `--egress-announce-time` | `200` | `EGRESSANNOUNCE` | Egress announce time, in seconds |

The node then advertises itself in the `egress` bucket of the
[ledger](../../explanation/the-ledger/) and serves the `/edgevpn/egress/0.1`
protocol. With the API enabled you can see the announcement on the node itself:

```bash
sudo edgevpn --egress --api --token "$TOKEN"   # API defaults to 127.0.0.1:8080
curl -s http://127.0.0.1:8080/api/ledger/egress
```

```json
{"12D3KooWMrvbf8SX1B6nj64mA2yJYiR1HBRcvX54KxocQ75g7knK":"\"ok\""}
```

Because the egress node runs the top-level command it also runs the aliveness
service automatically, which is what lets proxies tell whether it is still up.
Egress nodes get a VPN address like any other node, so give each one its own —
see [addressing and DHCP](../addressing-and-dhcp/).

## Using an egress node

On any other peer in the same network:

```bash
edgevpn proxy --listen :8080 --token "$TOKEN"
```

| Flag | Default | Environment | Description |
|---|---|---|---|
| `--listen` | `":8080"` | `PROXYLISTEN` | Address the local HTTP proxy listens on |
| `--interval` | `120` | `PROXYINTERVAL` | How often the proxy announces itself, in seconds |
| `--dead-interval` | `600` | `PROXYDEADINTERVAL` | Age, in seconds, after which an egress node is treated as offline |
| `--api` | `false` | `API` | Also start the API daemon and web UI |
| `--api-listen` | `"127.0.0.1:8081"` | `APILISTEN` | Address for the API, used only with `--api` |
| `--debug` | `false` | — | Start the API with `pprof` attached |

Then point a client at it like any other HTTP proxy:

```bash
http_proxy=http://localhost:8080 curl http://example.com/
```

or

```bash
curl -x http://localhost:8080 http://example.com/
```

Browsers should work the same way — set the HTTP proxy to `localhost:8080` in
the network settings — though only the two `curl` forms above have been tested,
and browsers default to HTTPS for most destinations, which
[does not work](#https-does-not-work).

The proxy also announces itself into the ledger's `users` bucket, and egress
nodes reject streams from peers that are not listed there. A freshly started
proxy therefore needs one announce round before its first request succeeds.

### Inspecting the proxy

Like the root command, `edgevpn proxy` only starts the API daemon and web UI if
you ask for it, and the API needs an address of its own — it cannot share
`--listen` with the proxy:

```bash
edgevpn proxy --listen :8080 --api --api-listen 127.0.0.1:8081 --token "$TOKEN"
```

Passing an `--api-listen` that would compete with `--listen` is refused at
startup rather than leaving one of the two servers silently dead. The two
addresses are resolved before being compared, so naming the same socket a
different way is caught too:

```console
$ edgevpn proxy --api --listen :8080 --api-listen :8080
--listen (":8080") and --api-listen (":8080") would bind the same address: give the API a different one

$ edgevpn proxy --api --listen localhost:8080 --api-listen 127.0.0.1:8080
--listen ("localhost:8080") and --api-listen ("127.0.0.1:8080") would bind the same address: give the API a different one
```

{{% alert title="Note" color="warning" %}}
`APILISTEN` is shared with the root command, where it defaults to
`127.0.0.1:8080`. If you export it globally, `edgevpn proxy --api` will refuse
to start, because that address collides with the proxy's own `--listen` default
of `:8080`. Pass `--api-listen` explicitly on the proxy, or unset the variable.
{{% /alert %}}

The API is a good way to check that an egress node has been seen:

```bash
curl -s http://127.0.0.1:8081/api/ledger/egress
```

## HTTPS does not work

There is no `CONNECT` handling anywhere in EdgeVPN. When a client asks the proxy
for an HTTPS URL it sends `CONNECT example.com:443`, and instead of opening a
tunnel the egress node forwards that request verbatim to the origin server,
which rejects it:

```console
$ curl -x http://localhost:8080 https://example.com/
curl: (56) CONNECT tunnel failed, response 400
```

The `400` comes from the destination server, not from EdgeVPN — the request does
reach it, it is simply not a request any web server will answer. Only plain HTTP
works today.

## Security

An HTTP egress node is a fully trusted intermediary, and this is the part to
think hardest about before enabling it.

- **The egress operator sees everything.** Requests leave the network from the
  egress node's own IP address, and that node's operator can observe every URL
  requested and every header sent. Because only unencrypted HTTP is proxied at
  all, they can also read and modify request and response bodies at will.
- **Destinations see the egress node, not you.** Whoever runs an egress is
  accepting responsibility for the traffic it emits — abuse reports, rate
  limits, and blocklists land on them.
- **Any token holder can use any egress.** EdgeVPN's trust model is
  perimeter-only: holding the network token makes a peer a full member, and
  there is no per-peer authorization for egress. You cannot allow one peer to
  proxy through an egress and deny another.

One thing the proxy does *not* do is expose the
[API](../../reference/api/) unless you ask: `--api` is off by default, because
the API writes to the ledger as the node running it and is unauthenticated.
Bind it to loopback if you enable it.

Read the [security model](../../explanation/security-model/) before running an
egress node on a network whose token is shared widely, and see
[trusted networks](../trusted-networks/) for narrowing who can join at all.

## How an egress is chosen

For every single request the proxy builds the list of nodes that appear in both
the `egress` bucket and the aliveness bucket with a healthcheck newer than
`--dead-interval`, then picks one of them uniformly at random.

Two consequences follow:

- **Requests are not pinned.** Consecutive requests from the same client may
  leave through different egress nodes, so anything that depends on a stable
  source address — session cookies tied to an IP, login flows, rate limits —
  can break when more than one egress node is running.
- **With no egress available the proxy answers `503`.** If no egress node has
  announced itself, or all of them went quiet longer ago than `--dead-interval`,
  the request fails immediately:

  ```console
  $ curl -x http://localhost:8080 http://example.com/
  no egress nodes available
  ```

Raising `--dead-interval` keeps egress nodes in the pool longer across brief
outages, at the cost of sending requests to nodes that have already gone away.
