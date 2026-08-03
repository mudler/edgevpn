# EdgeVPN React UI port + design system

**Date:** 2026-08-02
**Status:** Approved design, ready for implementation planning
**Scope:** Sub-projects 0 (design system) and 1 (React UI port)

---

## 1. Programme context

This spec covers the first two of four sub-projects agreed for the EdgeVPN
overhaul. The other two get their own spec → plan → implementation cycles.

| # | Sub-project | Touches | Depends on | Status |
|---|---|---|---|---|
| 0 | Design system | `api/react-ui/src/styles/` | — | **this spec** |
| 1 | React UI port | `api/`, build infra, CI | 0 | **this spec** |
| 2 | Docs restructure | `docs/content/` | — | deferred |
| 3 | Custom Hugo theme | `docs/layouts/`, `docs/assets/` | 0, easier after 2 | deferred |

They are separable because they touch disjoint directories. Only the design
tokens are shared, and they are copied into the Hugo theme in sub-project 3
rather than published as a package — an npm package shared between two static
consumers inside a Go repo is overhead for one variable set.

The visual direction was chosen from a three-way identity study: **Direction A
("Multiaddr")**, plus the peer-graph idea borrowed from Direction B.

---

## 2. Goals and non-goals

### Goals

1. Replace the Alpine.js + Tailwind-CDN UI with a React SPA built from source,
   embedded in the Go binary, with **no build artifacts committed to git**.
2. Establish the Direction A design system as CSS custom properties.
3. Reach **feature parity** with the current UI, plus a peer-graph view.
4. Remove the committed 2930-line generated `index.html` and the external CDN
   dependency that breaks air-gapped nodes.
5. Fix the polling behaviour, which currently issues requests for all six views
   continuously regardless of tab visibility.

### Non-goals

Explicitly out of scope, and not to be added opportunistically:

- Any **write/mutation** capability beyond what exists today (row deletes).
- **Authentication.** The API is unauthenticated by design; EdgeVPN's security
  model is perimeter-only. A login screen would have nothing behind it.
- **`json` struct tags** on Go types (see decision D3).
- **Reverse-proxy subpath support** (see decision D4).
- **SSE / WebSocket** transport. Polling stays, fixed rather than replaced.
- **`/api/version`.** Useful, but not parity. Deferred.
- New typed endpoints (`DELETE /api/machines/:address` etc.). The UI keeps
  using the generic ledger routes, as today.
- The two code bugs in Appendix B. Separate PRs.

---

## 3. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | **Mirror LocalAI's stack, but add TypeScript** | Vite + React 19 + react-router, hand-written CSS over tokens, hand-rolled `fetch` + `usePolling`. TypeScript because EdgeVPN's payloads are PascalCase (D3) and typed models make that survivable. |
| D2 | **Parity + polish only** | No new backend surface. Every view is already backed by a JSON endpoint. |
| D3 | **Keep PascalCase JSON; absorb casing in TypeScript** | Go types carry no `json` tags, so the API emits `PeerID`, `RateIn`, `BlockChain`. Adding tags silently breaks external consumers that unmarshal into their own structs — Kairos and LocalAI both build on `api/client`. |
| D4 | **Drop reverse-proxy subpath support** | LocalAI injects `<base href>` via `httpMiddleware.BaseURL`. EdgeVPN has no middleware package at all; porting one is not parity. Implementation note: Vite ships with `base: '/'`, not the `'./'` this decision originally assumed — a relative base makes the browser resolve `./assets/…` against the current deep link (`/app/nodes/assets/…`), which the SPA route answers with `index.html`, so every deep link loaded blank. Subpath support therefore needs an absolute base injected at serve time, not just a router change. |
| D5 | **BrowserRouter at `/app`, not hash routing** | Hash routing would need zero server changes, but deep links, asset caching and SPA fallback are all wanted anyway. `/` redirects to `/app`. |
| D6 | **Build infra lands before `go:embed`** | Not a preference — see §5. |
| D7 | **Peer graph v1 is an ego graph** | See §8. |

---

## 4. Design system (sub-project 0)

Lives at `api/react-ui/src/styles/tokens.css`, consumed by every component
through custom properties. No component references a literal colour.

### Palette

| Token | Value | Role |
|---|---|---|
| `--ev-bg` | `#14181D` | Ground. Blue-biased graphite, never neutral grey. |
| `--ev-panel` | `#1A2026` | Raised surface. |
| `--ev-rule` | `#2B333C` | Borders, dividers. |
| `--ev-ink` | `#DFE3E8` | Primary text. |
| `--ev-muted` | `#8A939E` | Secondary text. |
| `--ev-faint` | `#646D78` | Labels, captions. |
| `--ev-signal` | `#F2542D` | The single accent. Active path, current nav, focus. |
| `--ev-signal-2` | `#FFB35C` | Accent support. Used sparingly. |

