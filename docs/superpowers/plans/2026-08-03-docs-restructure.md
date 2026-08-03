# EdgeVPN Documentation Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure EdgeVPN's documentation into a Diátaxis information architecture, generate the CLI and environment-variable reference from the `cli.App` with a CI drift gate, and write the pages for features that are currently invisible.

**Architecture:** A new `docs/generate` package walks the real `cli.App` — shared with `main.go` via a new `cmd.NewApp()` so it cannot drift — and emits Hugo pages into `docs/content/en/docs/reference/`. Content moves into four Diátaxis sections with Hugo `aliases:` preserving every old URL. Docsy stays; the custom theme is a separate sub-project.

**Tech Stack:** Hugo (extended, 0.152.2 in CI) + Docsy via Hugo Modules, Go 1.26, urfave/cli v2.

## Global Constraints

- **Spec:** `docs/superpowers/specs/2026-08-03-docs-restructure-design.md`. Read it before starting.
- **The CLI is urfave/cli v2.** `go.mod:32` also declares `v3` as a direct dependency, but **no `.go` file imports it**. Never write docs or code against v3.
- **Every command shown in a page must be executed, or explicitly marked unverified in the implementation report.** Undocumented-but-untested command lines are exactly what produced the `--peerguardian` bug. A command needing two hosts or root is marked as such — never silently trusted.
- **Every moved page keeps its old URL** via a Hugo `aliases:` front-matter entry. The site is linked from the README, from Kairos, and from search results.
- **Generated files are never hand-edited.** They carry a banner saying so.
- **Weights must be unique within a section.** The current collisions are a defect being fixed, not a pattern to copy.
- **Do not fix the licence inconsistency.** `LICENSE` is Apache-2.0, the README badge says GPL3, the footer says Apache v2, the CLI banner is GPL-flavoured, ~10 source files carry GPL-2 headers. This is the maintainer's legal call. Flag it; never silently pick one.
- **Out of scope, do not touch:** the `urfave/cli/v3` phantom dep, `cmd/peergate.go`'s `go vet` failure, the echo path-param unescape bug, the custom Hugo theme.
- Branch: `feat/docs-restructure`. Commit after every task.
- **Working-tree hazard — never run a bare `git add -A` or `git add .`.** Two things are loose in the tree and must stay out of every commit:
  1. `api/react-ui/` is untracked here. It belongs to the sibling branch `feat/react-ui-design-system` (a whole React application) and is only present because the working tree was reused. Committing it onto this branch would drag an unrelated feature into a docs PR.
  2. `docs/package.json` and `docs/package-lock.json` are **tracked and simultaneously gitignored**, and `docs/scripts/build.sh` runs `npm install --save`, which rewrites them on **every docs build**. So `cd docs && make build` dirties the tree as a side effect. After building, restore them: `git checkout -- docs/package.json docs/package-lock.json`. Never commit those modifications — they are build noise, and fixing the underlying packaging problem is explicitly out of scope.

  Always `git add` explicit paths.
- **Hugo build baseline: ZERO errors.** Measured on this repo at branch point with the pinned toolchain (`docs/Makefile` → Hugo 0.152.2 extended): the build succeeds, emitting 29 pages, 0 aliases, and exactly three deprecation **warnings** (`params.algolia_docsearch`, the GA4/UA notice, `footer_about_disable`). Any *error* you see is one you introduced.
  - An earlier report claimed a 43-error baseline. That was measured on Hugo **0.146.3**, a version this project does not use. Ignore it; do not reintroduce it as a target.
  - The three deprecation warnings are pre-existing. Two are addressed incidentally by Task 4 (the GA4 one, when the placeholder analytics ID goes). Do not chase the others.
  - **`docs/themes/docsy` is an initialised submodule on this checkout** even though `config.toml` sets no `theme=`. Task 4 removes it; confirm the build still succeeds afterwards rather than assuming.

---

## File Structure

**Created:**

| Path | Responsibility |
|---|---|
| `cmd/app.go` | `NewApp(version string) *cli.App` — the single definition of the CLI, used by `main.go` and the generator. |
| `docs/generate/main.go` | Walks the app, emits reference pages. |
| `docs/generate/render.go` | Markdown/front-matter rendering, separated so it is unit-testable without file I/O. |
| `docs/generate/render_test.go` | Tests for the rendering logic. |
| `docs/content/en/docs/{tutorials,how-to,reference,explanation}/_index.md` | Section landing pages. |
| `CONTRIBUTING.md` | Repo root. The contributing page currently links to a 404. |

**Modified:** `main.go`, `Makefile`, `.github/workflows/pages.yml`, `docs/config.toml`, `.gitmodules`, `README.md`, and every page under `docs/content/en/docs/`.

**Deleted:** `docs/themes/docsy` submodule, `docs/content/en/community/_index.md`.

---

## Task 1: Share the CLI definition between main.go and the generator

**Files:**
- Create: `cmd/app.go`
- Modify: `main.go:29-52`
- Test: `cmd/app_test.go`

**Interfaces:**
- Consumes: existing exported `cmd.MainFlags()`, `cmd.CommonFlags`, `cmd.Start()`, `cmd.API()`, `cmd.ServiceAdd()`, `cmd.ServiceConnect()`, `cmd.FileReceive()`, `cmd.Proxy()`, `cmd.FileSend()`, `cmd.DNS()`, `cmd.Peergate()`, `cmd.Main()`, `cmd.Copyright`.
- Produces: `cmd.NewApp(version string) *cli.App`. Task 3's generator depends on this exact signature.

**Why:** if the generator built its own `cli.App`, it could drift from the real one and the CI gate would be verifying a copy. Both must read the same definition.

- [ ] **Step 1: Write the failing test at `cmd/app_test.go`**

```go
package cmd_test

import (
	"testing"

	"github.com/mudler/edgevpn/cmd"
)

func TestNewAppHasAllCommands(t *testing.T) {
	app := cmd.NewApp("v0.0.0-test")
	want := []string{"start", "api", "service-add", "service-connect", "file-receive", "proxy", "file-send", "dns", "peergater"}
	got := map[string]bool{}
	for _, c := range app.Commands {
		got[c.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("command %q missing from NewApp", name)
		}
	}
	if len(app.Commands) != len(want) {
		t.Errorf("got %d commands, want %d", len(app.Commands), len(want))
	}
}

func TestNewAppHasRootFlags(t *testing.T) {
	app := cmd.NewApp("v0.0.0-test")
	if len(app.Flags) == 0 {
		t.Fatal("NewApp has no root flags")
	}
	// The root flag set must include both the root-only flags and the
	// common flags; --api is root-only, --token is common.
	names := map[string]bool{}
	for _, f := range app.Flags {
		for _, n := range f.Names() {
			names[n] = true
		}
	}
	for _, want := range []string{"api", "token", "peerguard"} {
		if !names[want] {
			t.Errorf("root flag %q missing", want)
		}
	}
}

func TestNewAppVersionIsWired(t *testing.T) {
	if got := cmd.NewApp("v1.2.3").Version; got != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/ -run TestNewApp -v`
Expected: FAIL — `undefined: cmd.NewApp`.

- [ ] **Step 3: Create `cmd/app.go`**

Copy the exact app literal currently in `main.go`. Do not change any field.

```go
package cmd

import (
	"github.com/urfave/cli/v2"
)

// NewApp builds the EdgeVPN CLI.
//
// It is the single definition of the command-line surface: main.go runs it,
// and docs/generate walks it to produce the reference documentation. Keeping
// one definition is what makes the generated docs trustworthy — a second copy
// would let the docs drift from the binary while still passing their own
// drift check.
func NewApp(version string) *cli.App {
	return &cli.App{
		Name:        "edgevpn",
		Version:     version,
		Authors:     []*cli.Author{{Name: "Ettore Di Giacinto"}},
		Usage:       "edgevpn --config /etc/edgevpn/config.yaml",
		Description: "edgevpn uses libp2p to build an immutable trusted blockchain addressable p2p network",
		Copyright:   Copyright,
		Flags:       MainFlags(),
		Commands: []*cli.Command{
			Start(),
			API(),
			ServiceAdd(),
			ServiceConnect(),
			FileReceive(),
			Proxy(),
			FileSend(),
			DNS(),
			Peergate(),
		},
		Action: Main(),
	}
}
```

- [ ] **Step 4: Rewrite `main.go` to use it**

Replace the whole `app := &cli.App{...}` literal with:

```go
	app := cmd.NewApp(internal.Version)
```

