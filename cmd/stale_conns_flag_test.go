package cmd

import (
	"testing"

	"github.com/urfave/cli/v2"
)

func TestDisableStaleConnsWatcherFlag(t *testing.T) {
	for _, f := range CommonFlags {
		b, ok := f.(*cli.BoolFlag)
		if !ok || b.Name != "disable-stale-conns-watcher" {
			continue
		}
		if b.Value {
			t.Error("disable-stale-conns-watcher defaults to true, want false")
		}
		if len(b.EnvVars) != 1 || b.EnvVars[0] != "EDGEVPNDISABLESTALECONNSWATCHER" {
			t.Errorf("env vars = %v, want [EDGEVPNDISABLESTALECONNSWATCHER]", b.EnvVars)
		}
		return
	}
	t.Fatal("flag disable-stale-conns-watcher not found in CommonFlags")
}

func configFromArgs(t *testing.T, args ...string) bool {
	t.Helper()
	var got bool
	app := &cli.App{
		Flags: CommonFlags,
		Action: func(c *cli.Context) error {
			got = ConfigFromContext(c).Connection.DisableStaleConnsWatcher
			return nil
		},
	}
	if err := app.Run(append([]string{"edgevpn"}, args...)); err != nil {
		t.Fatalf("running app: %v", err)
	}
	return got
}

func TestConfigFromContextDisableStaleConnsWatcher(t *testing.T) {
	if !configFromArgs(t, "--disable-stale-conns-watcher") {
		t.Error("flag set: Connection.DisableStaleConnsWatcher = false, want true")
	}
	if configFromArgs(t) {
		t.Error("flag unset: Connection.DisableStaleConnsWatcher = true, want false")
	}
}
