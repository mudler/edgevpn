# EdgeVPN web UI

React + TypeScript + Vite. Built from source and embedded into the Go
binary — **the `dist/` directory is never committed.**

## Requirements

Node **20.19 or newer** (see `.nvmrc`). Vite 8 does not run on Node 18.

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
npm run dev
```

Vite prints `http://localhost:3000/`, but the router uses a hardcoded
`/app` basename, so **open <http://localhost:3000/app>** — the bare root
matches no route. `/api` and `/debug` are proxied to the backend.

Point at a different backend with `EDGEVPN_URL=http://host:port npm run dev`.

## Checks

```bash
npm run typecheck   # tsc --noEmit
npm test            # vitest run
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
