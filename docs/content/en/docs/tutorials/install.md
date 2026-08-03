---
title: "Install EdgeVPN"
linkTitle: "Install"
weight: 10
description: >
  Every way to get the binary — the install script, release archives, Homebrew, the container image, and building from source.
---

EdgeVPN is a single statically compiled binary with no runtime dependencies.
Pick whichever route suits the machine; if you just want the shortest path to a
running network, take the [install script](#install-script) and carry on to
[your first network](../your-first-network/).

## Install script

The repository ships an installer at
[`install.sh`](https://github.com/mudler/edgevpn/blob/master/install.sh):

```bash
curl -sfL https://raw.githubusercontent.com/mudler/edgevpn/master/install.sh | sh
```

It is **Linux only** — it aborts on any other platform. What it does:

1. Detects the architecture (`x86_64`, `arm64`, `armv6`, `i386`) and looks up
   the latest release tag from the GitHub API.
2. Downloads the matching release archive and installs the `edgevpn` binary
   into `/usr/local/bin`, using `sudo` when not running as root. If
   `/usr/local/bin` is not writable **and** `/opt/bin` already exists, it uses
   `/opt/bin` instead; it will not create that directory.
3. Writes a service unit: a systemd **template** unit
   `/etc/systemd/system/edgevpn@.service` if systemd is present, or
   `/etc/init.d/edgevpn` under OpenRC.

The script installs the unit but does not enable or start anything, so nothing
runs until you ask it to.

Environment variables it honours:

| Variable | Effect |
|---|---|
| `VERSION` | Install a specific release tag instead of the latest |
| `INSTALL_BIN_DIR` | Where to put the binary (default `/usr/local/bin`) |
| `INSTALL_SYSTEMD_DIR` | Where to write the unit (default `/etc/systemd/system`) |
| `DOWNLOADER` | `curl` (default) or `wget` |
| `ARCH`, `OS` | Override the detected architecture and platform |

For example:

```bash
curl -sfL https://raw.githubusercontent.com/mudler/edgevpn/master/install.sh | VERSION=v0.35.3 sh
```

Setting `VERSION` yourself is also the workaround when the GitHub API is
unavailable or rate-limits you: the script's version lookup has no working
fallback, so a failed lookup leaves `VERSION` empty, the download URL is
malformed, and the install aborts on the failed download rather than
installing anything wrong.

{{% pageinfo color="warning" %}}
The installer does **not** verify a checksum or signature on the download — the
script has a `TODO` where that check belongs. If that matters to you, take the
[manual route](#release-archives) and check the archive against the
`edgevpn-<tag>-checksums.txt` file published with the release.
{{% /pageinfo %}}

### Running it as a service

The systemd unit is a template, so one installation can run several networks
side by side. Each instance reads its configuration from an environment file
named after the instance:

```bash
# /etc/systemd/system.conf.d/edgevpn-home.env
EDGEVPNTOKEN=<your token>
ADDRESS=10.1.0.11/24
IFACE=edgevpn0
```

```bash
sudo systemctl enable --now edgevpn@home
```

The unit raises `LimitNOFILE` to 49152 and applies
`sysctl -w net.core.rmem_max=2500000` before starting, which is the buffer-size
tuning described in [troubleshooting](../../troubleshooting/). Create the
`/etc/systemd/system.conf.d/` directory first if it does not exist — the
installer does not create it, and the unit will fail to start without the
environment file.

See the [environment variables reference](../../reference/environment-variables/)
for everything that can go in that file.

## Release archives

Every release publishes statically compiled archives on the
[releases page](https://github.com/mudler/edgevpn/releases), for Linux, macOS,
Windows and FreeBSD on `amd64`, `arm64`, `arm`, `386` and `riscv64`.

Archives are named `edgevpn-<tag>-<OS>-<arch>.tar.gz` — for example
`edgevpn-v0.35.3-Linux-x86_64.tar.gz`. A `edgevpn-<tag>-checksums.txt` file is
published alongside them.

```bash
tar xvf edgevpn-*-Linux-x86_64.tar.gz
sudo install -m 755 edgevpn /usr/local/bin/edgevpn
edgevpn --version
```

## Homebrew (macOS)

If you're using homebrew in MacOS, you can use the
[edgevpn formula](https://formulae.brew.sh/formula/edgevpn):

```bash
brew install edgevpn
```

## Container image

Images are published to `quay.io/mudler/edgevpn` for `linux/amd64`,
`linux/arm64` and `linux/arm`.

{{% pageinfo color="warning" %}}
**`latest` is not the latest release.** The image workflow
(`.github/workflows/images.yml`) pushes `:latest` on **every push to
`master`**, so that tag tracks the development branch. It only re-tags a
release as `latest` when the git tag matches `^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
— and every EdgeVPN tag is `v`-prefixed (`v0.35.3`), which never matches. Pin
to a version tag if you want a release.
{{% /pageinfo %}}

So the tags you can expect are:

| Tag | What it is |
|---|---|
| `v0.35.3` (a release tag) | The image built from that release |
| `<8-char commit SHA>` | The image built from that exact commit, on both master pushes and tag pushes |
| `latest` | Whatever `master` last pushed — a development build |

Bringing up the VPN interface from inside a container needs host networking,
the `NET_ADMIN` capability and the TUN device:

```bash
docker run --rm \
  --network host \
  --cap-add NET_ADMIN \
  --device /dev/net/tun:/dev/net/tun \
  -e EDGEVPNTOKEN=<your token> \
  quay.io/mudler/edgevpn:v0.35.3 --address 10.1.0.11/24
```

Sub-commands that do not create an interface — `edgevpn file-send`,
`edgevpn service-connect`, `edgevpn proxy` — need none of that and run in a
plain container.

### Docker Compose

There is a
[`docker-compose.yml`](https://github.com/mudler/edgevpn/blob/master/docker-compose.yml)
in the repository with the same settings plus a healthcheck. Using docker is
still experimental as setups can vary wildly, so you'll likely need to edit it
— at minimum the `EDGEVPNTOKEN` and the volume path, both marked `CHANGEME`.
Note that it pins `image: quay.io/mudler/edgevpn:latest`, which is the
development build described above.

```bash
git clone https://github.com/mudler/edgevpn
cd edgevpn
sudo docker compose up --detach
```

## Build from source

You need [Go](https://golang.org/) 1.26 (the version in `go.mod`), `make`, and
[Node.js](https://nodejs.org/) 20.19 or newer. The web interface is a React
application that is compiled and then embedded into the binary, so a JavaScript
toolchain is part of a full build.

```bash
git clone https://github.com/mudler/edgevpn
cd edgevpn
make build        # compiles the web interface, then the Go binary
```

`make build` compiles the web interface first and the Go binary afterwards.
Running `go build` on its own works only when `api/react-ui/dist` already
exists — the interface is embedded with `//go:embed`, so a missing directory is
a compile error rather than a warning. If you are working on the Go side only,
you can stub it out:

```bash
mkdir -p api/react-ui/dist && touch api/react-ui/dist/index.html
```

Building the **documentation site** is the one thing that needs more: Hugo and
Node/npm, both of which `docs/scripts/build.sh` sets up for you.

```bash
cd docs && make build   # one-off build into docs/public
cd docs && make serve   # live preview on http://localhost:1313
```

See [contributing](../../contributing/) for the rest of the development
workflow.
