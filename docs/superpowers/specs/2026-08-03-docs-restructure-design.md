# EdgeVPN documentation restructure

**Date:** 2026-08-03
**Status:** Approved design, ready for implementation planning
**Scope:** Sub-project 2 (docs content restructure). Sub-project 3 (custom Hugo theme) remains deferred.

---

## 1. Programme context

Second of four sub-projects in the EdgeVPN overhaul.

| # | Sub-project | Status |
|---|---|---|
| 0 | Design system | done — `api/react-ui/src/styles/tokens.css` |
| 1 | React UI port | done — PR open, branch `feat/react-ui-design-system` |
| 2 | Docs restructure | **this spec** |
| 3 | Custom Hugo theme | deferred; consumes sub-project 0's tokens |

Sub-project 3 is deliberately excluded. A theme is easier to design once the
real page inventory exists, and the two touch different directories
(`docs/content/` vs `docs/layouts/` + `docs/assets/`).

---

## 2. The problem

The site is **16 pages, ~3,500 words**, for a project with **~90 CLI flags** and
**~75 environment variables**. That is not primarily a volume problem — it is
three specific failures:

1. **Actively wrong content.** Verified against the code, not inferred:

   | Defect | Evidence |
   |---|---|
   | `--peerguardian` does not exist | `cmd/util.go:368` defines `peerguard`. Four broken command lines in `Concepts/Overview/peerguardian.md` (lines 18, 21, 63, 91). urfave/cli hard-fails on an unknown flag. |
   | `edgevpn api --api-listen` is wrong | `Getting started/api.md:142`. The `api` subcommand takes `--listen`; `--api-listen` exists only on the root command. |
   | Egress / exit nodes / `edgevpn proxy` undocumented | `grep -ril egress docs/content/` → **0 files**, against 224 lines in `pkg/services/egress.go`. |
   | `edgevpn start` undocumented | `grep -rl 'edgevpn start' docs/content/` → **0 files**. It is the in-code recommended way to run a relay/hop node. |
   | Ledger ownership undocumented | `cmd/util.go` sets `Value: "enforce"` and its own usage string warns *"All nodes on a network must run the same mode/wire format, so flip the whole network together."* Explained only in `docs/design/authenticated-ledger.md`, which is outside `content/` and never ships. |

2. **An empty reference quadrant.** ~12 of ~90 flags documented; env vars
   nowhere. Hand-maintained flag tables are what let `--peerguardian` survive —
   a contributor once "fixed a typo" from `--peerguradian` to `--peerguardian`,
   i.e. into a flag that still does not exist.

3. **Broken information architecture.** `Concepts/Overview/` has an `_index.md`
   about the blockchain and four children (DNS, files, service tunnelling,
   PeerGuardian) that are how-tos, not concepts. Page weights collide —
   `Getting started/{_index,api,cli}.md` are all `weight: 1`; `Overview/files.md`
   and `Overview/dns.md` are both `weight: 20` — so ordering is arbitrary.

---

## 3. Goals and non-goals

### Goals

1. Replace the current tree with a Diátaxis information architecture.
2. Fix every verified factual defect in §2.1.
3. **Generate** the CLI and environment-variable reference from the `cli.App`,
   with a CI gate that fails on drift.
4. Write the P0 pages where a whole feature is currently invisible.
5. Publish `docs/design/authenticated-ledger.md`.
6. Make the site canonical: relocate the README's unique content, then trim it.

### Non-goals

- **The custom Hugo theme** (sub-project 3). Docsy stays for this round.
- **Rewriting prose that is already correct.** Existing good pages move; they
  are not re-authored.
- **The `urfave/cli/v3` phantom dependency** (`go.mod:32` declares it direct;
  no `.go` file imports it — the CLI is entirely v2). Separate PR.
- **`cmd/peergate.go`'s non-constant format string** (`go vet` failure at lines
  45 and 47, pre-existing at merge base). Separate PR.
- **The echo path-param unescape bug** affecting DNS-regex ledger deletes.
  Separate PR, documented as a known issue.
- **The three deprecation warnings** the build emits (`params.algolia_docsearch`,
  the GA4/UA notice, `params.ui.footer_about_disable`). The GA4 one goes away
  incidentally when §9 removes the placeholder analytics ID; the other two are
  Docsy configuration churn and belong with sub-project 3's theme work.
  - Note: an earlier report described three KaTeX math-parse *errors* where
    shell `$(...)`/`$VAR` in prose is read as math. Those do not occur on the
    pinned Hugo 0.152.2 — the build is clean. Stay alert for the pattern when
    adding new pages containing shell snippets, but there is nothing to fix.

---

