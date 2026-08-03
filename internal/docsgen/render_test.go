package main

import (
	"os"
	"path/filepath"
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

// cmd.stateDir() derives --privkey-cache-dir and --lease-dir from the calling
// user's home directory. Emitting that verbatim makes the output differ per
// machine, so the CI drift gate would fail on every run even though nothing
// changed. Machine-specific prefixes must be normalised away.
func TestRenderFlagTableNormalizesMachineSpecificDefaults(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		t.Skip("no usable home directory on this machine")
	}
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "privkey-cache-dir",
			Usage: "Specify a directory used to store the generated privkey",
			Value: filepath.Join(home, ".edgevpn"),
		},
	}
	out := renderFlagTable(flags)

	if strings.Contains(out, home) {
		t.Errorf("machine-specific home path %q leaked into the table:\n%s", home, out)
	}
	if !strings.Contains(out, "$HOME/.edgevpn") {
		t.Errorf("home path was not normalised to $HOME:\n%s", out)
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
