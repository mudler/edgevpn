---
title: "Persist node identity and state"
linkTitle: "Persist node identity"
weight: 120
description: >
  Placeholder — keeping a node's peer ID and ledger across restarts. Not written yet.
---

{{% pageinfo color="warning" %}}
**This page has not been written.** By default an EdgeVPN node generates a fresh
libp2p key on every start, so its peer ID changes on every restart. Under
`--ownership enforce` (the default) that orphans everything the node had
announced. The flags that change this are documented only by their one-line
`--help` text.

What is missing, and where the source is:

- **`--privkey-cache`** (env `EDGEVPNPRIVKEYCACHE`, off by default, marked
  experimental) and **`--privkey-cache-dir`** (env `EDGEVPNPRIVKEYCACHEDIR`,
  defaulting to `$HOME/.edgevpn`). The implementation is in `cmd/util.go`: it
  reads or generates `<dir>/privkey`, writing the directory `0700` and the file
  `0600`. The comment above it explains why it is not enabled automatically —
  the default directory is per-user, so two co-located EdgeVPN processes sharing
  it would boot with the *same* peer ID. Each node needs its own directory.
- **The interaction with ownership.** `cmd/util.go` emits a warning when
  ownership enforcement is on and the identity is ephemeral, because the node's
  entries are reclaimed after the liveness TTL on every restart. See
  [ledger ownership](../ledger-ownership/).
- **`--ledger-state`** (env `EDGEVPNLEDGERSTATE`) is a different thing that gets
  confused with the above: it points the ledger at a `DiskStore`
  (`pkg/blockchain/store_disk.go`) instead of the default in-memory store. It
  persists the block chain, not the identity.
- **Backup, rotation and revocation** of a cached key: not addressed anywhere.

Contributions welcome — see [contributing](../../contributing/).
{{% /pageinfo %}}