Semantic state colours are **separate from the accent** and never substitute
for it:

| Token | Value | Role |
|---|---|---|
| `--ev-ok` | `#6FBF8B` | online, direct, verified |
| `--ev-warn` | `#FFB35C` | stale, relayed, degraded |
| `--ev-crit` | `#E5484D` | offline, error |

### Typography

System stacks only. No webfont CDN — the artifact sandbox blocks them and so
does an air-gapped EdgeVPN node. If a display face is self-hosted later it must
degrade to these chains.

- `--ev-mono`: `ui-monospace, "SF Mono", SFMono-Regular, "Cascadia Mono", Menlo, Consolas, "Liberation Mono", monospace`
- `--ev-sans`: `system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`

**Rule:** mono for the wordmark, navigation, code, tables, and every address,
hash, flag and numeric value. Sans is permitted **only** for body prose, and in
this sub-project that means effectively nowhere — it exists for the docs theme
in sub-project 3. Hierarchy comes from weight, size and tracking, not family.

All columns of digits get `font-variant-numeric: tabular-nums`.

### Other tokens

- Radius: `--ev-radius: 2px`. Deliberately not rounded-lg.
- A type scale (`--ev-step--1` … `--ev-step-3`) using `clamp()`.
- Spacing scale on a 4px base.

### Structural device

The forward slash, drawn from libp2p multiaddr notation
(`/ip4/10.1.0.4/tcp/4001/p2p/12D3KooW…`). Used as the separator in breadcrumbs,
nav and path-like values, always rendered in `--ev-signal`. It encodes real
hierarchy; it is not decoration.

### Mark

One SVG React component, `<Mark size={…} />`: two `--ev-muted` rings joined by a
`--ev-signal` bar — peer, edge, peer, with no centre. Three shapes, so it
survives 16px and a monochrome avatar. Ships as `favicon.svg` too.

### Theming

Dark-committed. The design is a single visual world, and it removes a whole
class of contrast bugs from a data-dense interface.

> **Accepted parity exception.** The current UI has a working light/dark toggle
> (`index.tmpl:56-59`, an Alpine boolean persisted to `localStorage`, applied as
> `class="dark"` on `<body>`). Direction A is dark-committed, so **the port drops
> the light theme and the toggle.** This is the only deliberate feature
> regression in this spec. If a light theme is required, it is real additional
> work — a second full palette designed for a dense interface, not an inversion
> — and it should be scoped explicitly rather than assumed.

---

## 5. Build infrastructure (must land first)

**`//go:embed react-ui/dist/*` is a compile error when `dist/` is absent.** The
moment that directive lands, every workflow that compiles Go breaks unless the
infra below already exists. This ordering is load-bearing.

Four workflows compile Go today:

| Workflow | How it builds | Fix |
|---|---|---|
| `.github/workflows/test.yml` | `go build` directly, plus a `test-suite` job | `actions/setup-node` + `make react-ui` in both jobs |
| `.github/workflows/build.yml` | `goreleaser build --snapshot` | goreleaser `before.hooks` |
| `.github/workflows/release.yml` | `goreleaser release` | goreleaser `before.hooks` |
| `.github/workflows/images.yml` | `docker build` | Dockerfile builder stage |

`.github/workflows/pages.yml` builds Hugo only and is unaffected.

### Steps, in order

1. **Scaffold `api/react-ui/`** with Vite, `package.json`, `vite.config.ts`,
   `tsconfig.json`, and a minimal app that builds to `dist/`.

2. **`Makefile`** — new; EdgeVPN has none today:

   ```make
   react-ui:
   ifneq ($(wildcard api/react-ui/dist),)
   	@echo "react-ui dist already exists, skipping build"
   else
   	cd api/react-ui && npm ci && npm run build
   endif

   react-ui-force:
   	rm -rf api/react-ui/dist
   	cd api/react-ui && npm ci && npm run build

   api/react-ui/dist: react-ui

   build: api/react-ui/dist
   	go build -o edgevpn
   ```

   The `ifneq` skip-guard mirrors LocalAI's. It is a known footgun — a stale
   `dist/` is silently reused — so the Makefile also gets a `react-ui-force`
   target that `rm -rf`s first, and CI uses that.

3. **`.gitignore`** — append. Note the existing `/dist` entry is rooted and does
   **not** match:

   ```
   /api/react-ui/dist
   /api/react-ui/node_modules
   ```