## 4. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | **Diátaxis split** (tutorials / how-to / reference / explanation) | The current site's core failure is that everything is a half-tutorial-half-reference blob under "Concepts", and the reference quadrant is empty. Diátaxis names exactly that problem. |
| D2 | **Generate the CLI + env reference; gate CI on drift** | Rot, not absence, is the disease. A generator that CI enforces is the only option where the docs cannot silently diverge from the code. |
| D3 | **Custom generator, not `app.ToMarkdown()`** | `ToMarkdown` exists in v2.27.7 (`docs.go:19`) but emits one undifferentiated blob: no Hugo front matter, no per-command page splitting, no env-var column. A direct walk is ~120 lines and provides all three. |
| D4 | **Relocate README content, then trim** | The README duplicates ~60% of the site and has drifted, but uniquely holds the k3s walkthrough, `rmem_max` troubleshooting, and library usage. Relocation is not new writing; it ends the drift and makes the site canonical. |
| D5 | **Stub pages for known gaps, with honest notes** | An explicitly empty page beats silent absence: it tells the reader the gap is known and is a hook for contribution. No stub may imply content that does not exist. |
| D6 | **Docsy stays this round** | Changing IA and theme simultaneously makes review harder and couples two independent risks. |

---

## 5. Information architecture

```
docs/content/en/docs/
├─ _index.md                              "What is EdgeVPN"        [rewrite]
│
├─ tutorials/                                                       weight 10
│   ├─ install.md                         [README §Installation + install.sh]
│   ├─ your-first-network.md              [Getting started/_index.md, split]
│   ├─ share-a-service.md                 [Concepts/Overview/services.md, reframed]
│   └─ decentralized-k3s-cluster.md       [RELOCATED from README:153]
│
├─ how-to/                                                          weight 20
│   ├─ run-as-a-vpn.md                    [Getting started/cli.md]
│   ├─ addressing-and-dhcp.md             [cli.md §DHCP + --address/--router]
│   ├─ ipv6.md                            [cli.md §IPv6]
│   ├─ enable-dns.md                      [Concepts/Overview/dns.md]
│   ├─ send-and-receive-files.md          [Concepts/Overview/files.md]
│   ├─ tunnel-tcp-services.md             [Concepts/Overview/services.md]
│   ├─ exit-nodes-and-proxy.md            ★ NEW — --egress + `edgevpn proxy`
│   ├─ relays-and-hop-nodes.md            ★ NEW — `edgevpn start`, autorelay
│   ├─ trusted-networks.md                [peerguardian.md, --peerguard FIXED]
│   ├─ ledger-ownership.md                ★ NEW — operator half of the design doc
│   ├─ run-with-docker.md                 ★ NEW — docker-compose.yml explained
│   └─ use-as-a-library.md                [RELOCATED from README:174]
│
├─ reference/                                                       weight 30
│   ├─ cli/                               ⚙ GENERATED, one page per command
│   ├─ environment-variables.md           ⚙ GENERATED
│   ├─ network-config.md                  [Concepts/Token/_index.md]
│   ├─ api.md                             [Getting started/api.md, corrected]
│   ├─ ledger-buckets.md                  ★ NEW — pkg/protocol/protocol.go
│   └─ compatibility.md                   ★ NEW — ownership wire-format matrix
│
├─ explanation/                                                     weight 40
│   ├─ architecture.md                    [Concepts/Architecture/_index.md, updated]
│   ├─ the-ledger.md                      [Concepts/Overview/_index.md]
│   ├─ authenticated-ledger.md            ‡ PUBLISHED from docs/design/
│   ├─ security-model.md                  ★ NEW
│   └─ when-not-to-use-edgevpn.md         [README:130 "Is it for me?" + :149]
│
├─ tools/desktop-gui.md                   [Getting started/gui.md]
├─ troubleshooting.md                     [RELOCATED from README:240]  weight 50
└─ contributing.md                        [contribution-guidelines.md]  weight 60
```

**Weights are unique within each section.** The current collisions
(`Getting started/*` all at 1; `files.md`/`dns.md` both at 20) are a defect to
fix, not a pattern to carry forward.

**Redirects.** Every moved page gets a Hugo `aliases:` entry for its old URL.
The site is linked from the README, from Kairos, and from search results; moving
16 pages without aliases breaks all of it.

---

## 6. Generated reference

### Generator

New package `docs/generate/main.go`, mirroring the `api/generate` pattern the
repo used before this work removed it.

It imports `github.com/mudler/edgevpn/cmd`, walks:
- `cmd.MainFlags()` — root-only flags (18)
- `cmd.CommonFlags` — shared flags (68)
- each `*cli.Command` in `main.go`'s `Commands` slice, including subcommands

and emits into `docs/content/en/docs/reference/`:

- `cli/_index.md` — command index
- `cli/<command>.md` — one page per command: synopsis, aliases, description,
  and a flag table (name, type, default, env var, usage)
