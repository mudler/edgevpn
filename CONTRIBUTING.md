# Contributing to EdgeVPN

Thanks for wanting to help. This page is the short version; the full
documentation lives at <https://mudler.github.io/edgevpn/docs/>.

## Build

EdgeVPN ships as a single binary, but the web interface is a React application
compiled into it, so you need Go 1.26 (the version in `go.mod`) **and** Node.js
20.19 or newer:

```bash
make build            # compiles the web interface, then the Go binary
make react-ui-force   # force a clean rebuild of the interface
```

`go build` on its own works only when `api/react-ui/dist` already exists — the
interface is embedded with `//go:embed`, so a missing directory is a compile
error. Working on the Go side only? Stub it:

```bash
mkdir -p api/react-ui/dist && touch api/react-ui/dist/index.html
```

Building the **documentation site** additionally needs Hugo and Node/npm —
`docs/scripts/build.sh` downloads the pinned Hugo (see `docs/Makefile`) and
installs `postcss-cli` and `autoprefixer` for you:

```bash
cd docs && make build   # one-off build into docs/public
cd docs && make serve   # live preview on http://localhost:1313
```

## Test

```bash
make test       # go test ./...
```

The end-to-end suites that CI runs (VPN connectivity, services, file transfer)
are the scripts under `.github/`; see `.github/workflows/test.yml` for how they
are invoked.

## Adding or changing a CLI flag

The CLI and environment-variable reference under
`docs/content/en/docs/reference/` is **generated** from the real `cli.App` — do
not hand-edit those pages. After touching any flag or command, run:

```bash
make docs-gen
```

and commit the result. The `Docs reference drift` workflow regenerates the
reference on every push and pull request and fails if the committed output
differs, so a forgotten `make docs-gen` will turn CI red.

## Issues and pull requests

- Open issues and feature requests at
  <https://github.com/mudler/edgevpn/issues>.
- Questions and general discussion belong in
  [GitHub Discussions](https://github.com/mudler/edgevpn/discussions) or the
  [Matrix room](https://matrix.to/#/#edgevpn:matrix.org).
- Pull requests go against `master`. Please make sure `make test` and
  `make docs-gen` are clean before asking for review, and mark work in progress
  with a draft PR or a `WIP` prefix.

Docs-only changes have their own walkthrough (including the *Edit this page*
shortcut) at
<https://mudler.github.io/edgevpn/docs/contribution-guidelines/>.

## License

EdgeVPN is Apache 2.0 licensed. By contributing you agree that your
contributions are licensed under the same terms.