4. **`.goreleaser.yml`** — add before-hooks (the file currently has none):

   ```yaml
   before:
     hooks:
       - make react-ui-force
   ```

5. **`Dockerfile`** — add a node stage before the existing
   `golang:1.26-alpine` builder, and copy `dist` in before `go build`.

6. **CI stub for lint-only jobs** — where a job type-checks but does not need a
   real bundle, `mkdir -p api/react-ui/dist && touch api/react-ui/dist/index.html`
   satisfies the embed glob. EdgeVPN has no lint workflow today; this is
   documented so it is available if one is added.

7. **Only now** change `api/api.go:49`.

---

## 6. Server changes (`api/api.go`)

Current state: `//go:embed public` at line 49, and a bare
`ec.GET("/*", echo.WrapHandler(http.StripPrefix("/", assetHandler)))` at line
433 — no SPA fallback, no cache headers.

Three changes, ported from LocalAI `core/http/app.go:536-639`:

1. **Embed + graceful degradation.** `//go:embed react-ui/dist/*`, then
   `fs.Sub(reactUI, "react-ui/dist")`. On error, **log a warning and skip SPA
   route registration** rather than `panic` as `getFileSystem()` does today. A
   binary built without the UI must still serve the API.

2. **`serveIndex`** — reads `react-ui/dist/index.html`, sets
   `Cache-Control: no-cache`. Registered at `GET /app` and `GET /app/*`.
   `GET /` returns 301 to `/app`.

3. **Assets + SPA fallback.** `GET /assets/*` served from the embedded FS with
   `Cache-Control: public, max-age=31536000, immutable` (Vite emits
   content-hashed filenames). A custom `echo.HTTPErrorHandler` serves
   `index.html` on 404 when the request has `Accept: text/html` and is not
   `Content-Type: application/json`; otherwise it returns the JSON 404. This is
   how deep links work without a catch-all route swallowing API 404s.

The `/api/*` routes and `/debug/pprof/*` are untouched. The API contract does
not change in any way.

### Files deleted

- `api/public/index.tmpl` (582 lines)
- `api/public/functions.tmpl` (359 lines)
- `api/public/index.html` (2930 lines, generated **and committed**)
- `api/public/js/{alpine.min.js,alpine-magic-helpers.min.js,apexcharts.min.js,tailwind.min.js}`
- `api/public/images/logo.png` (replaced by the new mark)
- `api/generate/` (the whole package)
- `//go:generate` at `main.go:16`
- `getFileSystem()` in `api/api.go:52`

Removing the vendored `js/*.min.js` also removes them from Renovate's update
surface. Removing the CDN Font Awesome `<link>` at `index.tmpl:13` fixes icon
rendering on air-gapped nodes.

---

## 7. Frontend architecture

```
api/react-ui/
├── package.json
├── vite.config.ts          # base: './', dev proxy → :8080
├── tsconfig.json
├── index.html
├── src/
│   ├── main.tsx
│   ├── router.tsx          # createBrowserRouter, basename '/app', React.lazy
│   ├── styles/
│   │   ├── tokens.css      # the design system (§4)
│   │   └── base.css
│   ├── types/api.ts        # PascalCase models mirroring pkg/types + api/types
│   ├── lib/
│   │   ├── api.ts          # fetch wrapper, error with .status/.body
│   │   └── format.ts       # bytesToSize, truncated peer IDs
│   ├── hooks/
│   │   └── usePolling.ts
│   ├── components/         # Mark, DataTable, Tile, Pill, Breadcrumb, …
│   └── pages/              # one per route
└── dist/                   # gitignored
```

### API layer (`lib/api.ts`)

Mirrors `api/client/client.go:113-343` one-to-one. Single `handleResponse` that
attaches `status` and `body` to thrown errors. All models in `types/api.ts` use
**PascalCase field names** matching the wire format (D3) — e.g.
`{ PeerID: string; Address: string; Online: boolean }`.

### `usePolling`

The single most impactful fix. Today `index.tmpl` runs six independent
`$interval(updateItems, 1500)` timers, all mounted simultaneously because every
view lives in the DOM at once — roughly nine requests every 1.5s regardless of
which tab is visible or whether the previous request has returned.

The hook must be:
- **Visibility-aware** — pause on `document.hidden`, catch-up poll on focus.
- **Non-overlapping** — guard against stacked slow requests.
- **Route-scoped** — only the mounted route polls, which React Router gives us.
- **`enabled`-gated**, with a stable `refetch`.

Intervals: 1500ms for live views, 5500ms for `/app/blockchain` (matching
current behaviour).

### Dev workflow

