# EdgeVPN React UI Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace EdgeVPN's Alpine.js + Tailwind-CDN web UI with a React SPA built from source and embedded in the Go binary, with no build artifacts committed to git.

**Architecture:** A Vite + React + TypeScript app lives at `api/react-ui/`, builds to a gitignored `dist/`, and is embedded via `//go:embed`. A new `api/spa.go` owns index serving, immutable asset caching and SPA fallback, keeping `api/api.go` focused on the JSON API. Build infrastructure (Makefile, CI, goreleaser, Dockerfile) lands **before** the embed directive, because a missing `dist/` is a compile error.

**Tech Stack:** Go 1.26, echo v4, Vite 8, React 19, TypeScript 5.7, react-router-dom 7, Vitest 3. No CSS framework — hand-written CSS over custom properties. No charting library.

## Global Constraints

- **Design spec:** `docs/superpowers/specs/2026-08-02-react-ui-port-design.md`. Read it before starting.
- **Node ≥ 20.19 required.** Vite 8 will not run on Node 18. The local machine currently has v18.19.1 — upgrade before Task 1. CI uses Node 22.
- **The API contract does not change.** No new endpoints, no changed response shapes, no `json` struct tags. EdgeVPN's Go types serialize as **PascalCase** (`PeerID`, `RateIn`, `BlockChain`) and the TypeScript models must match that exactly.
- **No new dependencies beyond those listed in Task 1.** No Tailwind, no chart library, no component library, no state-management library.
- **Dark theme only.** No light theme, no theme toggle. Every colour comes from a token in `tokens.css`; no component may contain a literal colour value.
- **Never commit `api/react-ui/dist/` or `api/react-ui/node_modules/`.**
- **Accent discipline:** `--ev-signal` (`#F2542D`) is the only accent. Semantic state colours (`--ev-ok`, `--ev-warn`, `--ev-crit`) are separate and never used as decoration.
- **Typography:** monospace for all chrome, code, tables, addresses, hashes, flags and numbers. Columns of digits get `font-variant-numeric: tabular-nums`.
- **Commit after every task.** Work on branch `feat/react-ui-design-system`.

---

## File Structure

**Created:**

| Path | Responsibility |
|---|---|
| `Makefile` | Build orchestration. Guarantees `dist/` exists before `go build`. |
| `api/spa.go` | UI embed, index serving, asset caching, SPA fallback. All UI-serving logic, isolated from the JSON API. |
| `api/spa_test.go` | Tests for the above. |
| `api/react-ui/package.json`, `vite.config.ts`, `tsconfig.json`, `index.html` | Frontend build config. |
| `api/react-ui/src/styles/tokens.css` | The design system. Single source of colour, type and spacing. |
| `api/react-ui/src/styles/base.css` | Element-level resets and layout primitives. |
| `api/react-ui/src/types/api.ts` | TypeScript mirrors of the Go wire types. PascalCase. |
| `api/react-ui/src/lib/api.ts` | Fetch wrapper + one function per endpoint. Mirrors `api/client/client.go`. |
| `api/react-ui/src/lib/format.ts` | Pure formatting helpers. Ported from `index.tmpl:313-408`. |
| `api/react-ui/src/hooks/usePolling.ts` | Visibility-aware, non-overlapping polling. |
| `api/react-ui/src/components/*` | Mark, Layout, DataTable, Tile, Pill, PeerGraph. |
| `api/react-ui/src/pages/*` | One component per route. |
| `api/react-ui/src/router.tsx` | Route table, `basename="/app"`. |

**Modified:** `api/api.go` (embed + route registration), `.gitignore`, `.goreleaser.yml`, `Dockerfile`, `.github/workflows/test.yml`, `main.go`, `README.md`.

**Deleted:** `api/public/` (entire directory), `api/generate/` (entire package).

---

## Task 1: Frontend scaffold

**Files:**
- Create: `api/react-ui/package.json`, `api/react-ui/vite.config.ts`, `api/react-ui/tsconfig.json`, `api/react-ui/tsconfig.node.json`, `api/react-ui/index.html`, `api/react-ui/src/main.tsx`, `api/react-ui/src/App.tsx`, `api/react-ui/.gitignore`

**Interfaces:**
- Consumes: nothing.
- Produces: a working `npm run build` that emits `api/react-ui/dist/index.html` plus `dist/assets/*`. Every later frontend task depends on this toolchain.

- [ ] **Step 1: Verify Node version**

Run: `node -v`
Expected: `v20.19.x` or higher. **If it prints v18.x, stop and upgrade Node.** Vite 8 will fail with an unhelpful error otherwise.

- [ ] **Step 2: Create `api/react-ui/package.json`**

```json
{
  "name": "edgevpn-react-ui",
  "private": true,
  "version": "0.0.1",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit",
    "test": "vitest run"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0",
    "react-router-dom": "^7.18.1"
  },
  "devDependencies": {
    "@testing-library/react": "^16.1.0",
    "@types/react": "^19.1.0",
    "@types/react-dom": "^19.1.0",
    "@vitejs/plugin-react": "^6.0.2",
    "jsdom": "^25.0.1",
    "typescript": "^5.7.2",
    "vite": "^8.0.0",
    "vitest": "^3.0.0"
  }
}
```

- [ ] **Step 3: Create `api/react-ui/vite.config.ts`**

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = process.env.EDGEVPN_URL || 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  // Relative base so generated URLs resolve against the referencing file
  // rather than the origin root. Keeps the door open for reverse-proxy
  // subpath support later without a routing rewrite.
  base: './',
  server: {
    port: 3000,
    proxy: {
      '/api': backendUrl,
      '/debug': backendUrl,
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
})
```

- [ ] **Step 4: Create `api/react-ui/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "types": ["vitest/globals"]
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 5: Create `api/react-ui/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "noEmit": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 6: Create `api/react-ui/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="color-scheme" content="dark" />
    <link rel="icon" type="image/svg+xml" href="./favicon.svg" />
    <title>EdgeVPN</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 7: Create `api/react-ui/src/App.tsx`**

```tsx
export default function App() {
  return <div>EdgeVPN</div>
}
```

- [ ] **Step 8: Create `api/react-ui/src/main.tsx`**

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 9: Create `api/react-ui/.gitignore`**

```
dist
node_modules
```

- [ ] **Step 10: Install and build**

Run: `cd api/react-ui && npm install && npm run build`
Expected: PASS. `api/react-ui/dist/index.html` and `api/react-ui/dist/assets/` exist.

- [ ] **Step 11: Verify dist is not tracked**

Run: `git status --porcelain api/react-ui/dist`
Expected: **no output.** If `dist` files appear, the `.gitignore` in Step 9 is wrong — fix before continuing.

- [ ] **Step 12: Commit**

```bash
git add api/react-ui/package.json api/react-ui/package-lock.json api/react-ui/vite.config.ts \
        api/react-ui/tsconfig.json api/react-ui/tsconfig.node.json api/react-ui/index.html \
        api/react-ui/src api/react-ui/.gitignore
git commit -m "feat(ui): scaffold React + Vite + TypeScript frontend"
```

---

## Task 2: Build infrastructure

**Files:**
- Create: `Makefile`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `npm run build` from Task 1.
- Produces: `make react-ui`, `make react-ui-force`, `make build`. Task 3 and Task 14 depend on these target names.

- [ ] **Step 1: Create `Makefile`**

Note the `ifneq`/`else`/`endif` must start at column 0, and recipe lines must be indented with a **tab**, not spaces.

```make
.PHONY: all build react-ui react-ui-force test clean

all: build

# Skip the npm build when dist already exists, so repeated `make build`
# is fast. This intentionally reuses a stale dist — use react-ui-force
# in CI and releases where correctness matters more than speed.
react-ui:
ifneq ($(wildcard api/react-ui/dist),)
	@echo "api/react-ui/dist already exists, skipping build"
else
	cd api/react-ui && npm ci && npm run build
endif

# Always rebuild from source. Used by CI, goreleaser and Docker.
react-ui-force:
	rm -rf api/react-ui/dist
	cd api/react-ui && npm ci && npm run build

api/react-ui/dist: react-ui

build: api/react-ui/dist
	go build -o edgevpn

test: api/react-ui/dist
	go test ./...

clean:
	rm -rf api/react-ui/dist edgevpn
```

- [ ] **Step 2: Append to `.gitignore`**

The existing `/dist` entry is rooted and does **not** match `api/react-ui/dist`. Add:

```
/api/react-ui/dist
/api/react-ui/node_modules
```

- [ ] **Step 3: Verify the force target rebuilds**

Run: `rm -rf api/react-ui/dist && make react-ui-force && ls api/react-ui/dist/index.html`
Expected: the file exists.

- [ ] **Step 4: Verify the skip-guard skips**

Run: `make react-ui`
Expected: prints `api/react-ui/dist already exists, skipping build` and does not run npm.

- [ ] **Step 5: Verify `make build` still works**

Run: `make build && ./edgevpn --version`
Expected: builds and prints a version. (The embed still points at `api/public` at this stage — that is correct and intentional.)

- [ ] **Step 6: Commit**

```bash
git add Makefile .gitignore
git commit -m "build: add Makefile and gitignore react-ui build output"
```

---

## Task 3: CI, release and container wiring

**Files:**
- Modify: `.github/workflows/test.yml`, `.goreleaser.yml`, `Dockerfile`

**Interfaces:**
- Consumes: `make react-ui-force` from Task 2.
- Produces: every Go-compiling pipeline produces `dist/` first. Task 14 depends on this being green.

Four workflows compile Go. `pages.yml` builds Hugo only and needs no change.

- [ ] **Step 1: Add Node + UI build to both jobs in `.github/workflows/test.yml`**

In the `build` job, after the `Set up Go` step and before `Build`, insert:

```yaml
      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: 22
      - name: Build React UI
        run: make react-ui-force
```

Then change the `Build` step from `go build` to:

```yaml
      - name: Build
        run: go build
```

(unchanged command — it now succeeds because `dist/` exists).

Repeat the same two inserted steps in the `test-suite` job, after its `Set up Go` step.

- [ ] **Step 2: Add before-hooks to `.goreleaser.yml`**

The file currently has no `before:` block. Insert immediately after `version: 2`:

```yaml
before:
  hooks:
    - make react-ui-force
```

- [ ] **Step 3: Add a Node builder stage to `Dockerfile`**

Insert before the existing `FROM golang:1.26-alpine as builder` line:

```dockerfile
# Build the React UI in a Node stage so the Go builder can embed it
FROM node:22-alpine AS react-ui-builder
WORKDIR /ui
COPY api/react-ui/package.json api/react-ui/package-lock.json ./
RUN npm ci
COPY api/react-ui/ ./
RUN npm run build
```