- `environment-variables.md` — every flag carrying `EnvVars`, as a table of
  env var ↔ flag ↔ default ↔ command scope

Every generated file carries Hugo front matter and a banner:

```
<!-- Generated by docs/generate. Do not edit; run `make docs-gen`. -->
```

### Drift gate

`make docs-gen` regenerates. A CI job runs it and then
`git diff --exit-code docs/content/en/docs/reference/`. Adding a flag without
regenerating fails the build.

**This is the highest-value item in the spec.** It closes ~90 flags and ~75 env
vars in one mechanism and makes the `--peerguardian` class of defect
structurally impossible.

### Known follow-up, not fixed here

The env-var naming is wildly inconsistent — bare (`API`, `DHCP`, `ROUTER`),
`EDGEVPN`-prefixed unseparated (`EDGEVPNTOKEN`, `EDGEVPNMTU`), and
`EDGEVPN_`-prefixed (`EDGEVPN_RELAY_SERVICE`). The generated table will make
this visible for the first time. Renaming is a breaking change and belongs in
its own issue; the table documents what exists.

Two env vars bypass the flag system entirely and must be documented **by hand**
on `reference/environment-variables.md`, since the generator cannot see them:
- `APILISTENUNIXMODE` (`api/api.go`) — octal mode for the API unix socket
- `LISTEN_PID` / `LISTEN_FDS` (`api/api.go`) — systemd socket activation

---

## 7. New prose

Eight pages are marked `★ NEW` in §5. Five are **P0** — the docs are actively
wrong or a whole feature is invisible without them — and are listed below in
priority order. The remaining three are **P1**: real gaps, written this round
if the P0 work lands cleanly, otherwise demoted to stubs under §7's stub policy
rather than silently dropped.

**P1 (new, lower priority):** `how-to/run-with-docker.md` (the repo ships a
`docker-compose.yml` that is linked but never explained — `network_mode: host`,
`NET_ADMIN`, `/dev/net/tun`, the healthcheck, `--privkey-cache`),
`reference/ledger-buckets.md` (the bucket namespace from
`pkg/protocol/protocol.go`, which the API docs already tell users to `PUT` into
without ever defining), and `tutorials/share-a-service.md` where it goes beyond
reframing the existing `services.md`.

### P0 — five pages, ordered by how much damage their absence does.

1. **`how-to/exit-nodes-and-proxy.md`** — an entire headline feature is
   invisible. Covers `--egress`, `--egress-announce-time`, the `edgevpn proxy`
   subcommand, and the security implication that traffic leaves the network at
   the exit node's address and any token holder can use it.

2. **`how-to/ledger-ownership.md`** + **`explanation/authenticated-ledger.md`** —
   `--ownership` defaults to `enforce` and is wire-format incompatible across
   modes. A user upgrading into a mixed-version network gets a silently broken
   ledger. The how-to is the operator half; the explanation is the existing
   369-line design doc, published as-is with front matter added.

3. **`explanation/security-model.md`** — EdgeVPN ships trust zones, peer gating,
   ownership enforcement, relay ACLs and an unauthenticated API, and nothing
   ties them together. Must state plainly: the security model is
   perimeter-only — any token holder is fully trusted, there is no per-peer
   authorization on the data plane, and the API has no authentication.

4. **`how-to/relays-and-hop-nodes.md`** — `edgevpn start`, autorelay flags, and
   the relay-service family (8 flags, the newest and best-commented code in the
   repo).

5. **`reference/compatibility.md`** — version/wire-format matrix, driven by the
   ownership modes.

### Stub policy (D5)

Gaps not written this round get a stub with real front matter and an explicit
note naming what is missing and pointing at the source. Permitted stubs:
`how-to/run-with-systemd.md`, `how-to/persist-node-identity.md`,
`how-to/tune-for-low-end-devices.md`, `explanation/discovery-and-nat.md`.

A stub must not imply content exists. No "coming soon" without saying what.

---

## 8. README

Relocate verbatim (git-mv-shaped, not new writing):

| README section | Destination |
|---|---|
| `:notebook: As a library` (line 174) | `how-to/use-as-a-library.md` |
| k3s example (line 153) | `tutorials/decentralized-k3s-cluster.md` |
| `:notebook: Troubleshooting` (line 240) | `troubleshooting.md` |
| `:question: Is it for me?` (130) + `:warning: Warning!` (149) | `explanation/when-not-to-use-edgevpn.md` |

The README then keeps: badges, the one-paragraph pitch, the feature list,
screenshots, installation, a 5-minute quickstart, projects-using-EdgeVPN,
contribution, credits, licence — and links to the site for everything else.

