---
title: "Overview"
linkTitle: "Overview"
weight: 1
description: >
  EdgeVPN overview
---

EdgeVPN have a simplified model of a blockchain embedded. The model is actually simplified on purpose as the blockchain is used to store merely network and services metadata and not transaction, or content addressable network. 

The only data stored in the blockchain is:

- Network Peer IDs, Service IDs, File IDs
- Healthchecks, DNS records and IP allocation

However, the ledger is freely accessible via API, allowing for external coordination to use the blockchain mechanism as a shared memory access (which can be optionally persisted on disk).