Remove the now-unused `"github.com/urfave/cli/v2"` import from `main.go` if nothing else there needs it. Keep the `//go:generate`-free state — do not reintroduce one.

- [ ] **Step 5: Run the tests**

Run: `go test ./cmd/ -run TestNewApp -v && go build ./... && go vet ./cmd/ .`
Expected: 3 tests PASS, build clean. Note `go vet ./cmd/` will still report the two pre-existing `cmd/peergate.go` non-constant format string errors — that is expected and out of scope.

- [ ] **Step 6: Verify the binary is unchanged in behaviour**

Run: `go build -o /tmp/ev-check . && /tmp/ev-check --help | head -30`
Expected: identical help output to before (same commands, same flags). Compare against `git stash && go build -o /tmp/ev-before . && git stash pop` if you want a byte comparison.

- [ ] **Step 7: Commit**

```bash
git add cmd/app.go cmd/app_test.go main.go
git commit -m "refactor(cmd): extract NewApp so docs can walk the real CLI"
```

---

## Task 2: Section skeleton

**Files:**
- Create: `docs/content/en/docs/tutorials/_index.md`, `docs/content/en/docs/how-to/_index.md`, `docs/content/en/docs/reference/_index.md`, `docs/content/en/docs/explanation/_index.md`
- Modify: `docs/content/en/docs/_index.md`

**Interfaces:**
- Produces: the four section directories every later task writes into, with the weights they must respect: tutorials 10, how-to 20, reference 30, explanation 40.

- [ ] **Step 1: Create `docs/content/en/docs/tutorials/_index.md`**

```markdown
---
title: "Tutorials"
linkTitle: "Tutorials"
weight: 10
description: >
  Start here. End-to-end walkthroughs that take you from nothing to a working network.
---

Tutorials are learning-oriented: each one takes you all the way through a task
and tells you exactly what to type. If you already know what you want and need
the specifics, the [how-to guides](../how-to/) are shorter.
```

- [ ] **Step 2: Create `docs/content/en/docs/how-to/_index.md`**

```markdown
---
title: "How-to guides"
linkTitle: "How-to"
weight: 20
description: >
  Task-oriented recipes for a specific job — expose a service, run an exit node, lock a network down.
---

Each guide assumes you already have a working network. If you don't, start with
[your first network](../tutorials/your-first-network/).
```

- [ ] **Step 3: Create `docs/content/en/docs/reference/_index.md`**

```markdown
---
title: "Reference"
linkTitle: "Reference"
weight: 30
description: >
  Every command, flag, environment variable and API endpoint.
---

The [CLI reference](cli/) and [environment variables](environment-variables/)
pages are generated directly from the source, so they cannot drift from the
binary you are running.
```

- [ ] **Step 4: Create `docs/content/en/docs/explanation/_index.md`**

```markdown
---
title: "Explanation"
linkTitle: "Explanation"
weight: 40
description: >
  How EdgeVPN works and why it is built this way — architecture, the ledger, and the security model.
---

Background reading. Nothing here is required to use EdgeVPN, but the
[security model](security-model/) is worth reading before you deploy it.
```

- [ ] **Step 5: Rewrite `docs/content/en/docs/_index.md`**

Keep its existing front matter (`title`, `linkTitle`, `weight: 20`, `menu.main.weight: 20`) exactly as-is — changing the menu weight moves the top nav. Replace the body with a short "What is EdgeVPN" orientation plus links to the four sections. Draw the description from `README.md`'s opening paragraphs, which are accurate.

- [ ] **Step 6: Verify the build**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'`
Expected: `0`. The docs build clean on Hugo 0.152.2; any error you see is yours. If it increased, you introduced an error — find it before continuing.

- [ ] **Step 7: Commit**

```bash
git add docs/content/en/docs/
git commit -m "docs: add Diataxis section skeleton"
```

---

## Task 3: The reference generator

**Files:**
- Create: `docs/generate/main.go`, `docs/generate/render.go`, `docs/generate/render_test.go`
- Modify: `Makefile`

**Interfaces:**
- Consumes: `cmd.NewApp(version string) *cli.App` from Task 1.
- Produces: `make docs-gen`; generated pages under `docs/content/en/docs/reference/cli/` and `docs/content/en/docs/reference/environment-variables.md`. Task 4's CI gate depends on `make docs-gen` being idempotent.

**Key API:** flags implement `cli.DocGenerationFlag` (`urfave/cli/v2@v2.27.7/flag.go:130-154`) exposing `TakesValue() bool`, `GetUsage() string`, `GetValue() string`, `GetDefaultText() string`, `GetEnvVars() []string`, `IsVisible() bool`. Type-assert to it; skip flags where the assertion fails or `IsVisible()` is false.

**Do NOT use `app.ToMarkdown()`.** It exists (`docs.go:19`) but emits one blob with no front matter, no per-command splitting, and no env-var column.

- [ ] **Step 1: Write the failing test at `docs/generate/render_test.go`**

```go
package main

import (
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestRenderFlagTableIncludesEnvVars(t *testing.T) {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "token",
			Usage:   "Specify an edgevpn token in place of a config file",
			EnvVars: []string{"EDGEVPNTOKEN"},
		},
		&cli.BoolFlag{
			Name:    "peerguard",
			Usage:   "Enable peerguard. (Experimental)",
			EnvVars: []string{"PEERGUARD"},
		},
	}
	out := renderFlagTable(flags)

	for _, want := range []string{"--token", "EDGEVPNTOKEN", "--peerguard", "PEERGUARD", "Enable peerguard"} {
		if !strings.Contains(out, want) {
			t.Errorf("flag table missing %q\n%s", want, out)
		}
	}
}

func TestRenderFlagTableSkipsHiddenFlags(t *testing.T) {
	flags := []cli.Flag{
		&cli.StringFlag{Name: "visible", Usage: "shown"},
		&cli.StringFlag{Name: "secret", Usage: "hidden", Hidden: true},
	}
	out := renderFlagTable(flags)
	if strings.Contains(out, "secret") {
		t.Errorf("hidden flag leaked into the table:\n%s", out)
	}
	if !strings.Contains(out, "visible") {
		t.Errorf("visible flag missing:\n%s", out)
	}
}

func TestRenderFlagTableEscapesPipes(t *testing.T) {
	// Several real usage strings contain "|" (e.g. the ownership flag lists
	// "enforce | observe | off"), which would split the row into extra
	// markdown columns.
	flags := []cli.Flag{
		&cli.StringFlag{Name: "ownership", Usage: "enforce | observe | off", Value: "enforce"},
	}
	out := renderFlagTable(flags)

	var row string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "ownership") {
			row = line
		}
	}
	if row == "" {
		t.Fatal("no row rendered for the ownership flag")
	}
	if !strings.Contains(row, `enforce \| observe \| off`) {
		t.Errorf("pipes in usage were not escaped: %q", row)
	}
	// A well-formed 4-column row has exactly 5 structural pipes; every other
	// pipe must be escaped.
	if got := strings.Count(row, "|") - strings.Count(row, `\|`); got != 5 {
		t.Errorf("row has %d structural pipes, want 5 (4 columns): %q", got, row)
	}
}

func TestRenderPageHasFrontMatterAndBanner(t *testing.T) {
	out := renderCommandPage(&cli.Command{
		Name:        "proxy",
		Usage:       "Starts a local http proxy server",
		Aliases:     []string{},
		Description: "Routes traffic through the p2p network",
		Flags:       []cli.Flag{&cli.StringFlag{Name: "listen", Usage: "Listen address"}},
	}, 10)

	if !strings.HasPrefix(out, "---\n") {
		t.Error("page does not start with front matter")
	}
	for _, want := range []string{`title: "proxy"`, "weight: 10", "Do not edit", "--listen"} {
		if !strings.Contains(out, want) {
			t.Errorf("page missing %q\n%s", want, out)
		}
	}
}

func TestRenderEnvVarPageMapsBackToFlags(t *testing.T) {
	out := renderEnvVarPage(map[string][]envBinding{
		"EDGEVPNTOKEN": {{Flag: "--token", Command: "global", Default: ""}},
		"PROXYLISTEN":  {{Flag: "--listen", Command: "proxy", Default: ":8080"}},
	})
	for _, want := range []string{"EDGEVPNTOKEN", "--token", "PROXYLISTEN", "proxy", ":8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("env var page missing %q\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/docsgen/ -v`
Expected: FAIL — undefined `renderFlagTable`, `renderCommandPage`, `renderEnvVarPage`, `envBinding`.

