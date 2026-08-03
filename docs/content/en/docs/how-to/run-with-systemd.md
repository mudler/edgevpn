---
title: "Run with systemd"
linkTitle: "Run with systemd"
weight: 130
description: >
  Placeholder — running EdgeVPN as a systemd service, including API socket activation. Not written yet.
---

{{% pageinfo color="warning" %}}
**This page has not been written.** There is no systemd guide on this site
beyond the short
[template-unit section of the install page](../../tutorials/install/#running-it-as-a-service),
which covers only the `edgevpn@.service` unit that `install.sh` drops in.

What is missing, and where the source is:

- **API socket activation.** `api/api.go` (`systemdSocketListener`) reads
  `LISTEN_PID` and `LISTEN_FDS` and, when `LISTEN_PID` matches the process and
  `LISTEN_FDS` is exactly `1`, adopts the already-bound listener on FD 3 instead
  of binding one itself. The socket's path, owner, group and mode are then
  entirely whatever the `.socket` unit declares — EdgeVPN deliberately does not
  chmod or unlink it. No example `.socket`/`.service` pair is documented
  anywhere.
- **`APILISTENUNIXMODE`.** Read by `unixSocketMode` in `api/api.go`, it sets the
  mode only on the path where EdgeVPN creates the socket itself
  (`--api-listen unix:///run/edgevpn.sock`). It defaults to `0660` and silently
  falls back to that default if the value is not valid octal. It has no effect
  under socket activation.
- **Hardening a unit.** `NET_ADMIN`, `/dev/net/tun`, `DynamicUser`, and which
  of the [environment variables](../../reference/environment-variables/) belong
  in an `EnvironmentFile` rather than the unit.

Contributions welcome — see [contributing](../../contributing/).
{{% /pageinfo %}}