Then, inside the Go builder stage, after `WORKDIR /work` and **before** the `RUN apk add ... go build` line, insert:

```dockerfile
# Bring in the built UI so //go:embed finds it
COPY --from=react-ui-builder /ui/dist /work/api/react-ui/dist
```

- [ ] **Step 4: Verify the Docker build**

Run: `docker build -t edgevpn-uitest .`
Expected: build succeeds. (It still embeds `api/public` at this stage; the point is proving the Node stage and the COPY work.)

- [ ] **Step 5: Verify goreleaser runs the hook**

Run: `rm -rf api/react-ui/dist && goreleaser build --clean --snapshot --single-target 2>&1 | head -30`
Expected: output shows `make react-ui-force` running before the build. If `goreleaser` is not installed locally, skip this step and rely on CI.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/test.yml .goreleaser.yml Dockerfile
git commit -m "ci: build React UI before every Go compile step"
```

---

## Task 4: Design tokens and the mark

**Files:**
- Create: `api/react-ui/src/styles/tokens.css`, `api/react-ui/src/styles/base.css`, `api/react-ui/src/components/Mark.tsx`, `api/react-ui/public/favicon.svg`

**Interfaces:**
- Consumes: the Task 1 scaffold.
- Produces: CSS custom properties named `--ev-*` used by every subsequent component, and `<Mark size={number} />`.

- [ ] **Step 1: Create `api/react-ui/src/styles/tokens.css`**

```css
/* EdgeVPN design system — Direction A "Multiaddr".
   Dark-committed by design. Every colour in the app comes from here;
   no component may contain a literal colour value. */
:root {
  /* Ground and surfaces. Graphite is blue-biased, never neutral grey. */
  --ev-bg:      #14181D;
  --ev-panel:   #1A2026;
  --ev-rule:    #2B333C;
  --ev-code:    #10141A;

  /* Text */
  --ev-ink:     #DFE3E8;
  --ev-muted:   #8A939E;
  --ev-faint:   #646D78;

  /* The single accent. Active path, current nav, focus ring. */
  --ev-signal:   #F2542D;
  --ev-signal-2: #FFB35C;

  /* Semantic state. Separate from the accent; never decorative. */
  --ev-ok:      #6FBF8B;
  --ev-warn:    #FFB35C;
  --ev-crit:    #E5484D;

  /* Type */
  --ev-mono: ui-monospace, "SF Mono", SFMono-Regular, "Cascadia Mono", Menlo,
             Consolas, "Liberation Mono", monospace;
  --ev-sans: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue",
             Arial, sans-serif;

  --ev-step--1: 0.78rem;
  --ev-step-0:  0.92rem;
  --ev-step-1:  1.1rem;
  --ev-step-2:  1.45rem;
  --ev-step-3:  2rem;

  /* Space, 4px base */
  --ev-1: 4px;
  --ev-2: 8px;
  --ev-3: 12px;
  --ev-4: 16px;
  --ev-5: 24px;
  --ev-6: 32px;
  --ev-7: 48px;

  --ev-radius: 2px;
}
```

- [ ] **Step 2: Create `api/react-ui/src/styles/base.css`**

```css
@import './tokens.css';

*, *::before, *::after { box-sizing: border-box; }

body {
  margin: 0;
  background: var(--ev-bg);
  color: var(--ev-ink);
  font-family: var(--ev-mono);
  font-size: var(--ev-step-0);
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
}

a { color: inherit; }

:focus-visible {
  outline: 2px solid var(--ev-signal);
  outline-offset: 2px;
}

/* Digits that appear in columns must line up. */
.tabular { font-variant-numeric: tabular-nums; }

/* The slash separator, drawn from multiaddr notation. Structural, not
   decorative: it always separates real path segments. */
.slash { color: var(--ev-signal); padding-inline: 0.15em; }

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    transition-duration: 0.001ms !important;
  }
}
```

- [ ] **Step 3: Create `api/react-ui/src/components/Mark.tsx`**

Two muted rings joined by a signal bar: peer, edge, peer, with no centre. Three shapes, so it survives 16px.

```tsx
type MarkProps = {
  size?: number
  title?: string
}

export default function Mark({ size = 24, title = 'EdgeVPN' }: MarkProps) {
  // Stroke widths scale up at small sizes so the mark stays legible
  // in a favicon or a 16px nav slot.
  const heavy = size <= 20
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      role="img"
      aria-label={title}
      style={{ flex: 'none' }}
    >
      <circle cx="13" cy="49" r="7" fill="none"
              stroke="var(--ev-muted)" strokeWidth={heavy ? 6 : 3} />
      <circle cx="51" cy="15" r="7" fill="none"
              stroke="var(--ev-muted)" strokeWidth={heavy ? 6 : 3} />
      <path d="M20 42 L44 22" stroke="var(--ev-signal)"
            strokeWidth={heavy ? 8 : 5} strokeLinecap="round" />
    </svg>
  )
}
```

- [ ] **Step 4: Create `api/react-ui/public/favicon.svg`**

Colours are literal here because an SVG loaded as a favicon has no access to the document's custom properties.

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
  <rect width="64" height="64" fill="#14181D"/>
  <circle cx="13" cy="49" r="7" fill="none" stroke="#8A939E" stroke-width="7"/>
  <circle cx="51" cy="15" r="7" fill="none" stroke="#8A939E" stroke-width="7"/>
  <path d="M20 42 L44 22" stroke="#F2542D" stroke-width="9" stroke-linecap="round"/>
</svg>
```

- [ ] **Step 5: Import base.css and render the mark**

Replace `api/react-ui/src/App.tsx` with:

```tsx
import './styles/base.css'
import Mark from './components/Mark'

export default function App() {
  return (
    <div style={{ padding: 'var(--ev-5)', display: 'flex', gap: 'var(--ev-3)', alignItems: 'center' }}>
      <Mark size={32} />
      <span>edge<span className="slash">/</span>vpn</span>
    </div>
  )
}
```

- [ ] **Step 6: Verify it builds and renders**

Run: `cd api/react-ui && npm run build`
Expected: PASS with no TypeScript errors.

Run: `npm run dev` and open `http://localhost:3000`
Expected: dark graphite background, the mark in muted grey with an orange bar, `edge/vpn` with an orange slash. Stop the dev server.

- [ ] **Step 7: Commit**

```bash
git add api/react-ui/src/styles api/react-ui/src/components/Mark.tsx \
        api/react-ui/public/favicon.svg api/react-ui/src/App.tsx
git commit -m "feat(ui): add design tokens and the EdgeVPN mark"
```

---

## Task 5: Wire types and formatting helpers

**Files:**
- Create: `api/react-ui/src/types/api.ts`, `api/react-ui/src/lib/format.ts`, `api/react-ui/src/lib/format.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: exported types `Summary`, `Machine`, `Peer`, `User`, `Service`, `FileEntry`, `DNSEntry`, `Block`, `Stats`, `PeerStats`; exported functions `bytesToSize(bytes: number): string`, `truncateID(id: string, keep?: number): string`, `formatRate(bytesPerSec: number): string`.

Field names are **PascalCase** because the Go types carry no `json` struct tags. This is deliberate — see decision D3 in the spec.

- [ ] **Step 1: Write the failing test at `api/react-ui/src/lib/format.test.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { bytesToSize, truncateID, formatRate } from './format'

describe('bytesToSize', () => {
  it('returns 0 B for zero', () => {
    expect(bytesToSize(0)).toBe('0 B')
  })
  it('formats bytes without a decimal', () => {
    expect(bytesToSize(512)).toBe('512 B')
  })
  it('formats kilobytes to one decimal', () => {
    expect(bytesToSize(1536)).toBe('1.5 kB')
  })
  it('formats megabytes to one decimal', () => {
    expect(bytesToSize(1_572_864)).toBe('1.5 MB')
  })
  it('handles gigabytes', () => {
    expect(bytesToSize(3_221_225_472)).toBe('3.0 GB')
  })
  it('treats negative input as zero', () => {
    expect(bytesToSize(-1)).toBe('0 B')
  })
})

describe('truncateID', () => {
  it('shortens a long peer ID from both ends', () => {
    expect(truncateID('12D3KooWKzabcdefghijklmnop', 6)).toBe('12D3Ko…klmnop')
  })
  it('leaves short IDs alone', () => {
    expect(truncateID('short', 6)).toBe('short')
  })
  it('handles an empty string', () => {
    expect(truncateID('', 6)).toBe('')
  })
})