Note: `docs/` has its own `go.mod` (`github.com/mudler/edgevpn/docs`) for the Hugo module. The generator must be part of the **root** module so it can import `cmd`. Verify `docs/go.mod` does not shadow it — if `go test ./docs/generate/` fails with a module error, place the generator at `docs/generate/` but confirm the root `go.mod` covers it; if the nested `go.mod` interferes, move the generator to `internal/docsgen/` and adjust all paths in this task. Report which you did.

- [ ] **Step 3: Create `docs/generate/render.go`**

```go
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
)

const banner = "<!-- Generated by docs/generate. Do not edit; run `make docs-gen`. -->"

// envBinding records one place an environment variable is read from.
type envBinding struct {
	Flag    string
	Command string
	Default string
}

// escapeCell makes a string safe inside a markdown table cell. Several real
// usage strings contain "|" (the ownership flag lists its modes that way),
// which would otherwise split the row into extra columns.
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func flagNames(f cli.Flag) string {
	names := f.Names()
	out := make([]string, 0, len(names))
	for _, n := range names {
		if len(n) == 1 {
			out = append(out, "`-"+n+"`")
		} else {
			out = append(out, "`--"+n+"`")
		}
	}
	return strings.Join(out, ", ")
}

// renderFlagTable renders a markdown table for the visible flags.
func renderFlagTable(flags []cli.Flag) string {
	var b strings.Builder
	b.WriteString("| Flag | Default | Environment | Description |\n")
	b.WriteString("|---|---|---|---|\n")

	rows := 0
	for _, f := range flags {
		df, ok := f.(cli.DocGenerationFlag)
		if !ok || !df.IsVisible() {
			continue
		}
		def := df.GetDefaultText()
		if def == "" {
			def = df.GetValue()
		}
		if def == "" {
			def = "—"
		} else {
			def = "`" + escapeCell(def) + "`"
		}

		env := "—"
		if vars := df.GetEnvVars(); len(vars) > 0 {
			quoted := make([]string, len(vars))
			for i, v := range vars {
				quoted[i] = "`" + v + "`"
			}
			env = strings.Join(quoted, ", ")
		}

		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			flagNames(f), def, env, escapeCell(df.GetUsage()))
		rows++
	}
	if rows == 0 {
		return "_This command takes no flags of its own._\n"
	}
	return b.String()
}

// renderCommandPage renders one Hugo page for a command.
func renderCommandPage(c *cli.Command, weight int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: %q\nlinkTitle: %q\nweight: %d\ndescription: >\n  %s\n---\n\n",
		c.Name, c.Name, weight, escapeCell(c.Usage))
	b.WriteString(banner + "\n\n")

	if len(c.Aliases) > 0 {
		quoted := make([]string, len(c.Aliases))
		for i, a := range c.Aliases {
			quoted[i] = "`" + a + "`"
		}
		fmt.Fprintf(&b, "Aliases: %s\n\n", strings.Join(quoted, ", "))
	}
	if c.Description != "" && c.Description != c.Usage {
		fmt.Fprintf(&b, "%s\n\n", c.Description)
	}

	fmt.Fprintf(&b, "```\nedgevpn %s [options]\n```\n\n", c.Name)
	b.WriteString("## Flags\n\n")
	b.WriteString(renderFlagTable(c.Flags))

	for _, sub := range c.Subcommands {
		fmt.Fprintf(&b, "\n## `%s %s`\n\n", c.Name, sub.Name)
		if sub.Usage != "" {
			fmt.Fprintf(&b, "%s\n\n", sub.Usage)
		}
		b.WriteString(renderFlagTable(sub.Flags))
	}
	return b.String()
}

// renderEnvVarPage renders the environment-variable cross-reference.
func renderEnvVarPage(bindings map[string][]envBinding) string {
	var b strings.Builder
	b.WriteString("---\ntitle: \"Environment variables\"\nlinkTitle: \"Environment variables\"\nweight: 20\ndescription: >\n  Every environment variable EdgeVPN reads, and the flag it corresponds to.\n---\n\n")
	b.WriteString(banner + "\n\n")
	b.WriteString("Environment variables are read when the corresponding flag is not passed.\n\n")
	b.WriteString("| Variable | Flag | Command | Default |\n|---|---|---|---|\n")

	names := make([]string, 0, len(bindings))
	for k := range bindings {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, bind := range bindings[name] {
			def := bind.Default
			if def == "" {
				def = "—"
			} else {
				def = "`" + escapeCell(def) + "`"
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n",
				name, bind.Flag, bind.Command, def)
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/docsgen/ -v`
Expected: 5 tests PASS.

- [ ] **Step 5: Create `docs/generate/main.go`**

```go
// Command generate emits the CLI and environment-variable reference
// documentation from the real cli.App, so the docs cannot drift from the
// binary. Run via `make docs-gen`.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/mudler/edgevpn/cmd"
)

const outDir = "docs/content/en/docs/reference"

func main() {
	// The version is irrelevant to the generated output and must be fixed,
	// otherwise the CI drift check would fail on every release.
	app := cmd.NewApp("")

	if err := run(app); err != nil {
		fmt.Fprintln(os.Stderr, "docs generate:", err)
		os.Exit(1)
	}
}

func run(app *cli.App) error {
	cliDir := filepath.Join(outDir, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		return err
	}

	// Remove previously generated command pages so a deleted command does
	// not leave a stale page behind — the drift gate would never catch it.
	existing, err := filepath.Glob(filepath.Join(cliDir, "*.md"))
	if err != nil {
		return err
	}
	for _, p := range existing {
		if err := os.Remove(p); err != nil {
			return err
		}
	}

	bindings := map[string][]envBinding{}
	collectEnv(bindings, app.Flags, "global")

	// Index page, carrying the root flag table.
	var idx strings.Builder
	idx.WriteString("---\ntitle: \"CLI\"\nlinkTitle: \"CLI\"\nweight: 10\ndescription: >\n  Every EdgeVPN command and flag.\n---\n\n")
	idx.WriteString(banner + "\n\n")
	idx.WriteString("Running `edgevpn` with no subcommand starts the VPN.\n\n## Global flags\n\n")
	idx.WriteString(renderFlagTable(app.Flags))
	idx.WriteString("\n## Commands\n\n")
	for _, c := range app.Commands {
		fmt.Fprintf(&idx, "- [`%s`](%s/) — %s\n", c.Name, c.Name, escapeCell(c.Usage))
	}
	if err := os.WriteFile(filepath.Join(cliDir, "_index.md"), []byte(idx.String()), 0o644); err != nil {
		return err
	}

	for i, c := range app.Commands {
		page := renderCommandPage(c, (i+1)*10)
		path := filepath.Join(cliDir, c.Name+".md")
		if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
			return err
		}
		collectEnv(bindings, c.Flags, c.Name)
		for _, sub := range c.Subcommands {
			collectEnv(bindings, sub.Flags, c.Name+" "+sub.Name)
		}
	}

	return os.WriteFile(
		filepath.Join(outDir, "environment-variables.md"),
		[]byte(renderEnvVarPage(bindings)), 0o644)
}

func collectEnv(into map[string][]envBinding, flags []cli.Flag, command string) {
	for _, f := range flags {
		df, ok := f.(cli.DocGenerationFlag)
		if !ok || !df.IsVisible() {
			continue
		}
		names := f.Names()
		if len(names) == 0 {
			continue
		}
		def := df.GetDefaultText()
		if def == "" {
			def = df.GetValue()
		}
		for _, env := range df.GetEnvVars() {
			into[env] = append(into[env], envBinding{
				Flag:    "--" + names[0],
				Command: command,
				Default: def,
			})
		}
	}
}
```

- [ ] **Step 6: Add the Makefile target**

Add to `Makefile`, and add `docs-gen` to the `.PHONY` line:

```make
docs-gen:
	go run ./docs/generate
```

- [ ] **Step 7: Generate and inspect**

Run: `make docs-gen && ls docs/content/en/docs/reference/cli/ && head -30 docs/content/en/docs/reference/cli/proxy.md`
Expected: `_index.md` plus one page per command (`start`, `api`, `service-add`, `service-connect`, `file-receive`, `proxy`, `file-send`, `dns`, `peergater`). The `proxy.md` page shows front matter, the banner, and a flag table.

- [ ] **Step 8: Verify idempotency — this is what the CI gate depends on**

Run: `make docs-gen && git add -A docs/content/en/docs/reference && make docs-gen && git diff --exit-code docs/content/en/docs/reference && echo IDEMPOTENT`
Expected: prints `IDEMPOTENT`. If it does not, something in the output is nondeterministic (map iteration order is the usual culprit) — fix it now, or the CI gate will fail randomly.

