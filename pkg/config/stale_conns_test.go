package config

import (
	"testing"

	"github.com/mudler/edgevpn/pkg/node"
)

func staleConnsWatcherDisabled(t *testing.T, c Config) bool {
	t.Helper()
	opts, _, err := c.ToOpts(nil)
	if err != nil {
		t.Fatalf("ToOpts() failed: %v", err)
	}
	var nc node.Config
	// Options that need a running node can fail here; only the watcher
	// switch matters for this check.
	for _, o := range opts {
		_ = o(&nc)
	}
	return nc.DisableStaleConnsWatcher
}

func TestToOptsDisablesStaleConnsWatcherWhenSet(t *testing.T) {
	c := Config{NetworkToken: "dummy", Connection: Connection{DisableStaleConnsWatcher: true}}
	if !staleConnsWatcherDisabled(t, c) {
		t.Fatal("watcher still enabled with DisableStaleConnsWatcher set")
	}
}

func TestToOptsKeepsStaleConnsWatcherByDefault(t *testing.T) {
	c := Config{NetworkToken: "dummy"}
	if staleConnsWatcherDisabled(t, c) {
		t.Fatal("watcher disabled without DisableStaleConnsWatcher set")
	}
}