describe('formatRate', () => {
  it('appends a per-second suffix', () => {
    expect(formatRate(1536)).toBe('1.5 kB/s')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd api/react-ui && npx vitest run src/lib/format.test.ts`
Expected: FAIL — cannot resolve `./format`.

- [ ] **Step 3: Create `api/react-ui/src/lib/format.ts`**

```ts
const UNITS = ['B', 'kB', 'MB', 'GB', 'TB', 'PB'] as const

/** Human-readable byte size. Ported from index.tmpl's bytesToSize. */
export function bytesToSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const i = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    UNITS.length - 1,
  )
  const value = bytes / Math.pow(1024, i)
  // Bytes are whole; everything larger reads better with one decimal.
  return i === 0 ? `${Math.round(value)} B` : `${value.toFixed(1)} ${UNITS[i]}`
}

/** Same, with a per-second suffix. */
export function formatRate(bytesPerSec: number): string {
  return `${bytesToSize(bytesPerSec)}/s`
}

/**
 * Shorten a peer ID for display, keeping both ends so IDs stay
 * distinguishable — libp2p peer IDs share long common prefixes.
 */
export function truncateID(id: string, keep = 6): string {
  if (!id || id.length <= keep * 2 + 1) return id
  return `${id.slice(0, keep)}…${id.slice(-keep)}`
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/lib/format.test.ts`
Expected: PASS, 10 tests.

- [ ] **Step 5: Create `api/react-ui/src/types/api.ts`**

```ts
/**
 * TypeScript mirrors of EdgeVPN's Go wire types.
 *
 * Field names are PascalCase on purpose: the Go structs in pkg/types and
 * api/types carry no `json` struct tags, so encoding/json emits the Go
 * field names verbatim. Adding tags would break api/client and external
 * consumers (Kairos, LocalAI), so the casing is absorbed here instead.
 */

/** pkg/types.Summary */
export interface Summary {
  Files: number
  Machines: number
  Users: number
  Services: number
  BlockChain: number
  OnChainNodes: number
  Peers: number
  NodeID: string
}

/** pkg/types.Machine, embedded in api/types.Machine */
export interface Machine {
  PeerID: string
  Hostname: string
  OS: string
  Arch: string
  Address: string
  Version: string
  Connected: boolean
  OnChain: boolean
  Online: boolean
}

/** api/types.Peer. Note: /api/peerstore always reports Online === false. */
export interface Peer {
  ID: string
  Online: boolean
}

/** pkg/types.User */
export interface User {
  PeerID: string
  Timestamp: string
}

/** pkg/types.Service */
export interface Service {
  PeerID: string
  Name: string
}

/** pkg/types.File — named FileEntry to avoid clashing with the DOM File type. */
export interface FileEntry {
  PeerID: string
  Name: string
}

/** api/types.DNS */
export interface DNSEntry {
  Regex: string
  Records: Record<string, string>
}

/** libp2p metrics.Stats */
export interface Stats {
  TotalIn: number
  TotalOut: number
  RateIn: number
  RateOut: number
}

/** GET /api/metrics/peer — keyed by peer ID */
export type PeerStats = Record<string, Stats>

/** blockchain.Block */
export interface Block {
  Index: number
  Timestamp: string
  Hash: string
  PrevHash: string
  Storage: Record<string, Record<string, unknown>>
}
```

- [ ] **Step 6: Typecheck**

Run: `npm run typecheck`
Expected: PASS, no errors.

- [ ] **Step 7: Commit**

```bash
git add api/react-ui/src/types api/react-ui/src/lib
git commit -m "feat(ui): add wire types and formatting helpers"
```

---

## Task 6: API client layer

**Files:**
- Create: `api/react-ui/src/lib/api.ts`, `api/react-ui/src/lib/api.test.ts`

**Interfaces:**
- Consumes: types from Task 5.
- Produces: `ApiError` class with `.status` and `.body`; functions `getSummary()`, `getMachines()`, `getNodes()`, `getPeerstore()`, `getUsers()`, `getServices()`, `getFiles()`, `getDNS()`, `getBlockchain()`, `getMetrics()`, `getPeerMetrics()`, `deleteLedgerKey(bucket, key)`. All return `Promise<T>`.

- [ ] **Step 1: Write the failing test at `api/react-ui/src/lib/api.test.ts`**

```ts
import { describe, it, expect, vi, afterEach } from 'vitest'
import { ApiError, getSummary, deleteLedgerKey } from './api'

afterEach(() => { vi.unstubAllGlobals() })

describe('getSummary', () => {
  it('returns parsed JSON on success', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ Machines: 7, NodeID: 'abc' }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )))
    const s = await getSummary()
    expect(s.Machines).toBe(7)
    expect(s.NodeID).toBe('abc')
  })

  it('throws ApiError carrying the status on failure', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      'boom', { status: 503 },
    )))
    await expect(getSummary()).rejects.toBeInstanceOf(ApiError)
    await expect(getSummary()).rejects.toMatchObject({ status: 503 })
  })
})

