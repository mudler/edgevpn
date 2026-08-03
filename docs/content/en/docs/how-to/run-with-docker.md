---
title: "Run with Docker"
linkTitle: "Run with Docker"
weight: 110
description: >
  Run a VPN node in a container — why it needs host networking, NET_ADMIN and /dev/net/tun, and what the repository's compose file actually does.
---

The repository ships a
[`docker-compose.yml`](https://github.com/mudler/edgevpn/blob/master/docker-compose.yml)
that brings up a single VPN node. It is short, and every line in it is there for
a reason. This page explains those reasons, so you can adapt it instead of
copying it.

For the published image and its tags — in particular why `:latest` is a
development build and not the newest release — see
[install](../../tutorials/install/#container-image). Everything below assumes
you have picked a tag.

{{% pageinfo color="warning" %}}
Running the VPN in a container gives it the same reach as running it on the
host: with `network_mode: host` and `NET_ADMIN` it creates a systemwide network
interface and can reconfigure host networking. The container boundary is not
buying you isolation here.
{{% /pageinfo %}}

## The compose file, line by line

```yaml
services:
  edgevpn:
    image: quay.io/mudler/edgevpn:latest
    pull_policy: always
    container_name: edgevpn
    restart: unless-stopped
    volumes:
      - /home/CHANGEME/.edgevpn:/root/.edgevpn
    environment:
      - EDGEVPNTOKEN=CHANGEME
    network_mode: host
    devices:
      - /dev/net/tun:/dev/net/tun
    cap_add:
      - NET_ADMIN
    healthcheck:
      test: ["CMD", "sh", "-c", "ifconfig | grep -q edgevpn0"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

Two values are marked `CHANGEME` and the file does not work until you replace
both: the token, and the host path for the volume.

### `network_mode: host`

Required, and it is the setting people most often try to remove. EdgeVPN creates
a TUN interface (`edgevpn0` by default) inside whatever network namespace it is
running in. In a normal bridged container that namespace belongs to the
container alone: the interface comes up, the node joins the network, and nothing
on the host — or in any other container — can send a packet through it. Sharing
the host namespace is what makes `edgevpn0` a systemwide interface.

It has a second effect: mDNS peer discovery (`--mdns`, on by default) is
announced on the host's LAN interfaces rather than on a private bridge, so
local peers can find each other without going through the DHT.

The trade-off is that `--api-listen` and any port EdgeVPN binds land directly on
the host's ports. The API default (`127.0.0.1:8080`, only with `--api`) stays on
host loopback, which is what you want — see
[the API has no authentication](../../reference/api/#the-api-has-no-authentication).

### `devices: /dev/net/tun`

The TUN/TAP character device. Without it there is nothing to open, so the
interface cannot be created and bringing up the VPN fails. Passing the device is
not enough on its own — opening it and configuring the resulting interface also
needs the capability below.

### `cap_add: NET_ADMIN`

`CAP_NET_ADMIN` is what allows creating the TUN interface, assigning it an
address and bringing it up. Docker drops it from the default capability set, so
it has to be added back explicitly. It is the *only* extra capability needed —
if you find yourself reaching for `privileged: true`, you have a different
problem.

### `volumes: .../.edgevpn:/root/.edgevpn`

`/root/.edgevpn` is where EdgeVPN's state lives inside the container: the image
has no `USER`, so the process runs as root, and `--privkey-cache-dir` defaults
to `$HOME/.edgevpn`.

{{% pageinfo color="warning" %}}
**The volume alone does not persist anything.** `--privkey-cache` is off by
default, and the compose file leaves the line that enables it commented out. As
written, the container generates a fresh libp2p identity — a new peer ID — on
every restart, and the mounted directory stays empty.
{{% /pageinfo %}}

That matters because `--ownership` defaults to `enforce`, where a node's ledger
entries are tied to its identity. A node that changes peer ID on every restart
orphans its `machines`, `services` and `dns` entries each time; they are only
reclaimed once the previous identity's liveness window expires. EdgeVPN logs a
warning saying exactly this at startup.

To actually get a stable identity, uncomment the `entrypoint` line — or, more
simply, append the flags with `command:`, which keeps the image's own entrypoint:

```yaml
    command:
      - --address=10.1.0.11/24
      - --privkey-cache
      - --privkey-cache-dir=/root/.edgevpn
```

The image's `ENTRYPOINT` is `/usr/bin/edgevpn`, so `command:` entries are
appended to it as arguments. Give each node its own cache directory — two
processes sharing one would boot with the same peer ID.

### `environment: EDGEVPNTOKEN`

Every EdgeVPN flag has a matching environment variable, which is what makes the
container usable without an entrypoint override at all. `EDGEVPNTOKEN` is
`--token`; `ADDRESS` is `--address`; `EDGEVPNLOWPROFILE` is `--low-profile`, and
so on. The full mapping is the
[environment variables reference](../../reference/environment-variables/).
Flags win over environment variables when both are set.

Keep the token out of the compose file itself — put it in a `.env` file next to
it, or use a secrets mechanism. Anyone holding it is a full member of the
network.

### `healthcheck`

```yaml
test: ["CMD", "sh", "-c", "ifconfig | grep -q edgevpn0"]
```

`ifconfig` comes from busybox in the `alpine` base image, so the check needs no
extra tooling. It asks one question: does an interface whose name contains
`edgevpn0` exist? With `start_period: 40s` the container is given 40 seconds to
come up before failures count, then a failing check three times at 30-second
intervals marks it unhealthy.

Be clear about what this does and does not tell you:

- It does **not** check that the node has peers, that the ledger is syncing, or
  that traffic is flowing. An isolated node with a live interface is healthy by
  this definition.
- Because the container shares the host's network namespace, the check passes if
  *anything* on the host has created an `edgevpn0` interface — including a
  different EdgeVPN process, or a stale interface left behind by a previous run.
- `restart: unless-stopped` restarts the container when the process exits. Docker
  does not restart containers for being unhealthy, so an unhealthy-but-running
  node stays up until something acts on the status.

If you have `--api` enabled, `/api/summary` is a much better liveness signal —
it reports peer and machine counts, not just the presence of an interface.

### `pull_policy: always`

Combined with the pinned `:latest`, this pulls a new image on every `up`. Since
`:latest` tracks `master`, that means an unattended restart can move you onto a
different development build. Pin a release tag if you do not want that.

## Running it

```bash
docker compose up --detach
docker compose logs -f
```

Creating the interface needs root on the host (or membership of the `docker`
group, which is equivalent). Check the result from the host, not from inside the
container — with host networking they are the same namespace anyway:

```bash
ip addr show edgevpn0
```

## Running more than one node on a host

`network_mode: host` puts every container in the same network namespace, so a
second node using the defaults collides with the first on the `edgevpn0`
interface name. Give each one its own:

- `--interface` (env `IFACE`, default `edgevpn0`)
- `--address` (env `ADDRESS`, default `10.1.0.1/24`) — a distinct VPN address
- `--privkey-cache-dir`, if you enable `--privkey-cache`; two processes sharing
  one directory load the same key and appear on the network as one peer ID
- `--api-listen` (env `APILISTEN`, default `127.0.0.1:8080`), if you enable
  `--api` on more than one

The [systemd template unit](../../tutorials/install/#running-it-as-a-service)
that `install.sh` writes is built for exactly this shape, and is usually less
work.

## Sub-commands that need none of this

`edgevpn file-send`, `edgevpn file-receive`, `edgevpn service-add`,
`edgevpn service-connect` and `edgevpn proxy` never create a network interface.
They need no capability, no device and no host networking — just the token and,
for the ones that listen locally, a published port:

```bash
docker run --rm -e EDGEVPNTOKEN=<token> -p 127.0.0.1:9090:9090 \
  quay.io/mudler/edgevpn:v0.35.3 service-connect --name mysvc --address :9090
```
