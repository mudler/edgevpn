package main

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// docOnlyFlag implements cli.DocGenerationFlag but deliberately NOT
// cli.VisibleFlag. This is the exact case documented() exists to handle: the
// two interfaces are separate in urfave/cli, and a single type assertion
// against DocGenerationFlag cannot reach IsVisible(). Such a flag has no way to
// declare itself hidden, so it must render.
type docOnlyFlag struct {
	name string
}

func (f *docOnlyFlag) String() string            { return f.name }
func (f *docOnlyFlag) Apply(*flag.FlagSet) error { return nil }
func (f *docOnlyFlag) Names() []string           { return []string{f.name} }
func (f *docOnlyFlag) IsSet() bool               { return false }
func (f *docOnlyFlag) TakesValue() bool          { return true }
func (f *docOnlyFlag) GetUsage() string          { return "doc only, no visibility interface" }
func (f *docOnlyFlag) GetValue() string          { return "somevalue" }
func (f *docOnlyFlag) GetDefaultText() string    { return "" }
func (f *docOnlyFlag) GetEnvVars() []string      { return []string{"DOCONLYVAR"} }

// visibleOnlyFlag implements cli.VisibleFlag but NOT cli.DocGenerationFlag.
// There is no usage, default or env var to read off it, so rendering a row
// would produce empty cells. It must be skipped.
type visibleOnlyFlag struct {
	name string
}

func (f *visibleOnlyFlag) String() string            { return f.name }
func (f *visibleOnlyFlag) Apply(*flag.FlagSet) error { return nil }
func (f *visibleOnlyFlag) Names() []string           { return []string{f.name} }
func (f *visibleOnlyFlag) IsSet() bool               { return false }
func (f *visibleOnlyFlag) IsVisible() bool           { return true }

func TestRenderFlagTableRendersDocGenerationFlagWithoutVisibleFlag(t *testing.T) {
	out := renderFlagTable([]cli.Flag{&docOnlyFlag{name: "doc-only"}})

	if !strings.Contains(out, "--doc-only") {
		t.Errorf("a DocGenerationFlag that does not implement VisibleFlag was dropped:\n%s", out)
	}
	for _, want := range []string{"doc only, no visibility interface", "DOCONLYVAR", "somevalue"} {
		if !strings.Contains(out, want) {
			t.Errorf("row missing %q\n%s", want, out)
		}
	}
}

func TestRenderFlagTableSkipsVisibleFlagWithoutDocGeneration(t *testing.T) {
	out := renderFlagTable([]cli.Flag{&visibleOnlyFlag{name: "visible-only"}})

	if strings.Contains(out, "visible-only") {
		t.Errorf("a flag with no DocGenerationFlag data was rendered with empty cells:\n%s", out)
	}
	if !strings.Contains(out, "takes no flags of its own") {
		t.Errorf("expected the empty-table placeholder:\n%s", out)
	}
}

// collectEnv must apply the same two-interface rule as renderFlagTable, or the
// env page and the flag tables would disagree about which flags exist.
func TestCollectEnvAppliesTheSameVisibilityRule(t *testing.T) {
	bindings := map[string][]envBinding{}
	collectEnv(bindings, []cli.Flag{
		&docOnlyFlag{name: "doc-only"},
		&visibleOnlyFlag{name: "visible-only"},
		&cli.StringFlag{Name: "hidden", EnvVars: []string{"HIDDENVAR"}, Hidden: true},
	}, "global")

	if _, ok := bindings["DOCONLYVAR"]; !ok {
		t.Errorf("DocGenerationFlag without VisibleFlag was dropped from the env map: %v", bindings)
	}
	if _, ok := bindings["HIDDENVAR"]; ok {
		t.Errorf("hidden flag leaked into the env map: %v", bindings)
	}
}

func TestPruneStaleOnlyRemovesGeneratedPages(t *testing.T) {
	dir := t.TempDir()

	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// A generated page that is about to be rewritten: must survive the sweep,
	// so a later write failure cannot leave the directory emptied.
	fresh := write("proxy.md", banner+"\ncurrent\n")
	// A generated page for a command that no longer exists: must go.
	stale := write("removed-command.md", banner+"\nstale\n")
	// A hand-written page someone dropped in here: must survive, and be
	// reported rather than silently ignored.
	handWritten := write("troubleshooting.md", "---\ntitle: notes\n---\nhand written\n")
	// A non-markdown file: not our business at all.
	other := write("diagram.svg", "<svg/>")

	foreign, err := pruneStale(dir, map[string]bool{"proxy.md": true})
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{fresh, handWritten, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("pruneStale deleted a file it must not touch: %s", filepath.Base(p))
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("pruneStale left the stale generated page %s behind", filepath.Base(stale))
	}

	sort.Strings(foreign)
	if len(foreign) != 1 || foreign[0] != "troubleshooting.md" {
		t.Errorf("foreign files = %v, want [troubleshooting.md]", foreign)
	}
}
