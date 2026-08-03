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
