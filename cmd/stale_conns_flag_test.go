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