describe('deleteLedgerKey', () => {
  it('URL-encodes bucket and key so regexes and IPs survive', async () => {
    const spy = vi.fn(async () => new Response('{}', { status: 200 }))
    vi.stubGlobal('fetch', spy)
    await deleteLedgerKey('dns', 'foo.*\\.bar')
    expect(spy).toHaveBeenCalledWith(
      '/api/ledger/dns/foo.*%5C.bar',
      expect.objectContaining({ method: 'DELETE' }),
    )
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/lib/api.test.ts`
Expected: FAIL — cannot resolve `./api`.

- [ ] **Step 3: Create `api/react-ui/src/lib/api.ts`**

```ts
import type {
  Block, DNSEntry, FileEntry, Machine, Peer, PeerStats,
  Service, Stats, Summary, User,
} from '../types/api'

/** An HTTP-level failure. Carries the status so callers can branch on 404. */
export class ApiError extends Error {
  status: number
  body: string
  constructor(status: number, body: string) {
    super(`HTTP ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal })
  if (!res.ok) throw new ApiError(res.status, await res.text().catch(() => ''))
  return (await res.json()) as T
}

export const getSummary    = (s?: AbortSignal) => get<Summary>('/api/summary', s)
export const getMachines   = (s?: AbortSignal) => get<Machine[]>('/api/machines', s)
export const getNodes      = (s?: AbortSignal) => get<Peer[]>('/api/nodes', s)
export const getPeerstore  = (s?: AbortSignal) => get<Peer[]>('/api/peerstore', s)
export const getUsers      = (s?: AbortSignal) => get<User[]>('/api/users', s)
export const getServices   = (s?: AbortSignal) => get<Service[]>('/api/services', s)
export const getFiles      = (s?: AbortSignal) => get<FileEntry[]>('/api/files', s)
export const getDNS        = (s?: AbortSignal) => get<DNSEntry[]>('/api/dns', s)
export const getBlockchain = (s?: AbortSignal) => get<Block>('/api/blockchain', s)

/**
 * Bandwidth metrics. These routes are registered only when the node has a
 * bandwidth counter, so a 404 here is expected, not exceptional — callers
 * should treat it as "metrics unavailable".
 */
export const getMetrics     = (s?: AbortSignal) => get<Stats>('/api/metrics', s)
export const getPeerMetrics = (s?: AbortSignal) => get<PeerStats>('/api/metrics/peer', s)

/**
 * Delete a ledger entry. The UI must know each bucket's key semantics:
 * `machines` is keyed by IP address, `dns` by regex. Typed delete
 * endpoints are out of scope, so this coupling is preserved.
 */
export async function deleteLedgerKey(bucket: string, key: string): Promise<void> {
  const url = `/api/ledger/${encodeURIComponent(bucket)}/${encodeURIComponent(key)}`
  const res = await fetch(url, { method: 'DELETE' })
  if (!res.ok) throw new ApiError(res.status, await res.text().catch(() => ''))
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/lib/api.test.ts`
Expected: PASS, 3 tests.

- [ ] **Step 5: Commit**

```bash
git add api/react-ui/src/lib/api.ts api/react-ui/src/lib/api.test.ts
git commit -m "feat(ui): add typed API client mirroring api/client"
```

---

## Task 7: The polling hook

**Files:**
- Create: `api/react-ui/src/hooks/usePolling.ts`, `api/react-ui/src/hooks/usePolling.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `usePolling<T>(fetcher: (signal: AbortSignal) => Promise<T>, intervalMs: number, opts?: { enabled?: boolean }): { data: T | null; error: Error | null; loading: boolean; refetch: () => void }`. Every page uses this.

This is the single most impactful behavioural fix. The current UI runs six independent `$interval(updateItems, 1500)` timers, all mounted at once because every view lives in the DOM simultaneously — roughly nine requests every 1.5s regardless of tab visibility or whether the previous request returned.

- [ ] **Step 1: Write the failing test at `api/react-ui/src/hooks/usePolling.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { usePolling } from './usePolling'

beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
afterEach(() => { vi.useRealTimers() })

describe('usePolling', () => {
  it('fetches immediately on mount', async () => {
    const fetcher = vi.fn(async () => 'one')
    const { result } = renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(result.current.data).toBe('one'))
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('polls again after the interval elapses', async () => {
    const fetcher = vi.fn(async () => 'x')
    renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
  })

  it('does not stack requests when the fetcher is slower than the interval', async () => {
    let release: (v: string) => void = () => {}
    const fetcher = vi.fn(() => new Promise<string>((r) => { release = r }))
    renderHook(() => usePolling(fetcher, 100))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    // Let several intervals elapse while the first request is still open.
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(fetcher).toHaveBeenCalledTimes(1)
    await act(async () => { release('done') })
  })

  it('does not fetch when disabled', async () => {
    const fetcher = vi.fn(async () => 'x')
    renderHook(() => usePolling(fetcher, 1000, { enabled: false }))
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    expect(fetcher).not.toHaveBeenCalled()
  })

  it('surfaces fetcher errors without stopping the loop', async () => {
    const fetcher = vi.fn()
      .mockRejectedValueOnce(new Error('nope'))
      .mockResolvedValue('recovered')
    const { result } = renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(result.current.error?.message).toBe('nope'))
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    await waitFor(() => expect(result.current.data).toBe('recovered'))
    expect(result.current.error).toBeNull()
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/hooks/usePolling.test.ts`
Expected: FAIL — cannot resolve `./usePolling`.

- [ ] **Step 3: Create `api/react-ui/src/hooks/usePolling.ts`**

```ts
import { useCallback, useEffect, useRef, useState } from 'react'

type Options = { enabled?: boolean }

type Result<T> = {
  data: T | null
  error: Error | null
  loading: boolean
  refetch: () => void
}

/**
 * Poll an endpoint on an interval, with two guarantees the old Alpine UI
 * lacked:
 *
 *  - Visibility-aware: nothing is fetched while the tab is hidden, and a
 *    catch-up fetch runs the moment it becomes visible again.
 *  - Non-overlapping: a new request never starts while one is in flight,
 *    so a slow node cannot accumulate a backlog of stacked requests.
 *
 * Only the mounted route polls, since React Router unmounts the others.
 */
export function usePolling<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  intervalMs: number,
  opts: Options = {},
): Result<T> {
  const { enabled = true } = opts
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<Error | null>(null)
  const [loading, setLoading] = useState(false)

  const inFlight = useRef(false)
  const mounted = useRef(true)
  // Keep the latest fetcher in a ref so callers can pass an inline arrow
  // function without resetting the interval on every render.
  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const run = useCallback(async () => {
    if (inFlight.current || document.hidden) return
    inFlight.current = true
    setLoading(true)
    const controller = new AbortController()
    try {
      const result = await fetcherRef.current(controller.signal)
      if (!mounted.current) return
      setData(result)
      setError(null)
    } catch (e) {
      if (!mounted.current) return
      if ((e as Error).name === 'AbortError') return
      setError(e as Error)
    } finally {
      inFlight.current = false
      if (mounted.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    mounted.current = true
    if (!enabled) return

    void run()
    const id = setInterval(() => { void run() }, intervalMs)

    // Catch up as soon as the tab is visible again, rather than waiting
    // out the remainder of the interval.
    const onVisibility = () => { if (!document.hidden) void run() }
    document.addEventListener('visibilitychange', onVisibility)

    return () => {
      mounted.current = false
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, intervalMs, run])

  return { data, error, loading, refetch: run }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/hooks/usePolling.test.ts`
Expected: PASS, 5 tests.

- [ ] **Step 5: Run the whole suite and typecheck**

Run: `npx vitest run && npm run typecheck`
Expected: PASS, 18 tests total, no type errors.

- [ ] **Step 6: Commit**

```bash
git add api/react-ui/src/hooks
git commit -m "feat(ui): add visibility-aware non-overlapping polling hook"
```

---

## Task 8: Shell components and routing

**Files:**
- Create: `api/react-ui/src/components/Layout.tsx`, `api/react-ui/src/components/Layout.css`, `api/react-ui/src/components/Tile.tsx`, `api/react-ui/src/components/Pill.tsx`, `api/react-ui/src/components/DataTable.tsx`, `api/react-ui/src/components/DataTable.css`, `api/react-ui/src/router.tsx`
- Modify: `api/react-ui/src/main.tsx`, delete `api/react-ui/src/App.tsx`

**Interfaces:**
- Consumes: `Mark` (Task 4), tokens (Task 4).
- Produces:
  - `<Tile label={string} value={string | number} />`
  - `<Pill tone="ok" | "warn" | "crit">{children}</Pill>`
  - `<DataTable<T> columns={Column<T>[]} rows={T[]} rowKey={(r: T) => string} emptyText={string} />` where `Column<T> = { key: string; header: string; render: (row: T) => React.ReactNode; sortValue?: (row: T) => string | number }`
  - `router` from `router.tsx`

- [ ] **Step 1: Create `api/react-ui/src/components/Tile.tsx`**

```tsx
type TileProps = { label: string; value: string | number }

export default function Tile({ label, value }: TileProps) {
  return (
    <div className="ev-tile">
      <span className="ev-tile-k">{label}</span>
      <span className="ev-tile-v tabular">{value}</span>
    </div>
  )
}
```

- [ ] **Step 2: Create `api/react-ui/src/components/Pill.tsx`**

```tsx
import type { ReactNode } from 'react'

type Tone = 'ok' | 'warn' | 'crit'
type PillProps = { tone: Tone; children: ReactNode }

export default function Pill({ tone, children }: PillProps) {
  return <span className={`ev-pill ev-pill--${tone}`}>{children}</span>
}
```

- [ ] **Step 3: Create `api/react-ui/src/components/DataTable.tsx`**

Replaces the templated `define "table"` in `functions.tmpl`. Client-side search, sort and pagination, page size 10, matching current behaviour.

```tsx
import { useMemo, useState, type ReactNode } from 'react'
import './DataTable.css'

export type Column<T> = {
  key: string
  header: string
  render: (row: T) => ReactNode
  /** Omit to make the column unsortable. */
  sortValue?: (row: T) => string | number
}

type Props<T> = {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  emptyText?: string
  pageSize?: number
}

export default function DataTable<T>({
  columns, rows, rowKey, emptyText = 'Nothing here yet', pageSize = 10,
}: Props<T>) {
  const [query, setQuery] = useState('')
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [asc, setAsc] = useState(true)
  const [page, setPage] = useState(0)

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return rows
    // Search across every column's sortable value, falling back to the
    // row key so peer IDs remain findable.
    return rows.filter((r) =>
      columns.some((c) => c.sortValue
        ? String(c.sortValue(r)).toLowerCase().includes(q)
        : false) || rowKey(r).toLowerCase().includes(q),
    )
  }, [rows, query, columns, rowKey])

  const sorted = useMemo(() => {
    const col = columns.find((c) => c.key === sortKey)
    if (!col?.sortValue) return filtered
    const get = col.sortValue
    return [...filtered].sort((a, b) => {
      const av = get(a), bv = get(b)
      if (av === bv) return 0
      return (av < bv ? -1 : 1) * (asc ? 1 : -1)
    })
  }, [filtered, sortKey, asc, columns])

  const pages = Math.max(1, Math.ceil(sorted.length / pageSize))
  const current = Math.min(page, pages - 1)
  const visible = sorted.slice(current * pageSize, current * pageSize + pageSize)

  function toggleSort(key: string) {
    if (sortKey === key) setAsc(!asc)
    else { setSortKey(key); setAsc(true) }
    setPage(0)
  }

  return (
    <div className="ev-table-wrap">
      <div className="ev-table-tools">
        <input
          className="ev-search"
          type="search"
          placeholder="Filter…"
          value={query}
          onChange={(e) => { setQuery(e.target.value); setPage(0) }}
          aria-label="Filter table"
        />
        <span className="ev-count tabular">{sorted.length}</span>
      </div>

      <div className="ev-scroller">
        <table className="ev-table">
          <thead>
            <tr>
              {columns.map((c) => (
                <th key={c.key} scope="col">
                  {c.sortValue ? (
                    <button type="button" className="ev-sort" onClick={() => toggleSort(c.key)}>
                      {c.header}
                      {sortKey === c.key && <span aria-hidden="true">{asc ? ' ↑' : ' ↓'}</span>}
                    </button>
                  ) : c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 && (
              <tr><td colSpan={columns.length} className="ev-empty">{emptyText}</td></tr>
            )}
            {visible.map((row) => (
              <tr key={rowKey(row)}>
                {columns.map((c) => <td key={c.key}>{c.render(row)}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div className="ev-pager">
          <button type="button" onClick={() => setPage(Math.max(0, current - 1))}
                  disabled={current === 0}>Prev</button>
          <span className="tabular">{current + 1}<span className="slash">/</span>{pages}</span>
          <button type="button" onClick={() => setPage(Math.min(pages - 1, current + 1))}
                  disabled={current >= pages - 1}>Next</button>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 4: Create `api/react-ui/src/components/DataTable.css`**

```css
.ev-table-wrap { display: flex; flex-direction: column; gap: var(--ev-3); }

.ev-table-tools { display: flex; align-items: center; gap: var(--ev-3); }

.ev-search {
  flex: 1 1 auto; min-width: 0;
  background: var(--ev-bg);
  border: 1px solid var(--ev-rule);
  border-radius: var(--ev-radius);
  color: var(--ev-ink);
  font-family: var(--ev-mono);
  font-size: var(--ev-step--1);
  padding: var(--ev-2) var(--ev-3);
}
.ev-search::placeholder { color: var(--ev-faint); }

.ev-count { color: var(--ev-faint); font-size: var(--ev-step--1); }

.ev-scroller { overflow-x: auto; }

.ev-table { width: 100%; border-collapse: collapse; font-size: var(--ev-step--1); }
.ev-table th {
  text-align: left; font-weight: 500; color: var(--ev-faint);
  font-size: 0.68rem; letter-spacing: 0.1em; text-transform: uppercase;
  padding: var(--ev-2) var(--ev-3); border-bottom: 1px solid var(--ev-rule);
  white-space: nowrap;
}
.ev-table td {
  padding: var(--ev-2) var(--ev-3);
  border-bottom: 1px solid var(--ev-rule);
  color: var(--ev-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.ev-table tr:last-child td { border-bottom: 0; }

.ev-sort {
  background: none; border: 0; padding: 0; cursor: pointer;
  color: inherit; font: inherit; letter-spacing: inherit; text-transform: inherit;
}
.ev-sort:hover { color: var(--ev-ink); }

.ev-empty { color: var(--ev-faint); text-align: center; padding: var(--ev-5); }

.ev-pager { display: flex; align-items: center; gap: var(--ev-3); font-size: var(--ev-step--1); }
.ev-pager button {
  background: var(--ev-bg); color: var(--ev-ink);
  border: 1px solid var(--ev-rule); border-radius: var(--ev-radius);
  font-family: var(--ev-mono); font-size: var(--ev-step--1);
  padding: var(--ev-1) var(--ev-3); cursor: pointer;
}
.ev-pager button:disabled { color: var(--ev-faint); cursor: default; }
```

- [ ] **Step 5: Create `api/react-ui/src/components/Layout.css`**

```css
.ev-shell { min-height: 100vh; display: flex; flex-direction: column; }

.ev-header {
  display: flex; align-items: center; gap: var(--ev-5);
  padding: var(--ev-3) var(--ev-5);
  border-bottom: 1px solid var(--ev-rule);
  flex-wrap: wrap;
}

.ev-brand { display: flex; align-items: center; gap: var(--ev-2); font-weight: 600; }

.ev-nav { display: flex; gap: var(--ev-1); flex-wrap: wrap; }
.ev-nav a {
  text-decoration: none; color: var(--ev-muted);
  font-size: var(--ev-step--1); letter-spacing: 0.06em; text-transform: uppercase;
  padding: var(--ev-2) var(--ev-3);
  border-bottom: 2px solid transparent;
}
.ev-nav a:hover { color: var(--ev-ink); }
.ev-nav a.active { color: var(--ev-ink); border-bottom-color: var(--ev-signal); }

.ev-main { flex: 1; padding: var(--ev-5); display: flex; flex-direction: column; gap: var(--ev-5); }

.ev-panel {
  background: var(--ev-panel);
  border: 1px solid var(--ev-rule);
  border-radius: var(--ev-radius);
  padding: var(--ev-4);
  display: flex; flex-direction: column; gap: var(--ev-3);
}
.ev-panel-title {
  margin: 0; font-size: var(--ev-step--1); font-weight: 500;
  letter-spacing: 0.12em; text-transform: uppercase; color: var(--ev-faint);
}

.ev-tiles { display: grid; gap: var(--ev-2); grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); }
.ev-tile {
  border: 1px solid var(--ev-rule); border-radius: var(--ev-radius);
  padding: var(--ev-2) var(--ev-3); display: flex; flex-direction: column; gap: var(--ev-1);
}
.ev-tile-k { font-size: 0.62rem; letter-spacing: 0.1em; text-transform: uppercase; color: var(--ev-faint); }
.ev-tile-v { font-size: var(--ev-step-2); font-weight: 600; line-height: 1.1; }

.ev-pill {
  display: inline-flex; align-items: center;
  font-size: 0.62rem; letter-spacing: 0.06em; text-transform: uppercase;
  padding: 0 var(--ev-2); border-radius: 99px; border: 1px solid currentColor;
}
.ev-pill--ok { color: var(--ev-ok); }
.ev-pill--warn { color: var(--ev-warn); }
.ev-pill--crit { color: var(--ev-crit); }

.ev-error {
  color: var(--ev-crit); font-size: var(--ev-step--1);
  border: 1px solid currentColor; border-radius: var(--ev-radius);
  padding: var(--ev-2) var(--ev-3);
}
```

- [ ] **Step 6: Create `api/react-ui/src/components/Layout.tsx`**

```tsx
import { NavLink, Outlet } from 'react-router-dom'
import Mark from './Mark'
import './Layout.css'

const ROUTES = [
  { to: '/', label: 'Summary', end: true },
  { to: '/nodes', label: 'Nodes', end: false },
  { to: '/services', label: 'Services', end: false },
  { to: '/dns', label: 'DNS', end: false },
  { to: '/peers', label: 'Peers', end: false },
  { to: '/blockchain', label: 'Ledger', end: false },
]

export default function Layout() {
  return (
    <div className="ev-shell">
      <header className="ev-header">
        <div className="ev-brand">
          <Mark size={22} />
          <span>edge<span className="slash">/</span>vpn</span>
        </div>
        <nav className="ev-nav">
          {ROUTES.map((r) => (
            <NavLink key={r.to} to={r.to} end={r.end}
                     className={({ isActive }) => (isActive ? 'active' : undefined)}>
              {r.label}
            </NavLink>
          ))}
        </nav>
      </header>
      <main className="ev-main">
        <Outlet />
      </main>
    </div>
  )
}
```

- [ ] **Step 7: Create `api/react-ui/src/router.tsx`**

Pages are added in Tasks 9–13; for now every route renders a placeholder so routing can be verified independently.

```tsx
import { createBrowserRouter } from 'react-router-dom'
import { lazy, Suspense, type ComponentType } from 'react'
import Layout from './components/Layout'

function page(loader: () => Promise<{ default: ComponentType }>) {
  const C = lazy(loader)
  return (
    <Suspense fallback={<div className="ev-panel">Loading…</div>}>
      <C />
    </Suspense>
  )
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true,           element: page(() => import('./pages/SummaryPage')) },
      { path: 'nodes',         element: page(() => import('./pages/NodesPage')) },
      { path: 'services',      element: page(() => import('./pages/ServicesPage')) },
      { path: 'dns',           element: page(() => import('./pages/DNSPage')) },
      { path: 'peers',         element: page(() => import('./pages/PeersPage')) },
      { path: 'blockchain',    element: page(() => import('./pages/BlockchainPage')) },
    ],
  },
], { basename: '/app' })
```

- [ ] **Step 8: Create placeholder pages**

Create six files under `api/react-ui/src/pages/`, each replacing `NAME` with the page name (`SummaryPage`, `NodesPage`, `ServicesPage`, `DNSPage`, `PeersPage`, `BlockchainPage`):

```tsx
export default function NAME() {
  return <section className="ev-panel"><h2 className="ev-panel-title">NAME</h2></section>
}
```

- [ ] **Step 9: Replace `api/react-ui/src/main.tsx`**

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { router } from './router'
import './styles/base.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
```

- [ ] **Step 10: Delete the scaffold app**

Run: `rm api/react-ui/src/App.tsx`

- [ ] **Step 11: Verify**

Run: `npm run build`
Expected: PASS, no type errors.

Run: `npm run dev`, open `http://localhost:3000/app`
Expected: header with the mark and six nav items; clicking each changes the URL and the placeholder text; the active item has an orange underline. Stop the dev server.

- [ ] **Step 12: Commit**

```bash
git add -A api/react-ui/src
git commit -m "feat(ui): add shell layout, shared components and routing"
```

`git add -A` stages the `App.tsx` deletion from Step 10 along with the new files.

---

## Task 9: Summary page

**Files:**
- Modify: `api/react-ui/src/pages/SummaryPage.tsx`

**Interfaces:**
- Consumes: `usePolling`, `getSummary`, `getMetrics`, `getUsers`, `Tile`, `DataTable`, `formatRate`, `bytesToSize`, `truncateID`.
- Produces: nothing consumed elsewhere.

Replaces the `#` (Home) hash view. Endpoints: `/api/summary`, `/api/metrics`, `/api/users`. Interval 1500ms.

- [ ] **Step 1: Write the page**

```tsx
import { getMetrics, getSummary, getUsers } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { bytesToSize, formatRate, truncateID } from '../lib/format'
import type { User } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'
import Tile from '../components/Tile'

const COLUMNS: Column<User>[] = [
  { key: 'peer', header: 'Peer ID',
    render: (u) => <span title={u.PeerID}>{truncateID(u.PeerID, 8)}</span>,
    sortValue: (u) => u.PeerID },
  { key: 'ts', header: 'Last seen', render: (u) => u.Timestamp, sortValue: (u) => u.Timestamp },
]

export default function SummaryPage() {
  const summary = usePolling((s) => getSummary(s), 1500)
  const users = usePolling((s) => getUsers(s), 1500)
  // Metrics routes are registered only when the node has a bandwidth
  // counter, so absence is normal rather than an error.
  const metrics = usePolling((s) => getMetrics(s), 1500)

  const s = summary.data

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Network</h2>
        {summary.error && <p className="ev-error">Cannot reach the node: {summary.error.message}</p>}
        <div className="ev-tiles">
          <Tile label="Machines" value={s?.Machines ?? '—'} />
          <Tile label="On chain" value={s?.OnChainNodes ?? '—'} />
          <Tile label="Peers" value={s?.Peers ?? '—'} />
          <Tile label="Users" value={s?.Users ?? '—'} />
          <Tile label="Services" value={s?.Services ?? '—'} />
          <Tile label="Files" value={s?.Files ?? '—'} />
          <Tile label="Block" value={s?.BlockChain ?? '—'} />
        </div>
        {s?.NodeID && (
          <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
            node<span className="slash">/</span>
            <span title={s.NodeID}>{truncateID(s.NodeID, 10)}</span>
          </p>
        )}
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Bandwidth</h2>
        {metrics.data ? (
          <div className="ev-tiles">
            <Tile label="Rate in" value={formatRate(metrics.data.RateIn)} />
            <Tile label="Rate out" value={formatRate(metrics.data.RateOut)} />
            <Tile label="Total in" value={bytesToSize(metrics.data.TotalIn)} />
            <Tile label="Total out" value={bytesToSize(metrics.data.TotalOut)} />
          </div>
        ) : (
          <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
            Bandwidth metrics are not enabled on this node.
          </p>
        )}
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Connected users</h2>
        <DataTable
          columns={COLUMNS}
          rows={users.data ?? []}
          rowKey={(u) => u.PeerID}
          emptyText="No users announced yet"
        />
      </section>
    </>
  )
}
```

- [ ] **Step 2: Verify against a running node**

In terminal 1: `make build && ./edgevpn -g > /tmp/ev.yaml && ./edgevpn api --config /tmp/ev.yaml --listen ':8080'`
In terminal 2: `cd api/react-ui && npm run dev`
Open `http://localhost:3000/app`.
Expected: tiles populate with real numbers; the node ID renders truncated; the users table renders. Compare the numbers against `curl -s localhost:8080/api/summary`.

- [ ] **Step 3: Verify polling pauses when hidden**

With DevTools Network open, switch to another browser tab for ~10s, then return.
Expected: no requests while hidden, one immediate catch-up request on return.

- [ ] **Step 4: Typecheck and commit**

```bash
cd api/react-ui && npm run typecheck && cd -
git add api/react-ui/src/pages/SummaryPage.tsx
git commit -m "feat(ui): add summary page"
```

---

## Task 10: Nodes and Services pages

**Files:**
- Modify: `api/react-ui/src/pages/NodesPage.tsx`, `api/react-ui/src/pages/ServicesPage.tsx`

**Interfaces:**
- Consumes: `usePolling`, `getMachines`, `getServices`, `getFiles`, `deleteLedgerKey`, `DataTable`, `Pill`, `truncateID`.

Nodes replaces `#nodes`; Services replaces `#services`. Both at 1500ms.

- [ ] **Step 1: Write `NodesPage.tsx`**

The `machines` ledger bucket is keyed by **IP address**, which is why the delete uses `m.Address`.

```tsx
import { useState } from 'react'
import { deleteLedgerKey, getMachines } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { truncateID } from '../lib/format'
import type { Machine } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'
import Pill from '../components/Pill'

export default function NodesPage() {
  const machines = usePolling((s) => getMachines(s), 1500)
  const [busy, setBusy] = useState<string | null>(null)
  const [err, setErr] = useState<string | null>(null)

  async function forget(m: Machine) {
    setBusy(m.Address)
    setErr(null)
    try {
      await deleteLedgerKey('machines', m.Address)
      machines.refetch()
    } catch (e) {
      setErr(`Could not remove ${m.Address}: ${(e as Error).message}`)
    } finally {
      setBusy(null)
    }
  }

  const columns: Column<Machine>[] = [
    { key: 'addr', header: 'Address', render: (m) => m.Address, sortValue: (m) => m.Address },
    { key: 'host', header: 'Hostname', render: (m) => m.Hostname, sortValue: (m) => m.Hostname },
    { key: 'peer', header: 'Peer ID',
      render: (m) => <span title={m.PeerID}>{truncateID(m.PeerID, 6)}</span>,
      sortValue: (m) => m.PeerID },
    { key: 'os', header: 'Platform', render: (m) => `${m.OS}/${m.Arch}`, sortValue: (m) => m.OS },
    { key: 'ver', header: 'Version', render: (m) => m.Version, sortValue: (m) => m.Version },
    { key: 'state', header: 'State',
      render: (m) => m.Online
        ? <Pill tone="ok">{m.Connected ? 'direct' : 'online'}</Pill>
        : <Pill tone="warn">stale</Pill>,
      sortValue: (m) => (m.Online ? 1 : 0) },
    { key: 'act', header: '',
      render: (m) => (
        <button type="button" className="ev-sort" disabled={busy === m.Address}
                onClick={() => void forget(m)}
                aria-label={`Remove ${m.Address} from the ledger`}>
          {busy === m.Address ? '…' : 'remove'}
        </button>
      ) },
  ]

  return (
    <section className="ev-panel">
      <h2 className="ev-panel-title">Nodes</h2>
      {err && <p className="ev-error">{err}</p>}
      <DataTable columns={columns} rows={machines.data ?? []}
                 rowKey={(m) => m.Address} emptyText="No machines on the ledger yet" />
    </section>
  )
}
```

- [ ] **Step 2: Write `ServicesPage.tsx`**

```tsx
import { getFiles, getServices } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { truncateID } from '../lib/format'
import type { FileEntry, Service } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'

const SERVICE_COLUMNS: Column<Service>[] = [
  { key: 'name', header: 'Name', render: (s) => s.Name, sortValue: (s) => s.Name },
  { key: 'peer', header: 'Served by',
    render: (s) => <span title={s.PeerID}>{truncateID(s.PeerID, 8)}</span>,
    sortValue: (s) => s.PeerID },
]

const FILE_COLUMNS: Column<FileEntry>[] = [
  { key: 'name', header: 'Name', render: (f) => f.Name, sortValue: (f) => f.Name },
  { key: 'peer', header: 'Shared by',
    render: (f) => <span title={f.PeerID}>{truncateID(f.PeerID, 8)}</span>,
    sortValue: (f) => f.PeerID },
]

export default function ServicesPage() {
  const services = usePolling((s) => getServices(s), 1500)
  const files = usePolling((s) => getFiles(s), 1500)

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">TCP tunnels</h2>
        <DataTable columns={SERVICE_COLUMNS} rows={services.data ?? []}
                   rowKey={(s) => `${s.PeerID}/${s.Name}`}
                   emptyText="No services advertised" />
      </section>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Files</h2>
        <DataTable columns={FILE_COLUMNS} rows={files.data ?? []}
                   rowKey={(f) => `${f.PeerID}/${f.Name}`}
                   emptyText="No files shared" />
      </section>
    </>
  )
}
```

- [ ] **Step 3: Verify against a running node**

With the node and dev server from Task 9 Step 2, visit `/app/nodes` and `/app/services`.
Expected: the local machine appears in Nodes with an `online` pill; sorting and filtering work; the remove button issues a `DELETE /api/ledger/machines/<ip>` visible in the Network tab.

- [ ] **Step 4: Typecheck and commit**

```bash
cd api/react-ui && npm run typecheck && cd -
git add api/react-ui/src/pages/NodesPage.tsx api/react-ui/src/pages/ServicesPage.tsx
git commit -m "feat(ui): add nodes and services pages"
```

---

## Task 11: DNS and Ledger pages

**Files:**
- Modify: `api/react-ui/src/pages/DNSPage.tsx`, `api/react-ui/src/pages/BlockchainPage.tsx`

**Interfaces:**
- Consumes: `usePolling`, `getDNS`, `getBlockchain`, `deleteLedgerKey`, `DataTable`.

DNS replaces `#dns` (1500ms). Blockchain replaces `#blockchain` (**5500ms**, matching current behaviour).

- [ ] **Step 1: Write `DNSPage.tsx`**

The `dns` bucket is keyed by **regex**, hence the delete key.

```tsx
import { useState } from 'react'
import { deleteLedgerKey, getDNS } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import type { DNSEntry } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'

export default function DNSPage() {
  const dns = usePolling((s) => getDNS(s), 1500)
  const [busy, setBusy] = useState<string | null>(null)
  const [err, setErr] = useState<string | null>(null)

  async function forget(e: DNSEntry) {
    setBusy(e.Regex)
    setErr(null)
    try {
      await deleteLedgerKey('dns', e.Regex)
      dns.refetch()
    } catch (ex) {
      setErr(`Could not remove ${e.Regex}: ${(ex as Error).message}`)
    } finally {
      setBusy(null)
    }
  }

  const columns: Column<DNSEntry>[] = [
    { key: 'regex', header: 'Match', render: (e) => e.Regex, sortValue: (e) => e.Regex },
    { key: 'records', header: 'Records',
      render: (e) => (
        <span>
          {Object.entries(e.Records ?? {}).map(([type, value], i) => (
            <span key={type}>
              {i > 0 && <span className="slash">/</span>}
              <span style={{ color: 'var(--ev-faint)' }}>{type}</span> {value}
            </span>
          ))}
        </span>
      ) },
    { key: 'act', header: '',
      render: (e) => (
        <button type="button" className="ev-sort" disabled={busy === e.Regex}
                onClick={() => void forget(e)}
                aria-label={`Remove DNS entry ${e.Regex}`}>
          {busy === e.Regex ? '…' : 'remove'}
        </button>
      ) },
  ]

  return (
    <section className="ev-panel">
      <h2 className="ev-panel-title">DNS</h2>
      {err && <p className="ev-error">{err}</p>}
      <DataTable columns={columns} rows={dns.data ?? []}
                 rowKey={(e) => e.Regex} emptyText="No DNS entries announced" />
    </section>
  )
}
```

- [ ] **Step 2: Write `BlockchainPage.tsx`**

The old UI pretty-printed the entire last block including all `Storage`, which on a large mesh is a multi-megabyte render every 5.5s. This pages the buckets instead and only expands one at a time.

```tsx
import { useState } from 'react'
import { getBlockchain } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { truncateID } from '../lib/format'
import DataTable, { type Column } from '../components/DataTable'
import Tile from '../components/Tile'

type BucketRow = { bucket: string; keys: number }

export default function BlockchainPage() {
  const block = usePolling((s) => getBlockchain(s), 5500)
  const [open, setOpen] = useState<string | null>(null)

  const storage = block.data?.Storage ?? {}
  const rows: BucketRow[] = Object.entries(storage)
    .map(([bucket, entries]) => ({ bucket, keys: Object.keys(entries ?? {}).length }))

  const columns: Column<BucketRow>[] = [
    { key: 'bucket', header: 'Bucket', render: (r) => r.bucket, sortValue: (r) => r.bucket },
    { key: 'keys', header: 'Keys', render: (r) => r.keys, sortValue: (r) => r.keys },
    { key: 'act', header: '',
      render: (r) => (
        <button type="button" className="ev-sort"
                onClick={() => setOpen(open === r.bucket ? null : r.bucket)}>
          {open === r.bucket ? 'hide' : 'inspect'}
        </button>
      ) },
  ]

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Last block</h2>
        <div className="ev-tiles">
          <Tile label="Index" value={block.data?.Index ?? '—'} />
          <Tile label="Buckets" value={rows.length} />
        </div>
        {block.data && (
          <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
            {block.data.Timestamp}
            <span className="slash">/</span>
            <span title={block.data.Hash}>{truncateID(block.data.Hash, 10)}</span>
            <span className="slash">/</span>
            <span title={block.data.PrevHash}>prev {truncateID(block.data.PrevHash, 6)}</span>
          </p>
        )}
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Storage</h2>
        <DataTable columns={columns} rows={rows} rowKey={(r) => r.bucket}
                   emptyText="Block carries no storage" />
        {open && (
          <pre className="ev-scroller" style={{
            margin: 0, background: 'var(--ev-code)', border: '1px solid var(--ev-rule)',
            borderRadius: 'var(--ev-radius)', padding: 'var(--ev-3)',
            fontSize: 'var(--ev-step--1)', maxHeight: '40vh', overflow: 'auto',
          }}>
            {JSON.stringify(storage[open], null, 2)}
          </pre>
        )}
      </section>
    </>
  )
}
```

- [ ] **Step 3: Verify**

Visit `/app/dns` and `/app/blockchain` against the running node.
Expected: the ledger page shows the block index, a truncated hash, and a bucket table (`machines`, `users`, etc.); `inspect` expands exactly one bucket. Confirm in DevTools that `/api/blockchain` is requested every ~5.5s, not every 1.5s.

- [ ] **Step 4: Typecheck and commit**

```bash
cd api/react-ui && npm run typecheck && cd -
git add api/react-ui/src/pages/DNSPage.tsx api/react-ui/src/pages/BlockchainPage.tsx
git commit -m "feat(ui): add DNS and ledger pages"
```

---

## Task 12: Peers page (tables)

**Files:**
- Modify: `api/react-ui/src/pages/PeersPage.tsx`

**Interfaces:**
- Consumes: `usePolling`, `getNodes`, `getPeerstore`, `getMachines`, `getPeerMetrics`, `DataTable`, `Pill`.
- Produces: the `PeerRow` shape reused by Task 13 — `{ id: string; online: boolean; known: boolean; rateIn: number; rateOut: number }`, and an exported `usePeerRows()` hook returning `{ rows: PeerRow[]; error: Error | null }`.

Replaces `#peers`. **`/api/peerstore` always reports `Online: false`** (`api/api.go:408`), so the port must not render a live state for peerstore-only entries — the old UI did, and it was misleading.

- [ ] **Step 1: Write `PeersPage.tsx`**

```tsx
import { useMemo } from 'react'
import { getMachines, getNodes, getPeerMetrics, getPeerstore } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { formatRate, truncateID } from '../lib/format'
import DataTable, { type Column } from '../components/DataTable'
import Pill from '../components/Pill'

export type PeerRow = {
  id: string
  /** True only when a source that actually reports liveness says so. */
  online: boolean
  /** True when the peer is on the ledger, i.e. more than a peerstore entry. */
  known: boolean
  rateIn: number
  rateOut: number
}

/**
 * Merge the three peer sources into one view.
 *
 * /api/nodes reports real liveness. /api/peerstore is a bare address book
 * and always reports Online:false, so its entries contribute identity only.
 * /api/machines tells us which peers are on the ledger.
 */
export function usePeerRows(): { rows: PeerRow[]; error: Error | null } {
  const nodes = usePolling((s) => getNodes(s), 1500)
  const store = usePolling((s) => getPeerstore(s), 1500)
  const machines = usePolling((s) => getMachines(s), 1500)
  const metrics = usePolling((s) => getPeerMetrics(s), 1500)

  const rows = useMemo(() => {
    const byId = new Map<string, PeerRow>()
    const onLedger = new Set((machines.data ?? []).map((m) => m.PeerID))

    for (const p of store.data ?? []) {
      byId.set(p.ID, { id: p.ID, online: false, known: onLedger.has(p.ID), rateIn: 0, rateOut: 0 })
    }
    for (const p of nodes.data ?? []) {
      const existing = byId.get(p.ID)
      byId.set(p.ID, {
        id: p.ID,
        online: p.Online,
        known: onLedger.has(p.ID) || (existing?.known ?? false),
        rateIn: 0, rateOut: 0,
      })
    }
    for (const [id, stats] of Object.entries(metrics.data ?? {})) {
      const row = byId.get(id)
      if (row) { row.rateIn = stats.RateIn; row.rateOut = stats.RateOut }
    }
    return [...byId.values()]
  }, [nodes.data, store.data, machines.data, metrics.data])

  return { rows, error: nodes.error ?? store.error }
}

const COLUMNS: Column<PeerRow>[] = [
  { key: 'id', header: 'Peer ID',
    render: (p) => <span title={p.id}>{truncateID(p.id, 8)}</span>,
    sortValue: (p) => p.id },
  { key: 'state', header: 'State',
    render: (p) => p.online
      ? <Pill tone="ok">connected</Pill>
      : <Pill tone="warn">known</Pill>,
    sortValue: (p) => (p.online ? 1 : 0) },
  { key: 'ledger', header: 'On ledger',
    render: (p) => (p.known ? 'yes' : '—'), sortValue: (p) => (p.known ? 1 : 0) },
  { key: 'in', header: 'Rate in',
    render: (p) => (p.rateIn ? formatRate(p.rateIn) : '—'), sortValue: (p) => p.rateIn },
  { key: 'out', header: 'Rate out',
    render: (p) => (p.rateOut ? formatRate(p.rateOut) : '—'), sortValue: (p) => p.rateOut },
]

export default function PeersPage() {
  const { rows, error } = usePeerRows()

  return (
    <section className="ev-panel">
      <h2 className="ev-panel-title">Peers</h2>
      {error && <p className="ev-error">{error.message}</p>}
      <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
        <b style={{ color: 'var(--ev-muted)' }}>connected</b> peers have a live session.
        {' '}<b style={{ color: 'var(--ev-muted)' }}>known</b> peers are address-book entries
        {' '}whose liveness this node does not track.
      </p>
      <DataTable columns={COLUMNS} rows={rows} rowKey={(p) => p.id}
                 emptyText="No peers discovered yet" />
    </section>
  )
}
```

- [ ] **Step 2: Verify**

Visit `/app/peers`.
Expected: peers listed, no peerstore-only entry showing a live "connected" state, rates shown where `/api/metrics/peer` reports them and `—` otherwise.

- [ ] **Step 3: Typecheck and commit**

```bash
cd api/react-ui && npm run typecheck && cd -
git add api/react-ui/src/pages/PeersPage.tsx
git commit -m "feat(ui): add peers page merging nodes, peerstore and metrics"
```

---

## Task 13: Peer graph

**Files:**
- Create: `api/react-ui/src/components/PeerGraph.tsx`
- Modify: `api/react-ui/src/pages/PeersPage.tsx`

**Interfaces:**
- Consumes: `PeerRow` and `usePeerRows` from Task 12; `getSummary` for the local node ID.
- Produces: `<PeerGraph peers={PeerRow[]} selfId={string} />`.

**Scope constraint (approved):** this is an **ego graph**, not a mesh. From one node's API you can enumerate your own peers; no endpoint exposes edges between *other* peers. Showing that would require peers to publish their peer lists to the ledger — a feature, not a port.

- [ ] **Step 1: Create `api/react-ui/src/components/PeerGraph.tsx`**

```tsx
import { useEffect, useRef } from 'react'
import type { PeerRow } from '../pages/PeersPage'

type Props = { peers: PeerRow[]; selfId: string }

/**
 * Ego graph: this node at the centre, its peers on a ring.
 *
 * Edge thickness encodes real per-peer bandwidth from /api/metrics/peer.
 * Edges between other peers are deliberately absent — no endpoint exposes
 * that topology, and inventing it would be a lie about the network.
 *
 * The peers table below is the accessible equivalent; this canvas is
 * additive and is never the only way to read the data.
 */
export default function PeerGraph({ peers, selfId }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const peersRef = useRef(peers)
  peersRef.current = peers

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let raf = 0
    let running = true

    // Canvas cannot read CSS custom properties directly, so resolve them
    // once here. No literal fallbacks: tokens.css is imported by base.css
    // before this ever mounts, and a silent hardcoded fallback would let
    // the canvas drift out of the design system unnoticed.
    const css = getComputedStyle(document.documentElement)
    const colSignal = css.getPropertyValue('--ev-signal').trim()
    const colMuted = css.getPropertyValue('--ev-muted').trim()
    const colRule = css.getPropertyValue('--ev-rule').trim()
    const colOk = css.getPropertyValue('--ev-ok').trim()

    function resize() {
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      const rect = canvas!.getBoundingClientRect()
      canvas!.width = Math.round(rect.width * dpr)
      canvas!.height = Math.round(rect.height * dpr)
      ctx!.setTransform(dpr, 0, 0, dpr, 0, 0)
      return rect
    }

    function draw(t: number) {
      const rect = resize()
      const w = rect.width, h = rect.height
      const cx = w / 2, cy = h / 2
      const radius = Math.min(w, h) / 2 - 26
      ctx!.clearRect(0, 0, w, h)

      const list = peersRef.current
      const maxRate = Math.max(1, ...list.map((p) => p.rateIn + p.rateOut))

      list.forEach((p, i) => {
        const angle = (i / Math.max(1, list.length)) * Math.PI * 2 - Math.PI / 2
        const px = cx + Math.cos(angle) * radius
        const py = cy + Math.sin(angle) * radius

        // Edge: width from real traffic, colour from liveness.
        const share = (p.rateIn + p.rateOut) / maxRate
        ctx!.strokeStyle = p.online ? colMuted : colRule
        ctx!.lineWidth = 0.6 + share * 3
        ctx!.beginPath()
        ctx!.moveTo(cx, cy)
        ctx!.lineTo(px, py)
        ctx!.stroke()

        // A pulse travelling outward, only where traffic is actually flowing.
        if (!reduced && share > 0.02) {
          const phase = ((t / 1400) + i * 0.13) % 1
          ctx!.fillStyle = colSignal
          ctx!.beginPath()
          ctx!.arc(cx + (px - cx) * phase, cy + (py - cy) * phase, 2, 0, Math.PI * 2)
          ctx!.fill()
        }

        ctx!.fillStyle = p.online ? colOk : colRule
        ctx!.beginPath()
        ctx!.arc(px, py, 4.5, 0, Math.PI * 2)
        ctx!.fill()
      })

      // This node, last so it sits on top.
      ctx!.fillStyle = colSignal
      ctx!.beginPath()
      ctx!.arc(cx, cy, 7, 0, Math.PI * 2)
      ctx!.fill()

      if (running && !reduced) raf = requestAnimationFrame(draw)
    }

    raf = requestAnimationFrame(draw)

    // Stop burning frames when the graph scrolls out of view.
    const io = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        if (!running) { running = true; raf = requestAnimationFrame(draw) }
      } else {
        running = false
        cancelAnimationFrame(raf)
      }
    }, { threshold: 0.05 })
    io.observe(canvas)

    const onResize = () => { if (reduced) draw(0) }
    window.addEventListener('resize', onResize)

    return () => {
      running = false
      cancelAnimationFrame(raf)
      io.disconnect()
      window.removeEventListener('resize', onResize)
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      role="img"
      aria-label={`Network graph: this node connected to ${peers.length} peers. The table below lists them.`}
      style={{ display: 'block', width: '100%', height: '260px' }}
      data-self={selfId}
    />
  )
}
```

- [ ] **Step 2: Mount it in `PeersPage.tsx`**

In `api/react-ui/src/pages/PeersPage.tsx`, extend the existing `../lib/api`
import to include `getSummary` (do not add a second import from the same
module):

```tsx
import { getMachines, getNodes, getPeerMetrics, getPeerstore, getSummary } from '../lib/api'
```

And add:

```tsx
import PeerGraph from '../components/PeerGraph'
```

Then replace the `export default function PeersPage()` body with:

```tsx
export default function PeersPage() {
  const { rows, error } = usePeerRows()
  const summary = usePolling((s) => getSummary(s), 5500)

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Topology</h2>
        <PeerGraph peers={rows} selfId={summary.data?.NodeID ?? ''} />
        <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
          This node and its direct peers. Edge width is live per-peer bandwidth.
          Links between other peers are not shown — no endpoint reports them.
        </p>
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Peers</h2>
        {error && <p className="ev-error">{error.message}</p>}
        <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
          <b style={{ color: 'var(--ev-muted)' }}>connected</b> peers have a live session.
          {' '}<b style={{ color: 'var(--ev-muted)' }}>known</b> peers are address-book entries
          {' '}whose liveness this node does not track.
        </p>
        <DataTable columns={COLUMNS} rows={rows} rowKey={(p) => p.id}
                   emptyText="No peers discovered yet" />
      </section>
    </>
  )
}
```

- [ ] **Step 3: Verify rendering and motion**

Visit `/app/peers`.
Expected: a centre node in orange, peers on a ring in green (online) or grey, edges thicker where traffic is higher.

Then in DevTools → Rendering → **Emulate `prefers-reduced-motion: reduce`**, and reload.
Expected: the graph still renders a complete static frame, with no travelling pulses.

- [ ] **Step 4: Verify it stops off-screen**

Open DevTools → Performance, scroll the graph fully out of view for a few seconds.
Expected: no ongoing animation frames while off-screen.

- [ ] **Step 5: Typecheck and commit**

```bash
cd api/react-ui && npm run typecheck && cd -
git add api/react-ui/src/components/PeerGraph.tsx api/react-ui/src/pages/PeersPage.tsx
git commit -m "feat(ui): add ego peer graph with live bandwidth-weighted edges"
```

---

## Task 14: Serve the SPA from Go

**Files:**
- Create: `api/spa.go`, `api/spa_test.go`
- Modify: `api/api.go:49-58` (embed + `getFileSystem`), `api/api.go:265` (asset handler), `api/api.go:433` (static route), `main.go:16`
- Delete: `api/public/` (whole directory), `api/generate/` (whole package)

**Interfaces:**
- Consumes: a built `api/react-ui/dist` from Task 2.
- Produces: `registerUI(ec *echo.Echo) error` in package `api`.

**This is the task that breaks the build if Tasks 2 and 3 are not green.** Verify CI is passing before starting.

- [ ] **Step 1: Write the failing test at `api/spa_test.go`**

```go
package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newUIEcho(t *testing.T) *echo.Echo {
	t.Helper()
	ec := echo.New()
	if err := registerUI(ec); err != nil {
		t.Fatalf("registerUI: %v", err)
	}
	return ec
}

func TestRootRedirectsToApp(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("got %d, want 301", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/app" {
		t.Fatalf("Location = %q, want /app", got)
	}
}

func TestAppServesIndexWithNoCache(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestDeepLinkServesIndex(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/nodes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("deep link got %d, want 200", rec.Code)
	}
}

func TestMissingAssetReturns404(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/does-not-exist.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset got %d, want 404", rec.Code)
	}
}

func TestRealAssetIsImmutablyCached(t *testing.T) {
	// Asset filenames are content-hashed by Vite and therefore unknowable
	// at authoring time, so discover one from the embedded bundle.
	entries, err := fs.ReadDir(reactUI, "react-ui/dist/assets")
	if err != nil || len(entries) == 0 {
		t.Skip("no built assets embedded; run 'make react-ui-force' first")
	}
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/"+entries[0].Name(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("asset %s got %d, want 200", entries[0].Name(), rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != immutableAssetCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", cc, immutableAssetCacheControl)
	}
}

func TestUnknownAPIPathReturnsJSONNotIndex(t *testing.T) {
	ec := newUIEcho(t)
	req := httptest.NewRequest(http.MethodGet, "/api/nope", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "text/html; charset=utf-8" {
		t.Fatal("API 404 was swallowed by the SPA fallback")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./api/ -run 'TestRootRedirects|TestAppServes|TestDeepLink|TestMissingAsset|TestRealAsset|TestUnknownAPI' -v`
Expected: FAIL — `undefined: registerUI`.

- [ ] **Step 3: Create `api/spa.go`**

```go
// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package api

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed react-ui/dist/*
var reactUI embed.FS

// immutableAssetCacheControl is safe because Vite emits content-hashed
// filenames: a changed file always has a changed name.
const immutableAssetCacheControl = "public, max-age=31536000, immutable"

// registerUI wires the embedded React application into ec.
//
// It returns an error only when the embedded filesystem is unusable. The
// caller is expected to log and continue rather than abort — a binary
// built without the UI must still serve the JSON API.
func registerUI(ec *echo.Echo) error {
	uiFS, err := fs.Sub(reactUI, "react-ui/dist")
	if err != nil {
		return err
	}

	serveIndex := func(c echo.Context) error {
		index, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			return c.String(http.StatusNotFound, "React UI not built")
		}
		// The index must never be cached: it names the hashed asset
		// bundles, so a stale copy points at files that no longer exist.
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.HTMLBlob(http.StatusOK, index)
	}

	ec.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/app")
	})
	ec.GET("/app", serveIndex)
	ec.GET("/app/*", serveIndex)

	ec.GET("/assets/*", func(c echo.Context) error {
		name := "assets/" + c.Param("*")
		body, err := fs.ReadFile(uiFS, name)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		c.Response().Header().Set("Cache-Control", immutableAssetCacheControl)
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		return c.Blob(http.StatusOK, ctype, body)
	})

	// Anything else at the root that is not under /api or /debug — the
	// favicon, for instance — comes straight from the bundle.
	ec.GET("/favicon.svg", func(c echo.Context) error {
		body, err := fs.ReadFile(uiFS, "favicon.svg")
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return c.Blob(http.StatusOK, "image/svg+xml", body)
	})

	// SPA fallback. A browser navigating to a client-side route gets the
	// index; anything asking for JSON, and anything under /api, keeps the
	// real 404 so API errors are never masked by an HTML page.
	defaultHandler := ec.HTTPErrorHandler
	ec.HTTPErrorHandler = func(err error, c echo.Context) {
		he, ok := err.(*echo.HTTPError)
		if ok && he.Code == http.StatusNotFound &&
			c.Request().Method == http.MethodGet &&
			!strings.HasPrefix(c.Path(), "/api") &&
			!strings.HasPrefix(c.Request().URL.Path, "/api") &&
			!strings.HasPrefix(c.Request().URL.Path, "/debug") &&
			strings.Contains(c.Request().Header.Get("Accept"), "text/html") {
			if serveErr := serveIndex(c); serveErr == nil {
				return
			}
		}
		defaultHandler(err, c)
	}

	return nil
}
```

- [ ] **Step 4: Replace the embed block in `api/api.go`**

Delete lines 49-58 (the `//go:embed public`, `var embededFiles embed.FS`, and the whole `getFileSystem()` function). The `embed` and `io/fs` imports become unused in `api.go` — remove them from its import block, since `spa.go` now owns them.

- [ ] **Step 5: Replace the asset handler and static route in `api/api.go`**

Delete the line `assetHandler := http.FileServer(getFileSystem())` (around line 265).

Replace the static route (around line 433):

```go
	ec.GET("/*", echo.WrapHandler(http.StripPrefix("/", assetHandler)))
```

with:

```go
	if err := registerUI(ec); err != nil {
		// A binary built without the React UI must still serve the API.
		fmt.Printf("web UI not available: %v\n", err)
	}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `make react-ui-force && go test ./api/ -run 'TestRootRedirects|TestAppServes|TestDeepLink|TestMissingAsset|TestRealAsset|TestUnknownAPI' -v`
Expected: PASS, 6 tests.

- [ ] **Step 7: Delete the old UI**

```bash
git rm -r api/public api/generate
```

Then remove the `//go:generate` line at `main.go:16`:

```
//go:generate go run ./api/generate ./api/public/functions.tmpl ./api/public/index.tmpl ./api/public/index.html
```

- [ ] **Step 8: Verify the whole build and the existing suite**

Run: `go build ./... && go vet ./api/... && go test ./api/...`
Expected: all PASS. The existing `api_test.go` suite exercises the JSON API, which is unchanged.

- [ ] **Step 9: Verify end to end against a real binary**

Run: `make build && ./edgevpn -g > /tmp/ev.yaml && ./edgevpn api --config /tmp/ev.yaml --listen ':8080'`

In another terminal:
```bash
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' localhost:8080/
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/app/nodes
curl -s -I localhost:8080/app | grep -i cache-control
curl -s localhost:8080/api/summary | head -c 200
```
Expected: `301 http://localhost:8080/app`; `200` for the deep link; `Cache-Control: no-cache`; valid JSON from the API.

Then open `http://localhost:8080/app` in a browser and click through all six routes, including a hard reload on `/app/nodes` to prove the SPA fallback works.

- [ ] **Step 10: Commit**

```bash
git add api/spa.go api/spa_test.go api/api.go main.go
git commit -m "feat(api): serve the React SPA and remove the Alpine UI

Replaces the embedded Alpine.js/Tailwind-CDN page with the React bundle.
Adds SPA fallback, immutable asset caching and a no-cache index.

Removes the committed 2930-line generated index.html, the vendored
minified JS, and the api/generate template pipeline. The CDN Font
Awesome dependency goes with it, which fixes icon rendering on
air-gapped nodes."
```

---

## Task 15: Documentation and final verification

**Files:**
- Modify: `README.md`
- Create: `api/react-ui/README.md`

**Interfaces:**
- Consumes: everything.
- Produces: nothing.

- [ ] **Step 1: Create `api/react-ui/README.md`**

````markdown
# EdgeVPN web UI

React + TypeScript + Vite. Built from source and embedded into the Go
binary — **the `dist/` directory is never committed.**

## Requirements

Node **20.19 or newer**. Vite 8 does not run on Node 18.

## Development

Run the API in one terminal:

```bash
edgevpn -g > /tmp/ev.yaml
edgevpn api --config /tmp/ev.yaml --listen ':8080'
```

And the dev server in another:

```bash
cd api/react-ui
npm install
npm run dev      # http://localhost:3000/app, proxies /api to :8080
```

Point at a different backend with `EDGEVPN_URL=http://host:port npm run dev`.

## Checks

```bash
npm run typecheck   # tsc --noEmit
npm test            # vitest
npm run build       # typecheck + production build into dist/
```

## Design system

All colour, type and spacing live in `src/styles/tokens.css` as `--ev-*`
custom properties. **No component may contain a literal colour value.**
The UI is dark-only by design; there is no light theme.

`--ev-signal` is the sole accent. The semantic colours (`--ev-ok`,
`--ev-warn`, `--ev-crit`) encode state and are never decorative.

## Notes

The Go API has no `json` struct tags, so responses are **PascalCase**
(`PeerID`, `RateIn`, `BlockChain`). `src/types/api.ts` mirrors that
deliberately — do not "fix" the casing without changing the Go side,
which would break `api/client` and downstream consumers.
````

- [ ] **Step 2: Add a build section to the root `README.md`**

Insert after the installation section:

```markdown
## Building from source

The web UI is a React application compiled into the binary, so a Node
toolchain (**Node 20.19+**) is required for a full build:

```bash
make build        # builds the UI if needed, then the Go binary
make react-ui-force   # force a clean UI rebuild
```

`go build` alone works only when `api/react-ui/dist` already exists —
the UI is embedded with `//go:embed`, and a missing directory is a
compile error. For Go-only work you can stub it:

```bash
mkdir -p api/react-ui/dist && touch api/react-ui/dist/index.html
```

See `api/react-ui/README.md` for frontend development.
```

- [ ] **Step 3: Full verification from a clean tree**

```bash
git status --porcelain          # expect: clean
rm -rf api/react-ui/dist api/react-ui/node_modules
make react-ui-force
go build ./...
go vet ./...
go test ./api/...
cd api/react-ui && npm test && npm run typecheck && cd -
git status --porcelain          # expect: still clean — dist must not appear
```
Expected: every command succeeds, and the final `git status` is empty. **If `api/react-ui/dist` shows as untracked, `.gitignore` is wrong — fix it before finishing.**

- [ ] **Step 4: Confirm the old UI is fully gone**

```bash
git ls-files api/public api/generate    # expect: no output
grep -rn "go:generate" main.go          # expect: no output
grep -rn "cdnjs\|alpine\|apexcharts" --include=*.go --include=*.tmpl . # expect: no output
```

- [ ] **Step 5: Commit**

```bash
git add README.md api/react-ui/README.md
git commit -m "docs: document the React UI build and Node requirement"
```

- [ ] **Step 6: Push and open a PR**

```bash
git push -u origin feat/react-ui-design-system
```

---

## Verification checklist

Before declaring this complete, confirm each:

- [ ] `go build ./...` succeeds from a clean tree after `make react-ui-force`
- [ ] `go test ./api/...` passes
- [ ] `npm test` passes (18+ tests)
- [ ] `npm run typecheck` reports no errors
- [ ] `git status --porcelain` is empty after a build — `dist/` never appears
- [ ] All six routes render real data against a live node
- [ ] A hard reload on `/app/nodes` returns the app, not a 404
- [ ] `/` returns 301 to `/app`
- [ ] `curl -H 'Accept: application/json' localhost:8080/api/nope` returns a JSON 404, not HTML
- [ ] `/api/summary` output is byte-identical to before the change
- [ ] No requests are issued while the browser tab is hidden
- [ ] `/api/blockchain` is polled at 5.5s, everything else at 1.5s
- [ ] The peer graph renders a static frame under `prefers-reduced-motion: reduce`
- [ ] `grep -rn "#[0-9A-Fa-f]\{6\}" api/react-ui/src --include=*.tsx` returns nothing (no literal colours in components)
- [ ] `git ls-files api/public api/generate` returns nothing

---

## Out of scope — do not add

Confirmed excluded in the design spec. If any of these seem necessary mid-implementation, stop and raise it rather than adding it:

- Authentication or a login screen
- Write operations beyond the existing row deletes
- `json` struct tags on Go types
- Reverse-proxy subpath support (`<base href>` injection)
- SSE or WebSocket transport
- A `/api/version` endpoint
- Typed delete endpoints (`DELETE /api/machines/:address`)
- A light theme or theme toggle
- Playwright / e2e infrastructure
- Fixing the phantom `urfave/cli/v3` dependency in `go.mod:32`
- Fixing `fmt.Printf(string(priv))` in `cmd/peergate.go`