- [ ] **Step 9: Sanity-check coverage against the source**

Run:
```bash
grep -c 'Name:' cmd/util.go
grep -o '`--[a-z-]*`' docs/content/en/docs/reference/cli/_index.md | sort -u | wc -l
grep -c '^| `' docs/content/en/docs/reference/environment-variables.md
```
Expected: the global flag table covers the bulk of the 68 flags in `cmd/util.go` plus the 18 root-only ones, and the env table has dozens of rows. Exact numbers will differ (some flags share names, some have no env var) — the point is that the counts are in the right order of magnitude, not zero. Record the real numbers in your report.

- [ ] **Step 10: Verify the Hugo build**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'`
Expected: zero errors. The build is clean on Hugo 0.152.2 — any error is yours.

- [ ] **Step 11: Commit**

```bash
git add docs/generate Makefile docs/content/en/docs/reference
git commit -m "docs: generate the CLI and environment variable reference"
```

---

## Task 4: CI drift gate and docs infrastructure

**Files:**
- Modify: `.github/workflows/pages.yml`, `docs/config.toml`, `.gitmodules`
- Create: `CONTRIBUTING.md`, `.github/workflows/docs-gen.yml` (or a job added to an existing workflow)
- Delete: `docs/themes/docsy` submodule, `docs/content/en/community/_index.md`

**Interfaces:**
- Consumes: `make docs-gen` from Task 3.

- [ ] **Step 1: Add the drift gate**

Create `.github/workflows/docs-gen.yml`:

```yaml
name: Docs reference drift

on:
  push:
  pull_request:

jobs:
  docs-gen:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v7
      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: 1.26
      - name: Regenerate the reference
        run: make docs-gen
      - name: Fail if the generated reference is stale
        run: |
          if ! git diff --exit-code docs/content/en/docs/reference/; then
            echo "::error::The generated CLI reference is out of date. Run 'make docs-gen' and commit the result."
            exit 1
          fi
```

- [ ] **Step 2: Verify the gate actually catches drift**

Prove it locally rather than trusting it:

```bash
# Add a throwaway flag, regenerate, and confirm a diff appears
git stash list
sed -i 's|^var CommonFlags \[\]cli.Flag = \[\]cli.Flag{|&\n\t\&cli.BoolFlag{Name: "drift-canary", Usage: "temporary"},|' cmd/util.go
make docs-gen
git diff --stat docs/content/en/docs/reference/ | tail -1   # expect: changes
git checkout cmd/util.go && make docs-gen
git diff --exit-code docs/content/en/docs/reference/ && echo "RESTORED CLEAN"
```
Expected: the canary produces a diff, and reverting restores a clean tree. If adding a flag produces **no** diff, the generator is not reading what you think it is — stop and fix it.

- [ ] **Step 3: Add a PR trigger to the docs build**

In `.github/workflows/pages.yml`, extend the trigger so docs breakage is caught before merge rather than after. Keep the existing deploy behaviour limited to `master`:

```yaml
on:
  push:
    branches:
      - master
  pull_request:
    paths:
      - 'docs/**'
```

Then guard the deploy step so it only runs on `master` — add `if: github.ref == 'refs/heads/master' && github.event_name == 'push'` to the `JamesIves/github-pages-deploy-action` step. **A pull request must build the docs but must not deploy them.**

- [ ] **Step 4: Remove the unused docsy submodule**

`docs/config.toml` sets no `theme=` (verified: 0 matches for `^theme`); Docsy comes from Hugo Modules. The submodule is dead weight that Dependabot keeps bumping.

```bash
git submodule deinit -f docs/themes/docsy
git rm -f docs/themes/docsy
rm -rf .git/modules/docs/themes/docsy
```
Then remove the `docs/themes/docsy` entry from `.gitmodules` (delete the whole three-line stanza). If `.gitmodules` becomes empty, delete the file.

- [ ] **Step 5: Verify the docs still build without the submodule**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'`
Expected: still the baseline count. **If the build now fails differently, the submodule was load-bearing after all — restore it and report that.**

- [ ] **Step 6: Create `CONTRIBUTING.md`**

`docs/content/en/docs/contribution-guidelines.md` links to
`https://github.com/mudler/edgevpn/blob/master/CONTRIBUTING.md`, which 404s.
Write a short root `CONTRIBUTING.md` covering: how to build (referencing the
Node requirement from the React UI work), how to run tests, the `make docs-gen`
requirement when adding a flag, and how to open an issue or PR. Keep it brief
and link to the docs site rather than duplicating it.

- [ ] **Step 7: Fix the remaining config defects**

In `docs/config.toml`:
- Set `breadcrumb_disable = false` (the tree is three levels deep).
- Remove the placeholder `UA-00000000-0` analytics ID, or the `[services.googleAnalytics]` block entirely. Leaving `params.ui.feedback.enable = true` pointed at a dead property sends feedback events nowhere.
- Reconcile `baseURL` with `docs/scripts/build.sh`'s `-b` flag. `config.toml` says `https://mudler.github.io/edgevpn/docs/`; the script passes `https://mudler.github.io/edgevpn`. Make them agree — prefer the script's value, since that is what production actually serves, and record which you changed.

- [ ] **Step 8: Delete the empty community page**

`docs/content/en/community/_index.md` is an unfilled Docsy template shell that still occupies a main-nav slot.

```bash
git rm docs/content/en/community/_index.md
```
Then remove its `[[menu.main]]` entry from `docs/config.toml` if one exists, so the nav does not link to a 404.

- [ ] **Step 9: Verify**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` and confirm no increase; then confirm the generated nav has no dead community link by grepping the output HTML: `grep -ri 'community' docs/public/index.html | head -3` (expect nothing, or only unrelated prose).

- [ ] **Step 10: Commit**

```bash
git add -A .github CONTRIBUTING.md docs/config.toml .gitmodules docs/content
git commit -m "ci: gate the generated reference on drift; fix docs infrastructure"
```

---

## Task 5: Move existing pages into the new tree

**Files:**
- Move (with `git mv`): every page listed in the table below
- Delete: `docs/content/en/docs/Concepts/`, `docs/content/en/docs/Getting started/` (once empty)

**Interfaces:**
- Consumes: the section skeleton from Task 2.
- Produces: the page paths every later task links to.

**Every moved page MUST gain an `aliases:` front-matter entry for its old URL.** The site is linked from the README, from Kairos, and from search results.

| From | To | Alias to add |
|---|---|---|
| `Getting started/_index.md` | `tutorials/your-first-network.md` | `/docs/getting-started/` |
| `Getting started/cli.md` | `how-to/run-as-a-vpn.md` | `/docs/getting-started/cli/` |
| `Getting started/api.md` | `reference/api.md` | `/docs/getting-started/api/` |
| `Getting started/gui.md` | `tools/desktop-gui.md` | `/docs/getting-started/gui/` |
| `Concepts/Overview/dns.md` | `how-to/enable-dns.md` | `/docs/concepts/overview/dns/` |
| `Concepts/Overview/files.md` | `how-to/send-and-receive-files.md` | `/docs/concepts/overview/files/` |
| `Concepts/Overview/services.md` | `how-to/tunnel-tcp-services.md` | `/docs/concepts/overview/services/` |
| `Concepts/Overview/peerguardian.md` | `how-to/trusted-networks.md` | `/docs/concepts/overview/peerguardian/` |
| `Concepts/Overview/_index.md` | `explanation/the-ledger.md` | `/docs/concepts/overview/` |
| `Concepts/Architecture/_index.md` | `explanation/architecture.md` | `/docs/concepts/architecture/` |
| `Concepts/Token/_index.md` | `reference/network-config.md` | `/docs/concepts/token/` |
| `contribution-guidelines.md` | `contributing.md` | `/docs/contribution-guidelines/` |

- [ ] **Step 1: Confirm the real alias URLs before writing them**

Do not guess. Build the current site and read the actual paths:

```bash
cd docs && make build >/dev/null 2>&1
find public -name index.html | sed 's|^public||;s|index.html$||' | sort
```
Record the real URLs and use those in the `aliases:` entries. Hugo lowercases and replaces spaces, so `Getting started` becomes `getting-started`, but **verify rather than assume**.

- [ ] **Step 2: Move each page with `git mv`**

Use `git mv` (not copy-then-delete) so history follows the file. Example:

```bash
mkdir -p docs/content/en/docs/{tutorials,how-to,reference,explanation,tools}
git mv "docs/content/en/docs/Getting started/cli.md" docs/content/en/docs/how-to/run-as-a-vpn.md
```

- [ ] **Step 3: Update front matter on every moved page**

For each, set a unique `weight` within its new section, update `title`/`linkTitle` where the new name differs, and add the alias. Example for `how-to/run-as-a-vpn.md`:

```yaml
---
title: "Run as a VPN"
linkTitle: "Run as a VPN"
weight: 10
aliases:
  - /docs/getting-started/cli/