`npm run dev` → Vite on :3000, proxying `/api` and `/debug` to
`http://localhost:8080`. Every endpoint is under those two prefixes, so the
proxy config is two lines. Go backend runs separately via `edgevpn api`.

---

## 8. Views

Six routes, replacing six hash views. **Zero new endpoints.**

| Route | Replaces | Endpoints | Interval |
|---|---|---|---|
| `/app` | `#` (Home) | `/api/summary`, `/api/metrics`, `/api/users` | 1500ms |
| `/app/nodes` | `#nodes` | `/api/machines`; `DELETE /api/ledger/machines/:address` | 1500ms |
| `/app/services` | `#services` | `/api/services`, `/api/files` | 1500ms |
| `/app/dns` | `#dns` | `/api/dns`; `DELETE /api/ledger/dns/:regex` | 1500ms |
| `/app/peers` | `#peers` | `/api/nodes`, `/api/peerstore`, `/api/metrics/peer`, `/api/machines` | 1500ms |
| `/app/blockchain` | `#blockchain` | `/api/blockchain` | 5500ms |

Deletes keep going through the generic `DELETE /api/ledger/:bucket/:key` route,
as today. The frontend must know that the `machines` bucket is keyed by IP
address and `dns` by regex — this coupling is preserved deliberately rather
than fixed, since typed delete endpoints are out of scope.

### Behavioural fixes carried into the port

- **Ledger paging.** `/api/blockchain` returns the last block including all
  `Storage`; on a large mesh this is a multi-megabyte payload every 5.5s. The
  view must page/virtualise client-side rather than pretty-printing the whole
  block. (Reducing the payload server-side would be an API change — out of
  scope.)
- **`peerstore` `Online` is always `false`** (`api/api.go:408`). The current UI
  renders it as a live column, which is misleading. The port must not render an
  online state for peerstore entries.
- **`OnChainNodes`** is present in `types.Summary` and never displayed. Surface
  it on `/app`.

### Table behaviour

Client-side search, sort and pagination as today (page size 10). Extracted into
one `DataTable` component instead of the templated `define "table"` in
`functions.tmpl`.

---

## 9. Peer graph (`/app/peers`)

The one idea borrowed from Direction B, rendered in Direction A's palette:
`--ev-signal` for active links, `--ev-muted` for the resting graph.

### Constraint: it can only be an ego graph

From a single node's API you can enumerate **your own** peers. There is no
endpoint exposing edges *between other peers* — that would require peers to
publish their peer lists to the ledger, which is a feature, not a port. This
limitation is accepted (approved 2026-08-02).

### v1 specification

- **Centre:** this node (`Summary.NodeID`).
- **Ring:** peers from `/api/nodes` and `/api/peerstore`, deduplicated.
- **Edge weight:** per-peer bandwidth from `/api/metrics/peer`, which returns
  `map[peer.ID]Stats`. Real data, not decoration.
- **Node state:** `Connected` / `OnChain` / `Online` from `/api/machines`,
  encoded as colour using the semantic tokens.
- **Rendering:** Canvas, no charting library. ApexCharts is being removed.
- **Accessibility:** must respect `prefers-reduced-motion` (render a static
  frame), pause when off-screen via `IntersectionObserver`, and be accompanied
  by the existing peers table as the accessible equivalent — the graph is
  additive, never the only way to read the data.

The graph sits above the existing Nodes and Peer store tables on the same
route; it does not replace them.

---

## 10. Testing

- **Type checking** (`tsc --noEmit`) in CI — the main guard, given TypeScript.
- **Unit tests** for `lib/format.ts` (`bytesToSize` etc., ported from the inline
  helpers at `index.tmpl:313-408`) and for `usePolling`'s visibility and
  overlap-guard behaviour, which are the two places real logic lives.
- **A build test in CI** — `make react-ui-force` must succeed, and `go build`
  after it must succeed. This is the regression test that matters most, since
  the failure mode being guarded against is a broken embed.
- **No e2e in this sub-project.** LocalAI has Playwright; adding it here is a
  separate decision and would be the first e2e infrastructure in this repo.
- Existing Go tests (`api/api_test.go`, `api_suite_test.go`) must still pass;
  they exercise the API, which is unchanged.

---

## 11. Risks

| Risk | Mitigation |
|---|---|
| `go:embed` breaks every CI job | §5 ordering; infra lands and is verified green before the directive changes |
| Stale `dist/` silently reused by the `ifneq` guard | `react-ui-force` target, used by CI and goreleaser |
| Contributors now need Node to build | Documented in README; the `dist/` stub trick lets Go-only work proceed |
| Parity regressions vs. the old UI | The endpoint table in §8 is the checklist; each route is verified against the corresponding hash view before the old files are deleted |
| Deleting `api/public/` breaks an unknown consumer | It is embedded, not served from disk, and nothing outside `api/api.go` references it |

