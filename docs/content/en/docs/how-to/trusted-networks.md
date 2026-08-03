---
title: "Trusted networks"
linkTitle: "Trusted networks"
weight: 90
aliases:
  - /docs/concepts/overview/peerguardian/
description: >
  Restrict a network to authorized peers with PeerGuardian and peergating.
---


{{% pageinfo color="warning"%}}
Experimental feature!
{{% /pageinfo %}}

Trust zones are the only mechanism in EdgeVPN that can exclude a peer holding a
valid network token *at runtime* — everything else, the token itself, ledger
ownership and relay ACLs, assumes every token holder is a full member. If your
membership is a fixed set of hosts, `--static-peertable` excludes token holders
too, without being experimental; see
[static peer tables](../../explanation/security-model/#static-peer-tables).

Read [the security model](../../explanation/security-model/) first either way.
It covers what peergating adds, what it deliberately does not add, and the
admission gap in the current implementation that you should weigh before
relying on it.

## Peerguardian

PeerGuardian is a mechanism to prevent unauthorized access to the network if tokens are leaked or either revoke network access.

In order to enable it, start edgevpn nodes adding the `--peerguard` flag.

```bash
edgevpn --peerguard
```

To turn on peer gating, specify also `--peergate`. 

Peerguardian and peergating has several options:

```
   --peerguard                                   Enable peerguard. (Experimental) [$PEERGUARD]
   --peergate                                    Enable peergating. (Experimental) [$PEERGATE]
   --peergate-autoclean                          Enable peergating autoclean. (Experimental) [$PEERGATE_AUTOCLEAN]
   --peergate-relaxed                            Enable peergating relaxation. (Experimental) [$PEERGATE_RELAXED]
   --peergate-auth value                         Peergate auth [$PEERGATE_AUTH]
   --peergate-interval value                     Peergater interval time (default: 120) [$EDGEVPNPEERGATEINTERVAL]
```

When the PeerGuardian and Peergater are enabled, a VPN node will only accepts blocks from authorized nodes.

Peerguardian is extensible to support different mechanisms of authentication, we will see below specific implementations.

## ECDSA auth

The ECDSA authentication mechanism is used to verify peers in the blockchain using ECDSA keys.

To generate a new ECDSA keypair use `edgevpn peergater ecdsa-genkey`:

```bash
$ edgevpn peergater ecdsa-genkey
Private key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1JSGNBZ0VCQkVJQkhUZnRSTVZSRmlvaWZrdllhZEE2NXVRQXlSZTJSZHM0MW1UTGZlNlRIT3FBTTdkZW9sak0KZXVPbTk2V0hacEpzNlJiVU1tL3BCWnZZcElSZ0UwZDJjdUdnQndZRks0RUVBQ09oZ1lrRGdZWUFCQUdVWStMNQptUzcvVWVoSjg0b3JieGo3ZmZUMHBYZ09MSzNZWEZLMWVrSTlEWnR6YnZWOUdwMHl6OTB3aVZxajdpMDFVRnhVCnRKbU1lWURIRzBTQkNuVWpDZ0FGT3ByUURpTXBFR2xYTmZ4LzIvdEVySDIzZDNwSytraFdJbUIza01QL2tRNEIKZzJmYnk2cXJpY1dHd3B4TXBXNWxKZVZXUGlkeWJmMSs0cVhPTWdQbmRnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
Public key: LS0tLS1CRUdJTiBFQyBQVUJMSUMgS0VZLS0tLS0KTUlHYk1CQUdCeXFHU000OUFnRUdCU3VCQkFBakE0R0dBQVFCbEdQaStaa3UvMUhvU2ZPS0syOFkrMzMwOUtWNApEaXl0MkZ4U3RYcENQUTJiYzI3MWZScWRNcy9kTUlsYW8rNHROVkJjVkxTWmpIbUF4eHRFZ1FwMUl3b0FCVHFhCjBBNGpLUkJwVnpYOGY5djdSS3g5dDNkNlN2cElWaUpnZDVERC81RU9BWU5uMjh1cXE0bkZoc0tjVEtWdVpTWGwKVmo0bmNtMzlmdUtsempJRDUzWT0KLS0tLS1FTkQgRUMgUFVCTElDIEtFWS0tLS0tCg==
```

For example, to add a ECDSA public key, use the API as such from a node which is already trusted by PeerGuardian:

```bash
$ curl -X PUT 'http://localhost:8080/api/ledger/trustzoneAuth/ecdsa_1/LS0tLS1CRUdJTiBFQyBQVUJMSUMgS0VZLS0tLS0KTUlHYk1CQUdCeXFHU000OUFnRUdCU3VCQkFBakE0R0dBQVFBL09TTjhsUU9Wa3FHOHNHbGJiellWamZkdVVvUAplMEpsWUVzOFAyU3o1TDlzVUtDYi9kQWkrVFVONXU0ZVk2REpGeU50dWZjK2p0THNVTTlPb0xXVnBXb0E0eEVDCk9VdDFmRVNaRzUxckc4MEdFVjBuQTlBRGFvOW1XK3p4dmkvQnd0ZFVvSTNjTDB0VTdlUGEvSGM4Z1FLMmVOdE0KeDdBSmNYcWpPNXZXWGxZZ2NkOD0KLS0tLS1FTkQgRUMgUFVCTElDIEtFWS0tLS0tCg=='
```

Now the private key can be used while starting new nodes:

```bash
PEERGATE_AUTH="{ 'ecdsa' : { 'private_key': 'LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1JSGNBZ0VCQkVJQkhUZnRSTVZSRmlvaWZrdllhZEE2NXVRQXlSZTJSZHM0MW1UTGZlNlRIT3FBTTdkZW9sak0KZXVPbTk2V0hacEpzNlJiVU1tL3BCWnZZcElSZ0UwZDJjdUdnQndZRks0RUVBQ09oZ1lrRGdZWUFCQUdVWStMNQptUzcvVWVoSjg0b3JieGo3ZmZUMHBYZ09MSzNZWEZLMWVrSTlEWnR6YnZWOUdwMHl6OTB3aVZxajdpMDFVRnhVCnRKbU1lWURIRzBTQkNuVWpDZ0FGT3ByUURpTXBFR2xYTmZ4LzIvdEVySDIzZDNwSytraFdJbUIza01QL2tRNEIKZzJmYnk2cXJpY1dHd3B4TXBXNWxKZVZXUGlkeWJmMSs0cVhPTWdQbmRnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=' } }"
$ edgevpn --peerguard --peergate
```

## Enabling/Disabling peergating in runtime

Peergating can be disabled in runtime by leveraging the api.

These routes are registered only on nodes started with `--peerguard`: on a node
without it they return `404`, because there is no peer gater to query.

### Query status

```bash
$ curl -X GET 'http://localhost:8080/api/peergate'
```

### Enable peergating
```bash
$ curl -X PUT 'http://localhost:8080/api/peergate/enable'
```

### Disable peergating
```bash
$ curl -X PUT 'http://localhost:8080/api/peergate/disable'
```

## Starting a new network

To init a new Trusted network, start nodes with `--peergate-relaxed` and add the neccessary auth keys:

```bash
$ edgevpn --peerguard --peergate --peergate-relaxed
$ curl -X PUT 'http://localhost:8080/api/ledger/trustzoneAuth/keytype_1/XXX'
```

{{% alert title="Note" %}}
It is strongly suggested to use a local store for the ledger with PeerGuardian (`--ledger-state <dir>`). In this way nodes persist auth keys locally and you can avoid starting nodes with `--peergate-relaxed`.
{{% /alert %}}
