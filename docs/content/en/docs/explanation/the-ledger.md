---
title: "The ledger"
linkTitle: "The ledger"
weight: 20
aliases:
  - /docs/concepts/overview/
description: >
  What the shared ledger stores, and what it deliberately does not.
---


EdgeVPN have a simplified model of a blockchain embedded. The model is actually simplified on purpose as the blockchain is used to store merely network and services metadata and not transaction, or content addressable network. 

The only data stored in the blockchain is:

- Network Peer IDs, Service IDs, File IDs
- Healthchecks, DNS records and IP allocation

However, the ledger is freely accessible via API, allowing for external coordination to use the blockchain mechanism as a shared memory access (which can be optionally persisted on disk).

Writes to the ledger are authenticated by default: each entry is signed by the
peer that wrote it, and a peer cannot overwrite an entry owned by another live
peer. See [ledger ownership](../../how-to/ledger-ownership/) for how to operate
it — in particular before changing `--ownership` on a running network — and
[the authenticated ledger](../authenticated-ledger/) for how it works.