---

## Appendix A — Endpoint reference

Confirmed against `api/api.go`. Unchanged by this work; listed so the port has
a single source of truth.

| Method | Path | Response |
|---|---|---|
| GET | `/api/summary` | `types.Summary{Files,Machines,Users,Services,BlockChain,OnChainNodes,Peers int; NodeID string}` |
| GET | `/api/machines` | `[]apiTypes.Machine` (`types.Machine` + `Connected,OnChain,Online bool`) |
| GET | `/api/nodes` | `[]apiTypes.Peer{ID,Online}` |
| GET | `/api/peerstore` | `[]apiTypes.Peer{ID}` — `Online` always false |
| GET | `/api/users` | `[]types.User{PeerID,Timestamp}` |
| GET | `/api/services` | `[]types.Service{PeerID,Name}` |
| GET | `/api/files` | `[]types.File{PeerID,Name}` |
| GET | `/api/dns` | `[]apiTypes.DNS{Regex,Records}` |
| GET | `/api/blockchain` | `blockchain.Block{Index,Timestamp,Storage,Hash,PrevHash}` |
| GET | `/api/ledger[/:bucket[/:key]]` | ledger data |
| GET | `/api/metrics` | `metrics.Stats{TotalIn,TotalOut,RateIn,RateOut}` |
| GET | `/api/metrics/peer` | `map[peer.ID]Stats` |
| GET | `/api/metrics/{peer,protocol}/:id` | `Stats` |
| GET | `/api/peergate` | `bool` |
| PUT | `/api/ledger/:bucket/:key/:value` | `{"State":"Announcing"}` |
| PUT | `/api/peergate/:state` | `bool` |
| POST | `/api/dns` | `{"State":"Announcing"}` |
| DELETE | `/api/ledger/:bucket[/:key]` | `{"State":"Announcing"}` |

Metrics routes are registered only when a bandwidth counter is present;
`/api/peergate` only when a peer gater is configured. The UI must tolerate
their absence.

`POST /api/dns` and `PUT /api/peergate/:state` are implemented but unused by
the current UI. They remain unused here — surfacing them means adding write
capability, which is out of scope.

---

## Appendix B — Deferred register

Found during scoping, deliberately excluded. Recorded so they are not lost.

### Code bugs (separate PRs)

1. **Phantom `urfave/cli/v3` dependency.** `go.mod:32` declares
   `github.com/urfave/cli/v3 v3.10.1` as a **direct** dependency, but no `.go`
   file imports it — the CLI is entirely v2 (`main.go:21`, all of `cmd/`).
   Renovate PRs (#1030, #1045, #1047) bumped the module line without porting
   any code. Either finish the migration or drop the line.
2. **`fmt.Printf(string(priv))` in `cmd/peergate.go`** — non-constant format
   string, a `go vet` printf violation. A `%` in the key payload corrupts output.

### Documentation defects (sub-project 2)

Highest-impact findings from the docs audit, for the deferred docs spec:

1. **`--peerguardian` does not exist.** `docs/content/en/docs/Concepts/Overview/peerguardian.md`
   lines 18, 21, 63, 91 instruct users to run it; the real flag is `--peerguard`
   (`cmd/util.go:368`). urfave/cli hard-fails on the unknown flag.
2. **The egress / exit-node / `edgevpn proxy` feature is entirely undocumented** —
   zero hits in `docs/content/`, despite `pkg/services/egress.go` being 224 lines.
3. **`edgevpn start` is undocumented** — the recommended way to run a relay/hop node.
4. **`--ownership` defaults to `enforce`**, is wire-format-incompatible across
   modes, and is documented only in `docs/design/authenticated-ledger.md`, which
   sits outside `content/` and never ships.
5. **`edgevpn api --api-listen` is wrong** in `Getting started/api.md`; the `api`
   subcommand takes `--listen`.
6. **No CLI or env-var reference.** ~90 flags exist and roughly 12 are
   documented; ~75 env vars are documented nowhere. Both should be
   **generated** from the `cli.App`, not hand-written — hand-maintained flag
   tables are what produced defect 1.
7. **~40% of API routes undocumented**, including the entire `/api/metrics` tree.
8. Proposed IA: a Diátaxis split (tutorials / how-to / reference / explanation)
   replacing the current `Concepts/Overview/` bucket, which holds four unrelated
   how-to topics under a heading that describes none of them.