description: >
  Join a network as a VPN peer, with automatic or static addressing.
---
```

Assign weights in the order the pages appear in the spec's §5 tree, in tens (10, 20, 30…). **No two pages in a section may share a weight** — the old tree had `Getting started/{_index,api,cli}.md` all at `weight: 1`.

- [ ] **Step 3b: Split `run-as-a-vpn.md` into its three topics**

The old `Getting started/cli.md` covers three separate jobs in one page, which
is why it reads as a grab-bag. Split it:

- **`how-to/run-as-a-vpn.md`** keeps joining a network and the basic VPN flow.
- **`how-to/addressing-and-dhcp.md`** (new file) takes the DHCP section plus
  `--address`, and adds `--router` and `--static-peertable`, which are
  undocumented today. Read `pkg/vpn/vpn.go` (the `--router` behaviour is
  packet-to-single-node routing) and `cmd/util.go` before writing the additions.
- **`how-to/ipv6.md`** (new file) takes the IPv6 section. It currently links
  issue #15 and calls IPv6 "very experimental, highly unstable" — keep the
  caveat, but check whether the issue is still open and say what is actually
  true today rather than copying a claim from an unknown date.

Both new files need their own unique `weight` and `description`. The alias for
`/docs/getting-started/cli/` stays on `run-as-a-vpn.md` — it is the page that
inherits the old URL's primary topic.

Also drop the stale version claims while you are here: the page says
"Automatic IP negotiation is available since version `0.8.1`" and pins sample
output to `Version: v0.8.4`. Either update them against `git tag` or remove the
version qualifier — do not leave a claim you have not checked.

- [ ] **Step 3c: Create `tutorials/share-a-service.md`**

Reframe `how-to/tunnel-tcp-services.md` as a beginner walkthrough: two hosts,
one exposing a TCP service, one connecting to it, with every command shown in
order and the expected output. The how-to page stays as the terse reference for
someone who already knows the shape. Cross-link them both ways.

If, on reading the existing services page, a separate tutorial would be pure
duplication rather than a genuinely gentler path, say so in your report and
create it as a stub under Task 12's stub policy instead of padding.

- [ ] **Step 4: Remove the empty old directories**

```bash
rmdir "docs/content/en/docs/Getting started" docs/content/en/docs/Concepts/Overview docs/content/en/docs/Concepts/Architecture docs/content/en/docs/Concepts/Token docs/content/en/docs/Concepts 2>/dev/null
git status --porcelain docs/content
```
Expected: only the intended renames.

- [ ] **Step 4c: Fix the marketing homepage's links**

`docs/content/en/_index.html` (the site's front page, not the docs index)
links at lines 74 and 77 to `{{< relref "/docs">}}/getting-started/api/` and
`.../gui/`. The path segment sits *outside* the shortcode, so Hugo does not
error on it — it silently produces a 404 once `Getting started/` moves.

Update both to the new locations (`reference/api/` and `tools/desktop-gui/`).
Then grep the whole content tree for the same pattern, since anything built
this way is invisible to Hugo's link checking:

```bash
grep -rn 'relref' docs/content/
```

- [ ] **Step 5: Fix internal links broken by the moves**

```bash
grep -rn '](/docs/\|](\.\./\|](\./' docs/content/en/docs/ | grep -v aliases
```
Fix every link that now points at a moved page. Also fix the known dangling link in the old `Concepts/Token/_index.md` (now `reference/network-config.md`): it ends with `See [the Architecture section]()` — an empty target. Point it at `../../explanation/architecture/`.

- [ ] **Step 6: Verify aliases actually work**

Run: `cd docs && make build >/dev/null 2>&1 && for u in getting-started/cli concepts/overview/dns concepts/token; do test -f "public/docs/$u/index.html" && echo "OK  $u" || echo "MISSING $u"; done`
Expected: all `OK` — Hugo writes a redirect stub at each alias path. **A `MISSING` here means a real 404 for existing inbound links.**

- [ ] **Step 7: Verify the build and commit**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` — no increase over baseline.

```bash
git add -A docs/content
git commit -m "docs: move existing pages into the Diataxis tree with aliases"
```

---

## Task 6: Fix the factually wrong content

**Files:**
- Modify: `docs/content/en/docs/how-to/trusted-networks.md`, `docs/content/en/docs/reference/api.md`, `docs/content/en/docs/explanation/architecture.md`

**These are the defects that make the current docs actively harmful. Each is verified against the code.**

- [ ] **Step 1: Fix `--peerguardian` → `--peerguard`**

`how-to/trusted-networks.md` (formerly `peerguardian.md`) instructs users to run `--peerguardian` on four lines (originally 18, 21, 63, 91). The real flag is `peerguard` (`cmd/util.go:368`). urfave/cli hard-fails on an unknown flag, so every one of those command lines is broken.

Note the same page already shows `--peerguard` correctly in a pasted help output around line 29 — the page contradicts itself.

```bash
grep -n 'peerguardian' docs/content/en/docs/how-to/trusted-networks.md
```
Replace every `--peerguardian` **flag usage** with `--peerguard`. Do **not** blanket-replace the word: "PeerGuardian" as the feature's proper name in prose is correct and should stay.

- [ ] **Step 2: Verify the corrected commands actually run**

```bash
go build -o /tmp/ev . && /tmp/ev --peerguard --help >/dev/null 2>&1 && echo "peerguard OK"
/tmp/ev --peerguardian --help 2>&1 | head -2   # expect: flag provided but not defined
```
Expected: `peerguard OK`, and the old spelling errors out — proving the bug was real.

- [ ] **Step 3: Fix the `api --api-listen` example**

`reference/api.md` (formerly `Getting started/api.md`, line 142) shows:

```
$ edgevpn api --api-listen "unix://<path/to/socket>"
```

The `api` subcommand takes `--listen`; `--api-listen` exists only on the root command. Correct form for the subcommand:

```
$ edgevpn api --listen "unix://<path/to/socket>"
```

The root-command form (`edgevpn --api --api-listen unix://...`) is also valid. Show whichever is clearer, but verify it:

```bash
/tmp/ev api --listen "unix:///tmp/ev-test.sock" --help >/dev/null 2>&1 && echo "api --listen OK"
/tmp/ev api --api-listen "unix:///tmp/x.sock" 2>&1 | head -2   # expect: not defined
```

- [ ] **Step 4: Document the API's undocumented endpoints**

`reference/api.md` is missing roughly 40% of the routes. Add, verified against `api/api.go`:

- `GET /api/summary`, `GET /api/files`, `GET /api/nodes`, `GET /api/peerstore`
- the whole `GET /api/metrics` tree: `/api/metrics`, `/api/metrics/protocol`, `/api/metrics/peer`, `/api/metrics/peer/:peer`, `/api/metrics/protocol/:protocol` — noting these are registered **only** when the node has a bandwidth counter, so a 404 means "not enabled", not "broken"
- `GET /debug/pprof/*` when `--debug` is set

Also state plainly, because it is currently implied nowhere: **the API has no authentication.** Anything that can reach the port can write to the network's ledger via `PUT /api/ledger/:bucket/:key/:value`. Link to the security model page (Task 10).

Note that responses are PascalCase (`PeerID`, `RateIn`, `BlockChain`) because the Go types carry no `json` struct tags.

- [ ] **Step 5: Update the stale architecture claims**

`explanation/architecture.md` predates the authenticated-ledger work. It says the blockchain is "ephemeral and on-memory" and that nodes not on the blockchain "can't talk to each other". Since `pkg/blockchain/{sign,policy,reaper}.go` landed, entries are signed, owner-scoped and TTL-reaped, and a disk store exists (`pkg/blockchain/store_disk.go`).

Correct the claims and link forward to `explanation/authenticated-ledger.md` (Task 9). Read the actual code before rewriting — do not paraphrase this plan.

- [ ] **Step 6: Verify and commit**

