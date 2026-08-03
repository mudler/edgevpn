---
title: "The security model"
linkTitle: "Security model"
weight: 15
description: >
  What the network token protects, what it does not, and what every mechanism layered on top actually buys you.
---

{{% pageinfo color="warning" %}}
**EdgeVPN's security model is perimeter-only.** Anyone holding the network
token is a fully trusted member of the network. There is no per-peer
authorization on the data plane, and no audit trail of which peer did what.
The token *is* the security boundary.
{{% /pageinfo %}}

Everything else on this page — ledger ownership, trust zones, relay ACLs,
socket permissions — narrows *specific* abuses by someone who is already
inside, or protects the machine you are running on. None of it changes the
sentence above. Read this before you decide who gets a copy of your token,
because that decision is the security design.

EdgeVPN has also **not been through a security audit**. See
[when not to use EdgeVPN](../when-not-to-use-edgevpn/).

## The token is the boundary

A token is a base64-encoded YAML config — nothing more. It carries a gossip
room name, a DHT rendezvous string, an mDNS service tag, and two OTP secrets
(`otp.dht.key` and `otp.crypto.key`). You can decode one with `base64 -d` and
read it. The [network config reference](../../reference/network-config/)
describes every field.

There is no per-node credential, no enrollment step, no revocation list. A node
is a member because it has the token.

### What a leaked token grants

Someone who obtains your token can, with no further access:

- **Join the network.** Derive the rendezvous point, find peers on the public
  DHT, and join the gossip topic.
- **Read the entire ledger.** Every machine's IP and peer ID, every announced
  service, every DNS record, every file announcement.
- **Write to the ledger.** Claim any unclaimed DNS name, announce services and
  files, and — on a network running `--ownership off` — overwrite anyone's
  entries.
- **Join the VPN.** Announce a machine entry, get an IP over
  [DHCP](../../how-to/addressing-and-dhcp/), and exchange packets with every
  other node.
- **Use any egress node.** If a node on the network runs
  [HTTP egress](../../how-to/http-egress-and-proxy/), the intruder can proxy
  traffic through it. There is no way to allow one member and deny another.
- **Reach any tunnelled service.** Anything published with
  [`service-add`](../../how-to/tunnel-tcp-services/) is reachable by any member
  that can run `service-connect`.

Treat a token exactly as you would a private key with no passphrase. Store it
in a secrets manager, pass it via `EDGEVPNTOKEN` or a config file readable only
by root, and keep it out of shell history, CI logs and process listings.

## What the token *does* protect

The token is weak as an authorization mechanism and strong as a confidentiality
mechanism. Traffic between members is protected on three independent layers:

1. **libp2p transport encryption.** Every peer-to-peer connection is
   authenticated to the remote peer ID and encrypted by libp2p itself. VPN
   packets ride these streams; an on-path observer sees libp2p noise, not your
   frames.
2. **A rotating DHT rendezvous.** The rendezvous point announced to the public
   DHT is not the token's `rendezvous` string but `MD5(TOTP(otp.dht.key))`,
   recomputed on every interval (`pkg/discovery/dht.go`). Without the token you
   cannot compute the current rendezvous, so you cannot enumerate the network's
   members through the DHT. Setting `otp.dht.key` to an empty value falls back
   to the static `rendezvous` string, which is permanent and therefore
   greppable forever once observed.
3. **AES sealing of gossip messages.** Every message published to the gossip
   room is sealed with AES-GCM under `MD5(TOTP(otp.crypto.key))`
   (`pkg/node/connection.go`, `pkg/crypto/aes.go`). This is a second layer on
   top of libp2p's own encryption, so the ledger stays unreadable even to
   someone who joined the topic without the crypto key.

{{% alert title="A note on the sealing key" color="info" %}}
The seal key is the hex encoding of an MD5 digest — 32 ASCII characters, used
directly as a 32-byte AES-256 key. The cipher is AES-256, but the key material
behind it carries at most the 128 bits of an MD5 output. This is adequate
against an eavesdropper who does not have the token, which is what it is there
for; it is not a reason to relax how you handle the token.
{{% /alert %}}

## Rotation does not revoke

The OTP mechanism rotates *derived* values — the rendezvous point and the seal
key — on a fixed interval. It does not rotate the secrets they are derived
from. Those live in the token, unchanged, forever.

So if a token leaks, waiting for the next OTP tick achieves nothing: the holder
recomputes the new rendezvous and the new seal key just as every legitimate
node does. OTP rotation defends against someone who observed *one* rendezvous
point or *one* sealed message, not against someone who has the token.

`--key-otp-interval` (default `360` seconds) only sets the interval baked into
a token at the moment you generate it:

```bash
edgevpn --key-otp-interval 120 -g -b > token.txt
```

It has no effect on an existing network — the interval is read from the token,
so all nodes must use the same one.

**The only way to revoke a leaked token is to generate a new one and restart
every node with it.** There is no partial revocation, and a network with the
old token keeps working for anyone still holding it. Plan for that: if you
cannot reach every node to re-key it, you cannot recover from a leak.

Two mechanisms let you keep the token and still deny the leaker. If your
membership is a fixed set of hosts, [static peer tables](#static-peer-tables)
are the non-experimental option: each node accepts only the peer IDs you list.
If membership changes at runtime, a
[trust zone](#trust-zones-peerguardian-and-peergating) can withdraw a peer's
authorization while the network runs — at the cost of an experimental feature.
Neither is a substitute for re-keying: the leaked token still admits its holder
anywhere you have not applied one of them.

## Trust zones: PeerGuardian and peergating

{{% alert title="Experimental" color="warning" %}}
`--peerguard`, `--peergate`, `--peergate-autoclean` and `--peergate-relaxed`
are all marked *(Experimental)* in their own usage strings — that is, every
flag that switches the feature on or changes how it gates. (`--peergate-auth`
and `--peergate-interval` carry no such marker, but they only supply the key
material and the sync cadence for the same experimental machinery.) Do not
build a security posture on it. It is the right shape for admission control,
but the implementation has the gaps described below.
{{% /alert %}}

[Trust zones](../../how-to/trusted-networks/) add an admission-control layer
*inside* the token perimeter. The shape is:

- A node started with `--peergate-auth` holding an ECDSA P-521 private key
  signs a challenge and publishes it to the gossip room
  (`pkg/trustzone/authprovider/ecdsa/provider.go`).
- Nodes running `--peerguard` verify that signature against the public keys
  stored in the `trustzoneAuth` ledger bucket. On success, they write the
  sender's peer ID into the `trustzone` bucket
  (`pkg/trustzone/peerguardian.go`).
- Nodes running `--peergate` drop gossip messages from any peer that is not in
  `trustzone` (`pkg/trustzone/peergater.go`, applied in
  `pkg/node/connection.go`).

**What this adds:** a token holder without an authorized ECDSA key has its
ledger messages dropped by gated nodes. Because the VPN data plane only accepts
streams from peers present in the `machines` bucket, a peer whose announcements
never land cannot exchange VPN traffic with a gated node either. That is real,
and it is the only mechanism that can exclude a token holder *dynamically* —
authorization is added and withdrawn in the ledger at runtime.

It is not, however, the only mechanism that can exclude a token holder. If your
membership is a fixed set of hosts, [static peer tables](#static-peer-tables)
do it without any of the caveats below. Weigh both before reaching for an
experimental feature.

**What this does not add:**

- **It gates who may join, not what a member may do.** Once a peer is in the
  trust zone it is an ordinary full member with every capability listed under
  [what a leaked token grants](#what-a-leaked-token-grants).
- **The buckets it runs on are outside ledger ownership.** Both `trustzone`
  (the admitted peers) and `trustzoneAuth` (the public keys they are admitted
  against) are absent from the policy registry in `pkg/blockchain/policy.go`, so
  they take the zero policy — no owner, never expiring, and writable by
  anything whose ledger writes a node already accepts, including the
  [unauthenticated API](#the-api-is-unauthenticated) on *any* node. (A signing
  node does still sign what it writes there; what the zero policy skips is the
  *verification*, so a signature says who wrote an entry and nothing about
  whether they were entitled to.) Adding a
  trusted key is therefore an ordinary ledger write, which means passing a
  challenge is not the only way into the trust zone: a token holder can instead
  supply the key the challenge is checked against. Admission rests on a store
  with weaker integrity than the `machines` and `dns` entries it exists to
  protect. See [ledger buckets](../../reference/ledger-buckets/).
- **The challenge is a constant, and the signature is not bound to the sender.**
  The provider signs the fixed string `"challenge"`, and PeerGuardian admits
  `m.SenderID` whenever the attached signature verifies against a trusted public
  key. Nothing ties that signature to the peer that sent it, and the signed
  challenge travels over the same gossip room every token holder can already
  read. A token holder that has never held an authorized ECDSA key can
  therefore still end up inside the trust zone. Treat peergating as raising the
  cost of an attack, not as a boundary.
- **`--peergate-relaxed` gates nothing while `trustzone` is empty**, by design,
  so a network can bootstrap. During that window every token holder is
  admitted. Prefer a persistent ledger (`--ledger-state <dir>`) so authorized
  keys survive restarts and you can stop using relaxed mode.
- **`--peergate-autoclean` removes peers from the trust zone when they leave
  the gossip topic**, which means a transient disconnect costs a peer its
  admission until it re-authenticates.
- **Gating is per-node local policy.** A peer that has not enabled `--peergate`
  keeps accepting everything, and it is still on the same network.

## Ledger ownership: write integrity, not admission

`--ownership` (default `enforce`) makes every entry in a **registered** bucket
carry its author's peer ID, a version, a timestamp and an Ed25519 signature,
verified against the public key embedded in the peer ID. A live peer's entries
in those buckets cannot be overwritten or replayed by anyone else.

**"Registered" is the load-bearing word.** The registry in
`pkg/blockchain/policy.go` covers `machines`, `services`, `files`, `users`,
`egress`, `healthcheck` and `dns`. Every other bucket — `trustzone`,
`trustzoneAuth`, `dhcp`, and any bucket you invent through the API — takes the
zero policy: no owner, never expiring, and overwritable by any writer that
supplies a strictly higher version. Those entries are still signed; it is the
verification the zero policy skips, not the signing.
Enforcement says nothing about those, which matters most for the two the
[trust zone](#trust-zones-peerguardian-and-peergating) is built on. See
[ledger buckets](../../reference/ledger-buckets/) for the full list.

This constrains a malicious member; it does not keep one out. Identities are
free, so a token holder can still mint peer IDs and claim any *unclaimed* name
or address. See [ledger ownership](../../how-to/ledger-ownership/) for
operating it and [the authenticated ledger](../authenticated-ledger/) for the
design.

{{% alert title="Check your ownership mode on older releases" color="warning" %}}
On **every released version at the time of writing, up to and including
v0.35.3**, an unrecognised `--ownership` value — say `enabled` instead of
`enforce`, or any typo — falls through to `off`. The node starts normally, logs
nothing, and accepts unsigned writes from any token holder.

A fix that rejects an invalid mode at startup, before the node joins a network,
has landed but is not yet in a tagged release. Until you are running a build
that contains it, verify the exact spelling on every node.
{{% /alert %}}

## The API is unauthenticated

The [HTTP API](../../reference/api/) has no authentication, authorization or
CSRF protection on any route. Anything that can reach the listener can read the
whole ledger and write to it:

```bash
curl -X PUT 'http://localhost:8080/api/ledger/<bucket>/<key>/<value>'
```

Writes are gossiped to the rest of the network. An exposed API port is
therefore equivalent to handing out the token, with the extra property that the
writes are signed as *your* node.

- The API is **off by default** when running the VPN; `--api` turns it on.
- The default listener is `127.0.0.1:8080`. **Never bind it to a routable
  address.**
- Prefer a unix socket on any shared host:

  ```bash
  edgevpn --api --api-listen "unix:///run/edgevpn/api.sock"
  ```

  The socket is created mode `0660` (owner and group only), overridable with
  `APILISTENUNIXMODE`. An unparseable value falls back to `0660` rather than
  widening permissions. systemd socket activation is honoured, in which case
  the unit's own ownership and permissions apply.

Filesystem permissions are the only access control the API has. Anyone who can
open that socket controls the node.

## Relay ACLs protect your bandwidth

`--relay-service-network-only` (default **on**) restricts incoming circuit-v2
relay *reservations* to peers seen in the local ledger's alive bucket, so
strangers who found you through the public DHT cannot use you as a relay
(`pkg/config/relay_acl.go`). Two caveats worth knowing:

- Only the reservation step is gated. Once a peer holds a reservation, connects
  through it are permitted.
- The ACL stays fully open during a bootstrap window, and stays open
  indefinitely if the alive bucket is empty — for example if the aliveness
  service is disabled. A debug line is logged on each refresh in that case.

This is a resource-abuse control, not a network access control: everyone it
admits is already a token holder.

Note that `--whitelist` is **not** an access control despite the name. It
passes multiaddrs to the libp2p resource manager's allowlist, exempting them
from connection limits. It does not restrict who may connect.

## Static peer tables

`--static-peertable` is the one **non-experimental** way to exclude a peer that
holds a valid token. It takes `ip:peerid` pairs, and a node configured with it
accepts VPN streams and gossip messages only from the peer IDs on the list
(`pkg/vpn/vpn.go`, `pkg/node/connection.go`):

```bash
edgevpn --static-peertable 10.1.0.1:12D3KooW... \
        --static-peertable 10.1.0.2:12D3KooW...
```

When it is set it *replaces* the ledger lookup rather than adding to it. Inbound
streams are matched against the table instead of the `machines` bucket, and
outbound packets are routed only to addresses in the table. A peer that is not
on the list cannot reach that node's data plane whatever it announces to the
ledger — which is exactly the property you want against a leaked token.

The trade-offs are why it is not the general answer:

- **It is local policy.** Each node carries its own table, and a node without
  one still accepts every token holder.
- **It is static.** Adding or removing a peer means editing configuration and
  restarting nodes. There is no runtime revocation.
- **It replaces automatic addressing** for the peers it covers: you are
  maintaining the IP-to-peer-ID mapping by hand instead of letting
  [DHCP](../../how-to/addressing-and-dhcp/) assign it.

For a handful of fixed hosts this is a genuine boundary with no experimental
caveats. For membership that changes, trust zones are the only dynamic option,
with the gap described above.

## What EdgeVPN does not protect against

### A malicious member

This is the big one. A member with a valid token — or a compromised node on
your own network — can:

- Read every machine, service and DNS entry in the ledger.
- Claim unclaimed DNS names and route your internal name lookups wherever it
  likes, if you use [the DNS service](../../how-to/enable-dns/).
- Announce a service or file and have any member connect to it.
- Send traffic to every VPN IP on the network. EdgeVPN routes packets between
  members; it does not filter them. **Host firewalls on the `edgevpn0`
  interface are your only per-service access control**, and you should treat
  the VPN as a flat, hostile LAN.
- Run an egress node and proxy — or observe — other members' HTTP traffic.

Ledger ownership, trust zones and firewalls each shave a piece off this list.
Nothing removes it.

### Traffic analysis by an egress node

An [HTTP egress](../../how-to/http-egress-and-proxy/) node is a fully trusted
intermediary. Its operator sees every URL and header, and because only
unencrypted HTTP is proxied at all (there is no `CONNECT` handling, so HTTPS
does not work through it), it can read and modify request and response bodies
at will. Any token holder can use any egress, and egress selection is random
per request, so you cannot even predict which node saw a given request.

### A compromised or hostile bootstrap peer

With no `--discovery-bootstrap-peers` set, EdgeVPN bootstraps against the
**public IPFS DHT bootstrap nodes**. Those nodes learn that your peer ID exists
and see the addresses you dial from; anyone able to observe DHT queries for
your current rendezvous can enumerate the peer IDs and IP addresses of your
network's members. They cannot read gossip (sealed) or VPN traffic
(libp2p-encrypted), and they cannot join without the token — but membership is
metadata, and metadata leaks. If that matters, point
`--discovery-bootstrap-peers` at infrastructure you control, or disable the DHT
and rely on mDNS on a trusted LAN.

### After the fact: there is no audit trail

Nothing in EdgeVPN records who did what. Under ownership an entry names its
current author and last update time, but the default in-memory store keeps only
the current block (`pkg/blockchain/store_memory.go`) — previous values are
gone. Reads are never recorded at all: there is no log of who queried the
ledger, who resolved a DNS name, or whose traffic went through an egress. If
you need accountability, it has to come from somewhere else in your stack.

### Host compromise

The token, the persisted ledger state (`--ledger-state`) and any cached private
key (`--privkey-cache-dir`, default `~/.edgevpn`) sit on disk. Root on any node
means the token, which means the network. The default cache directory is shared
per user, so co-located EdgeVPN processes would load the same identity — which
is exactly why `--privkey-cache` is opt-in and wants a distinct directory per
process.

## A short checklist

- Treat the token as a root credential. Rotating it means restarting every
  node, so plan the distribution path before you need it.
- Leave `--ownership enforce` on, and confirm the exact spelling on every node
  until you are running a build that validates it — no released version does
  yet.
- Keep the API on loopback or, better, a unix socket. Never on a routable
  address.
- Firewall the `edgevpn0` interface per host. The VPN is a flat network.
- Leave `--relay-service-network-only` on unless you deliberately want to relay
  for strangers.
- Run an egress node only on a network whose members you trust with your plain
  HTTP traffic.
- If you need to exclude a token holder without re-keying: on a fixed set of
  hosts use [static peer tables](#static-peer-tables), which are not
  experimental. Only if membership changes at runtime reach for trust zones,
  and weigh the admission gap described above first.

## See also

- [When not to use EdgeVPN](../when-not-to-use-edgevpn/) — the honest list of
  what the design costs you.
- [The ledger](../the-ledger/) and
  [the authenticated ledger](../authenticated-ledger/).
- [Ledger ownership](../../how-to/ledger-ownership/) — operating `--ownership`.
- [Trusted networks](../../how-to/trusted-networks/) — configuring PeerGuardian
  and peergating.
- [WebUI and API](../../reference/api/) — every route, and how to bind the
  listener safely.