**Licence inconsistency, flagged not fixed:** `LICENSE` is Apache-2.0, the
README badge says GPL3, the README footer says "Apache License v2", the CLI
banner is GPL-flavoured, and ~10 source files carry GPL-2 headers. This is a
legal question for the maintainer, not a docs edit. The spec records it; the
implementation must not silently pick one.

---

## 9. Infrastructure

Small, in scope because they are docs-build correctness:

1. **Add `CONTRIBUTING.md`** at the repo root. `contribution-guidelines.md`
   links to it and it does not exist — the contributing page 404s today.
2. **Remove the `docsy` git submodule** and its `.gitmodules` entries.
   `docs/config.toml` sets no `theme=` (verified: 0 matches), so the submodule
   is unused; Dependabot bumps it regardless.
3. **Add `pull_request` + `paths: [docs/**]` to `.github/workflows/pages.yml`**,
   which currently triggers only on push to master — docs breakage is found
   only after merge.
4. **Reconcile `baseURL`** — `docs/config.toml` says
   `https://mudler.github.io/edgevpn/docs/` while `docs/scripts/build.sh`
   passes `-b https://mudler.github.io/edgevpn`. They disagree; local link
   prefixes differ from production.
5. **Set `breadcrumb_disable = false`** — the tree is three levels deep.
6. **Fix the empty link** in `Concepts/Token/_index.md` ("See [the Architecture
   section]()").
7. **Delete or fill `community/_index.md`** — an empty Docsy template shell
   currently in the main nav.
8. **Remove the placeholder `UA-00000000-0` analytics ID** while
   `params.ui.feedback.enable = true` sends feedback events nowhere.

Not in scope: `docs/package.json` and `package-lock.json` are both tracked
**and** gitignored, so the ignore rules are dead and `build.sh`'s
`npm install --save` dirties the tree on every docs build. Real, but it is a
build-tooling fix that belongs with sub-project 3's theme work.

---

## 10. Verification

- `cd docs && make build` succeeds, with **no new errors** relative to the
  merge base. Measured baseline on the pinned toolchain (Hugo 0.152.2
  extended, per `docs/Makefile`): **zero errors**, 29 pages, three deprecation
  warnings. An earlier report's "43 errors" figure came from Hugo 0.146.3, a
  version this project does not use.
- **Every command in every page is executed or explicitly marked unverified.**
  This is the discipline whose absence produced `--peerguardian`. A command
  that cannot be run in CI (needs two hosts, needs root) is marked as such in
  the implementation report, not silently trusted.
- `make docs-gen && git diff --exit-code docs/content/en/docs/reference/` is
  clean — the gate must pass on the branch that introduces it.
- No internal link 404s. Every moved page has a working `aliases:` entry.
- `grep -ril peerguardian docs/content/` returns nothing.
- `grep -ril egress docs/content/` returns at least the exit-nodes page.
- Every page has unique front matter `weight` within its section.

---

## Appendix A — Verified defect evidence

Confirmed by direct inspection on 2026-08-03, not taken from the audit report:

| Claim | Verification |
|---|---|
| `--peerguard` is the real flag | `cmd/util.go:368` `Name: "peerguard"` |
| egress undocumented | `grep -ril egress docs/content/` → 0 |
| `edgevpn start` undocumented | `grep -rl 'edgevpn start' docs/content/` → 0 |
| ownership defaults to enforce | `cmd/util.go` `Value: "enforce"` + wire-format warning in its usage string |
| bad api example | `Getting started/api.md:142` |
| design doc unpublished | `docs/design/authenticated-ledger.md` = 369 lines; 0 matches under `docs/content/` |
| `CONTRIBUTING.md` missing | absent from repo root |
| docsy submodule unused | 3 `.gitmodules` entries; 0 `^theme` lines in `docs/config.toml` |
| `ToMarkdown` exists but unsuitable | `urfave/cli/v2@v2.27.7/docs.go:19` |
| weight collisions | `Getting started/{_index,api,cli}.md` all `weight: 1`; `Overview/{files,dns}.md` both `weight: 20` |

## Appendix B — Deferred register

Carried forward, not addressed here:

1. `go.mod:32` — `urfave/cli/v3` declared direct, imported nowhere.
2. `cmd/peergate.go:45,47` — non-constant format string; fails `go vet`.
3. echo does not unescape path params, so ledger deletes silently no-op for
   DNS regexes containing `\ ^ $ /`. Pre-existing; `machines` unaffected.
4. Env-var naming inconsistency (three conventions).
5. Licence signalling inconsistency (§8).
6. `/api/metrics/peer/:peer` uses a raw `peer.ID()` cast, not `peer.Decode`.
7. Nodes co-hosting several peers of one network emit continuous
   `ownership violation (rejected): rollback to an older version` logs.
8. `docs/package.json` tracked and gitignored simultaneously.