Run: `grep -rc 'peerguardian' docs/content/en/docs/ | grep -v ':0' || echo "no stale flag usages"`
Expected: no `--peerguardian` flag usages remain (prose mentions of the feature name are fine).

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` — no increase.

```bash
git add docs/content/en/docs
git commit -m "docs: fix the broken peerguard flag, api example and stale architecture claims"
```

---

## Task 7: Relocate the README's unique content

**Files:**
- Create: `docs/content/en/docs/how-to/use-as-a-library.md`, `docs/content/en/docs/tutorials/decentralized-k3s-cluster.md`, `docs/content/en/docs/troubleshooting.md`, `docs/content/en/docs/explanation/when-not-to-use-edgevpn.md`
- Modify: `README.md`

**This is relocation, not new writing.** The content already exists and is correct; it is moving so the site becomes canonical and the drift ends.

| README section (line at time of writing) | Destination |
|---|---|
| `:notebook: As a library` (174) | `how-to/use-as-a-library.md` |
| k3s example (153) | `tutorials/decentralized-k3s-cluster.md` |
| `:notebook: Troubleshooting` (240) | `troubleshooting.md` |
| `:question: Is it for me?` (130) + `:warning: Warning!` (149) | `explanation/when-not-to-use-edgevpn.md` |

- [ ] **Step 1: Locate the sections (line numbers will have shifted)**

```bash
grep -n '^#\{1,3\} ' README.md
```

- [ ] **Step 2: Create each destination page**

Move the content across with front matter added. Adapt only what must change: GitHub-flavoured emoji headings (`:notebook:`) become plain titles, and relative repo links become site links. **Do not rewrite the prose** — it is correct, and rewriting invites new errors.

Give each a unique weight in its section and a `description`.

- [ ] **Step 3: Verify the library example still compiles**

`how-to/use-as-a-library.md` contains a Go snippet using `edgevpn` as a package. Extract it to a scratch file outside the repo and confirm it builds against the current API:

```bash
mkdir -p /tmp/evlib && cd /tmp/evlib
# write the snippet as main.go, with a go.mod requiring github.com/mudler/edgevpn
go mod init evlibcheck && go mod edit -replace github.com/mudler/edgevpn=/home/mudler/_git/edgevpn
go mod tidy && go build ./... && echo "LIBRARY EXAMPLE COMPILES"
```
If it does **not** compile, the README example is already stale. Fix it to match the current API and say so in your report — that is a real find, not a failure.

- [ ] **Step 4: Verify the troubleshooting commands**

The troubleshooting section covers `sysctl net.core.rmem_max` and multiplexer negotiation failures. Confirm the sysctl name and syntax are correct on Linux:

```bash
sysctl net.core.rmem_max
```
Mark anything you cannot verify (e.g. the multiplexer error text, which needs a real failing peer) as unverified in your report rather than asserting it.

- [ ] **Step 4b: Create `tutorials/install.md`**

The site has no installation page of its own, and `install.sh` — the one-liner
installer at the repo root — is never mentioned in the documentation at all.

Write a fuller install page than the README carries: the `install.sh`
one-liner, downloading a release binary, the container image
(`quay.io/mudler/edgevpn`), and building from source (which now needs Node —
see `CONTRIBUTING.md` and the React UI work). Give it `weight: 10` so it sorts
above `your-first-network.md`.

The README keeps its own short Installation section, per the spec — it links
here for the fuller version rather than duplicating it. That is deliberate: the
README's job is to get someone running in a minute, and this page's job is to
cover every installation route.

Verify the one-liner is real before documenting it:

```bash
head -20 install.sh
grep -n 'curl\|wget' README.md | head -3
```

- [ ] **Step 5: Trim the README**

Remove the four relocated sections. Replace each with a one-line pointer to the site page. The README keeps: badges, the pitch, the feature list, screenshots, installation, a 5-minute quickstart, projects-using-EdgeVPN, contribution, credits, licence.

**Do not touch the licence badge or footer.** They are inconsistent with `LICENSE` (see Global Constraints) and resolving that is the maintainer's call.

- [ ] **Step 6: Verify no content was lost**

```bash
git show HEAD:README.md | wc -l ; wc -l < README.md
grep -c 'k3s' README.md docs/content/en/docs/tutorials/decentralized-k3s-cluster.md
```
Confirm every removed section exists at its destination. A section that appears in neither is a real loss — check before committing.

- [ ] **Step 7: Verify and commit**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` — no increase.

```bash
git add README.md docs/content/en/docs
git commit -m "docs: relocate README-only content to the site and trim the README"
```

---

## Task 8: HTTP egress and the proxy

**Files:**
- Create: `docs/content/en/docs/how-to/http-egress-and-proxy.md`
- Modify: `pkg/services/egress.go` (one-line bug fix plus a guard — authorised, see Step 0)

**An entire feature is invisible.** `grep -ril egress docs/content/` returns 0 files, against 224 lines in `pkg/services/egress.go` plus the `edgevpn proxy` subcommand.

**Corrections to this plan, established by direct inspection — the earlier draft of this task was wrong in three ways:**

1. **It is an HTTP forward proxy, not an IP-level exit node.** `ProxyService` (`pkg/services/egress.go:92`) "starts a local http proxy server which redirects requests to egresses into the network". `ServeHTTP` reads an HTTP request, opens a libp2p stream to a chosen egress peer, and writes the request over it; the egress side round-trips it with `http.DefaultTransport`. It does **not** route arbitrary IP traffic, so do not describe it as a VPN exit node or imply "all your traffic". The page is named accordingly.
2. **There is no `CONNECT` handling** anywhere in `ServeHTTP` or the egress handler. The egress side sets `req.URL.Scheme = "https"` in one branch, but no tunnel is established. Determine empirically whether HTTPS works at all and state what you find — do not assume either way.
3. **The `--egress` flag lives in `cmd/main.go:111`**, not `cmd/util.go`. `--egress-announce-time` is at `cmd/main.go:116`, and `cmd/main.go:180-181` wires them to `services.Egress(...)`.

- [ ] **Step 0: Fix the egress selection panic (authorised code change)**

`pkg/services/egress.go:158` selects a peer with:

```go
chosen := availableEgresses[rand.Intn(len(availableEgresses)-1)]
```

`rand.Intn` panics for any argument ≤ 0. I reproduced the behaviour directly:

| available egresses | result |
|---|---|
| 0 | **panic**: invalid argument to Intn |
| 1 | **panic**: invalid argument to Intn |
| 2 | always index 0 — last element unreachable |
| 3 | index 0 or 1 — last element unreachable |

So `edgevpn proxy` crashes in exactly the setup this page will document — one exit node — and can never select the last egress in any list.

Fix it: use `rand.Intn(len(availableEgresses))`, and guard the empty case before the call by returning `http.StatusServiceUnavailable` with a clear message rather than panicking. Note `ServeHTTP` already has an `http.Error(..., http.StatusServiceUnavailable)` path lower down; match that style.

Write a Go test that fails against the current code. The selection logic is inline in `ServeHTTP`, so extracting it into a small helper (e.g. `pickEgress(available []string) (string, bool)`) is the cleanest way to make it testable — do that, keep the change minimal, and cover 0, 1, 2 and many.

Commit this separately from the documentation, with a message describing it as a bug fix, since it is not docs work.

- [ ] **Step 1: Establish the remaining facts from source**

```bash
sed -n '40,100p' pkg/services/egress.go     # the egress side
sed -n '130,200p' pkg/services/egress.go    # the proxy side
sed -n '25,60p' cmd/proxy.go
sed -n '105,125p' cmd/main.go               # --egress and --egress-announce-time
grep -n 'Egress' pkg/protocol/protocol.go
```
Record: exact flag names and defaults, the `edgevpn proxy` flags (`--listen`, `--interval`, `--dead-interval`, `--debug`), the ledger bucket egress nodes announce into (`protocol.EgressService`), and the protocol ID (`protocol.EgressProtocol`).

- [ ] **Step 2: Write the page**

Cover, in this order:

1. **What it is, stated precisely** — a node started with `--egress` advertises itself as an HTTP egress. A peer running `edgevpn proxy` exposes a local HTTP proxy that forwards requests over libp2p to one of those egress nodes, which performs the request and returns the response. No VPN interface is required on either side, which is what distinguishes this from VPN mode. Say explicitly that this proxies **HTTP requests**, not arbitrary IP traffic — a reader who expects an exit node will otherwise be surprised.
2. **Running an egress node** — the `--egress` flag, `--egress-announce-time`, and their env vars, taken from `cmd/main.go`.
3. **Using one** — `edgevpn proxy --listen :8080`, then pointing a client at that local HTTP proxy (`http_proxy=http://localhost:8080`, or a browser proxy setting).
4. **HTTPS** — state what you actually determined in Step 1. There is no `CONNECT` handling in the code, so if HTTPS does not work, say so plainly; that is a far more useful page than one that stays silent and lets the reader discover it.
5. **A security section, which is mandatory.** Requests leave the network at the egress node's address, so that node's operator sees every URL and can read or alter unencrypted traffic. Any holder of the network token can route through any egress, because EdgeVPN's model is perimeter-only — there is no per-peer authorization. Anyone running an egress is accepting responsibility for that traffic. Link to `explanation/security-model.md`.
6. **Selection behaviour** — an egress is chosen at random per request from those seen alive within `--dead-interval`. Requests are not pinned to one egress, so consecutive requests may exit from different nodes. That matters for anything session-based.

Include a `weight` unique in `how-to/`, and a `description`.

- [ ] **Step 3: Verify every command**

```bash
go build -o /tmp/ev .
/tmp/ev --egress --help >/dev/null 2>&1 && echo "--egress accepted"
/tmp/ev proxy --help
```
Confirm every flag you documented appears in the real help output, with the defaults you claimed. Any command needing two hosts (actually routing traffic) is marked **unverified** in your report — do not claim you ran it.

- [ ] **Step 4: Verify and commit**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` — no increase.
Run: `grep -ril egress docs/content/ | wc -l` — expect at least 1.

```bash
git add docs/content/en/docs/how-to/http-egress-and-proxy.md
git commit -m "docs: document HTTP egress and the proxy"
```

---

## Task 9: Ledger ownership

**Files:**
- Create: `docs/content/en/docs/how-to/ledger-ownership.md`
- Move: `docs/design/authenticated-ledger.md` → `docs/content/en/docs/explanation/authenticated-ledger.md`

**Why this matters most for existing users:** `--ownership` defaults to `"enforce"` (`cmd/util.go`), and its own usage string warns *"All nodes on a network must run the same mode/wire format, so flip the whole network together."* A user upgrading into a mixed-version network gets a silently broken ledger, and the only explanation lives in a 369-line design document that has never shipped.

- [ ] **Step 1: Publish the design document**

```bash
git mv docs/design/authenticated-ledger.md docs/content/en/docs/explanation/authenticated-ledger.md
```

Add Hugo front matter at the top. **Do not rewrite the body** — it is the best writing in the repository. Only adjust what breaks in Hugo: check for `$(...)` or `$VAR` in prose that Hugo's KaTeX renderer parses as math (this is the cause of three pre-existing build errors elsewhere), and for headings that clash with the front-matter title.

```yaml
---
title: "The authenticated ledger"
linkTitle: "Authenticated ledger"
weight: 30
description: >
  How ledger entries are signed, owned, versioned and reaped.
---
```

- [ ] **Step 2: Establish the operator-facing facts**

```bash
grep -n -B2 -A6 '"ownership"' cmd/util.go
grep -n -A6 'ownership-ttl' cmd/util.go
sed -n '1,60p' pkg/blockchain/sign.go
sed -n '1,50p' pkg/blockchain/policy.go
sed -n '1,50p' pkg/blockchain/reaper.go
grep -rn 'SignedData' pkg/blockchain/data.go | head
```
Record the three modes and exactly what each does, the TTL flag and its default, and what a node logs when it rejects a write.

- [ ] **Step 3: Write `how-to/ledger-ownership.md`**

The operator's half. Cover:

1. **The three modes** — `enforce` (sign, and reject unauthorized writes; the default), `observe` (sign, log violations, accept), `off` (legacy, opt out).
2. **The upgrade hazard, prominently.** Modes are wire-format incompatible. All nodes on a network must run the same mode. Flipping one node at a time produces a network that appears to work while writes are silently rejected. Give the safe path: move the whole network to `observe` first, confirm no violations are logged, then move to `enforce`.
3. **Ephemeral identities** — the runtime warning users will see, and its relationship to `--privkey-cache`.
4. **`--ownership-ttl`** and how reaping interacts with nodes that go offline.
5. A link to `explanation/authenticated-ledger.md` for the design.

- [ ] **Step 4: Verify the flags and defaults**

```bash
go build -o /tmp/ev .
/tmp/ev --help 2>&1 | grep -A2 'ownership'
```
Confirm the modes, the default, and the TTL default match what you wrote.

- [ ] **Step 5: Verify and commit**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'`
Expected: `0`. The published design doc is 369 lines of new content — if it introduces KaTeX math-parse errors, fix them here (escape the `$`), because this page is new and its errors are yours.

```bash
git add -A docs
git commit -m "docs: document ledger ownership and publish the authenticated ledger design"
```

---

## Task 10: The security model

**Files:**
- Create: `docs/content/en/docs/explanation/security-model.md`

**EdgeVPN ships trust zones, peer gating, ownership enforcement, relay ACLs and an unauthenticated API, and nothing ties them together.** The content exists only scattered across the trusted-networks page, the authenticated-ledger design doc, long usage strings in `cmd/util.go`, and a README warning.

**This page must be honest above all else.** It is the page a person reads before deciding whether to trust EdgeVPN with their network. Overstating the guarantees here is worse than having no page.

- [ ] **Step 1: Establish the model from source**

```bash
sed -n '1,60p' pkg/trustzone/peerguardian.go
sed -n '1,60p' pkg/trustzone/peergater.go
ls pkg/trustzone/authprovider/ecdsa/
grep -n -A4 'peergate\|peerguard\|whitelist\|blacklist' cmd/util.go | head -40
sed -n '1,50p' pkg/config/relay_acl.go
grep -n 'crypto\|otp\|sealer' pkg/crypto/*.go | head
```

- [ ] **Step 2: Write the page**

It must state plainly, near the top:

> **EdgeVPN's security model is perimeter-only.** Anyone holding the network
> token is a fully trusted member of the network. There is no per-peer
> authorization on the data plane, and no audit trail of which peer did what.
> The token *is* the security boundary.

Then cover:

1. **What a leaked token grants** — full network membership: join the VPN, read and write the ledger, use any exit node, resolve internal DNS.
2. **Token rotation** — the OTP mechanism, `--key-otp-interval`, and what rotating actually invalidates.
3. **Trust zones / PeerGuardian / PeerGater** — what they add on top (admission control via ECDSA-signed authorization), what they do *not* add (they gate who may join, not what a member may do), and that they are marked Experimental in the flag usage.
4. **Ledger ownership** — what signing adds, linking to Task 9's page.
5. **The API has no authentication.** Anything that can reach the port can write to the ledger. Recommend the unix-socket mode (`unix://`, with `APILISTENUNIXMODE` controlling the file mode, default `0660`) over a TCP listener, and never exposing the TCP port beyond localhost.
6. **Relay ACLs** — `--relay-service-network-only` and what it restricts.
7. **What EdgeVPN does not protect against** — a malicious member, traffic analysis by an exit node, a compromised bootstrap peer.

- [ ] **Step 3: Cross-link**

Add links from `how-to/trusted-networks.md`, `reference/api.md` and `how-to/http-egress-and-proxy.md` to this page. Those three pages all raise security questions they should not answer themselves.

- [ ] **Step 4: Verify and commit**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` — no increase.

```bash
git add docs/content/en/docs
git commit -m "docs: add the security model explanation"
```

---

## Task 11: Relays, hop nodes, and compatibility

**Files:**
- Create: `docs/content/en/docs/how-to/relays-and-hop-nodes.md`, `docs/content/en/docs/reference/compatibility.md`

**`edgevpn start` is undocumented** — `grep -rl 'edgevpn start' docs/content/` returns 0 files, despite `cmd/join.go` describing it as "Useful for setting up relays or hop nodes to improve the network connectivity".

- [ ] **Step 1: Establish the facts**

```bash
sed -n '1,60p' cmd/join.go
grep -n -A4 'autorelay\|relay-service' cmd/util.go | head -60
sed -n '1,60p' pkg/config/relay_acl.go
```
Record: what `start` does that the root command does not, the autorelay flag family, and the eight `relay-service-*` flags with their defaults.

- [ ] **Step 2: Write `how-to/relays-and-hop-nodes.md`**

Cover: why a relay helps (NAT traversal failure modes), `edgevpn start` and how it differs from running the VPN, the autorelay flags (`--autorelay`, `--autorelay-discovery-interval`, `--autorelay-static-only`, `--autorelay-static-peer`), and the relay-service family with its resource limits (`--relay-service-max-circuits`, `--relay-service-max-data`, `--relay-service-max-duration`, `--relay-service-reservation-ttl`, `--relay-service-buffer-size`, `--relay-service-acl-refresh`, `--relay-service-network-only`).

Point at the generated `reference/cli/start/` page rather than duplicating the flag table.

- [ ] **Step 3: Write `reference/compatibility.md`**

A version and wire-format matrix. The driver is `--ownership`: modes are wire-format incompatible, so this page tells an operator whether two versions can share a network and what to do when they cannot. Include the safe upgrade sequence from Task 9 and link to it.

Be explicit about what you do **not** know: if you cannot determine from the repository which release introduced ownership modes, say so on the page and in your report rather than inventing a version number. Check `git log --oneline -- pkg/blockchain/sign.go | tail -5` and the tags around it.

- [ ] **Step 4: Verify**

```bash
go build -o /tmp/ev . && /tmp/ev start --help
```
Confirm every flag you documented is real, with the defaults you claimed.

- [ ] **Step 5: Commit**

```bash
git add docs/content/en/docs
git commit -m "docs: document relays, hop nodes and version compatibility"
```

---

## Task 12: P1 pages and honest stubs

**Files:**
- Create: `docs/content/en/docs/how-to/run-with-docker.md`, `docs/content/en/docs/reference/ledger-buckets.md`
- Create as stubs: `docs/content/en/docs/how-to/run-with-systemd.md`, `docs/content/en/docs/how-to/persist-node-identity.md`, `docs/content/en/docs/how-to/tune-for-low-end-devices.md`, `docs/content/en/docs/explanation/discovery-and-nat.md`

- [ ] **Step 1: Write `how-to/run-with-docker.md`**

`docker-compose.yml` is linked from the docs but never explained. Read it and cover: `network_mode: host` and why it is required, the `NET_ADMIN` capability, the `/dev/net/tun` device mount, the healthcheck, `--privkey-cache`, and driving configuration through `EDGEVPNTOKEN`. Mention the published image (`quay.io/mudler/edgevpn`, built by `.github/workflows/images.yml`), which the docs never mention.

- [ ] **Step 2: Write `reference/ledger-buckets.md`**

`pkg/protocol/protocol.go` defines the bucket namespace — `files`, `machines`, `services`, `users`, `healthcheck`, `dns`, `egress`, `trustzone`, `trustzoneAuth` — plus the protocol IDs. The API docs already tell users to `PUT /api/ledger/trustzoneAuth/...` without ever defining what a bucket is.

```bash
sed -n '1,50p' pkg/protocol/protocol.go
```

Document each bucket: what writes to it, what its keys are (`machines` is keyed by **IP address**, `dns` by **regex** — this trips people up), and what reads it.

- [ ] **Step 3: Write the four stubs**

Each stub gets real front matter and an explicit note naming what is missing and where the source is. A stub must not imply content exists.

```markdown
---
title: "Run with systemd"
linkTitle: "Run with systemd"
weight: 130
description: >
  Running EdgeVPN as a systemd service, including socket activation.
---

{{% pageinfo %}}
This page has not been written yet.

EdgeVPN supports systemd socket activation for its API — it reads `LISTEN_PID`
and `LISTEN_FDS` (see `api/api.go`) and inherits the listener systemd passes
it, so the socket's user, group and mode are whatever the `.socket` unit
declares. `APILISTENUNIXMODE` sets the mode when EdgeVPN creates the socket
itself instead. None of this is documented.

Contributions welcome — see [contributing](../../contributing/).
{{% /pageinfo %}}
```

Do the same shape for:
- `persist-node-identity.md` — `--privkey-cache`, `--privkey-cache-dir`, `--ledger-state`
- `tune-for-low-end-devices.md` — `--low-profile`, the nine `limit-*` flags, connection water marks
- `discovery-and-nat.md` — DHT, mDNS, OTP rendezvous, holepunching, relay fallback

Verify `{{% pageinfo %}}` is a real Docsy shortcode before using it: `grep -rn 'pageinfo' $(go env GOMODCACHE)/github.com/google/docsy*/layouts/shortcodes/ 2>/dev/null | head -2`. If it is not available, use a plain blockquote instead.

- [ ] **Step 4: Verify and commit**

Run: `cd docs && make build 2>&1 | grep -ciE '^(error|ERROR)'` — no increase.

```bash
git add docs/content/en/docs
git commit -m "docs: add docker and ledger bucket reference, plus stubs for known gaps"
```

---

## Task 13: Whole-site verification

**Files:** none created; this task verifies and fixes what it finds.

- [ ] **Step 1: Regenerate and confirm the drift gate is clean**

```bash
make docs-gen && git diff --exit-code docs/content/en/docs/reference/ && echo "GATE CLEAN"
```
Expected: `GATE CLEAN`. This must pass on the branch that introduces the gate.

- [ ] **Step 2: Confirm no new build errors**

```bash
cd docs && make build 2>&1 | grep -iE '^(error|ERROR)' | sort | uniq -c | sort -rn | head
```
Expected: no output at all — the build is clean. Any **new** error class is yours to fix. Record the before/after counts in your report.

- [ ] **Step 3: Check every internal link**

```bash
cd docs && make build >/dev/null 2>&1
grep -rhoE 'href="(/docs/[^"]*|\.\./[^"]*)"' public/docs --include=index.html \
  | sed 's/href="//;s/"$//' | sort -u > /tmp/links.txt
wc -l /tmp/links.txt
```
For each site-internal link, confirm a corresponding file exists under `public/`. Report every 404 and fix it. If a link checker is available (`lychee`, `htmltest`), use it and say so.

- [ ] **Step 4: Confirm every alias resolves**

```bash
cd docs
for u in getting-started getting-started/cli getting-started/api getting-started/gui \
         concepts/overview concepts/overview/dns concepts/overview/files \
         concepts/overview/services concepts/overview/peerguardian \
         concepts/architecture concepts/token contribution-guidelines; do
  test -f "public/docs/$u/index.html" && echo "OK  $u" || echo "MISSING $u"
done
```
Expected: all `OK`. Every `MISSING` is a broken inbound link from the README, Kairos, or search results.

- [ ] **Step 5: Confirm the factual fixes hold**

```bash
grep -rn '\-\-peerguardian' docs/content/ && echo "STILL BROKEN" || echo "peerguard fixed"
grep -rn 'api --api-listen' docs/content/ && echo "STILL BROKEN" || echo "api example fixed"
grep -ril egress docs/content/ | wc -l          # expect >= 1
grep -rl 'edgevpn start' docs/content/ | wc -l  # expect >= 1
```

- [ ] **Step 6: Confirm no weight collisions**

```bash
for d in tutorials how-to reference explanation; do
  echo "== $d"
  grep -h '^weight:' docs/content/en/docs/$d/*.md 2>/dev/null | sort | uniq -d
done
```
Expected: no duplicate weights printed for any section.

- [ ] **Step 7: Confirm the whole repo still builds**

```bash
go build ./... && go test ./cmd/ ./internal/docsgen/ && echo "GO OK"
```

- [ ] **Step 8: Commit any fixes**

```bash
git add docs README.md CONTRIBUTING.md .github
git commit -m "docs: fix links and aliases found in whole-site verification"
```

**Never run a bare `git add -A` on this branch** — see the working-tree hazard in Global Constraints.

---

## Verification checklist

- [ ] `make docs-gen && git diff --exit-code docs/content/en/docs/reference/` is clean
- [ ] Adding a flag to `cmd/util.go` produces a diff in the generated reference (canary test from Task 4)
- [ ] `cd docs && make build` succeeds with **zero** errors (baseline is clean)
- [ ] Every alias in Task 13 Step 4 resolves
- [ ] No internal link 404s
- [ ] `grep -rn '\-\-peerguardian' docs/content/` returns nothing
- [ ] `grep -ril egress docs/content/` returns at least one file
- [ ] `grep -rl 'edgevpn start' docs/content/` returns at least one file
- [ ] No duplicate `weight:` values within any section
- [ ] `go build ./... && go test ./cmd/ ./internal/docsgen/` passes
- [ ] The library example in `how-to/use-as-a-library.md` compiles against the current API
- [ ] Every command shown in a page was executed, or is marked unverified in the report
- [ ] `docs/design/` no longer contains `authenticated-ledger.md` (it moved into `content/`)
- [ ] `CONTRIBUTING.md` exists at the repo root

---

## Out of scope — do not add

- The custom Hugo theme (sub-project 3)
- Fixing the licence inconsistency — maintainer's legal call
- The `urfave/cli/v3` phantom direct dependency in `go.mod:32`
- `cmd/peergate.go`'s non-constant format string `go vet` failure
- The echo path-param unescape bug affecting DNS-regex ledger deletes
- Renaming environment variables to a consistent scheme (breaking change)
- The 40 pre-existing KaTeX CDN font errors
- `docs/package.json` being tracked and gitignored simultaneously
