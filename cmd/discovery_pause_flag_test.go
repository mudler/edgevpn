package cmd

import (
	"testing"

	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/urfave/cli/v2"

	"github.com/ipfs/go-log"
)

// runWithFlags runs an app with CommonFlags and the given arguments, and
// hands the context to f.
func runWithFlags(t *testing.T, f func(c *cli.Context), args ...string) {
	t.Helper()
	app := &cli.App{
		Flags:  CommonFlags,
		Action: func(c *cli.Context) error { f(c); return nil },
	}
	if err := app.Run(append([]string{"edgevpn"}, args...)); err != nil {
		t.Fatalf("running app: %v", err)
	}
}

func TestTheDiscoveryPauseIsOffByDefault(t *testing.T) {
	runWithFlags(t, func(c *cli.Context) {
		nc := ConfigFromContext(c)
		opts := discoveryPauseFromContext(c, nc, logger.New(log.LevelError))
		if nc.Discovery.Pause != nil || nc.Discovery.Wake != nil || len(opts) != 0 {
			t.Errorf("without the flag: Pause set %t, Wake set %t, %d node options; want none",
				nc.Discovery.Pause != nil, nc.Discovery.Wake != nil, len(opts))
		}
	})
}

func TestTheDiscoveryPauseFlagWiresThePause(t *testing.T) {
	runWithFlags(t, func(c *cli.Context) {
		nc := ConfigFromContext(c)
		opts := discoveryPauseFromContext(c, nc, logger.New(log.LevelError))
		if nc.Discovery.Pause == nil || nc.Discovery.Wake == nil {
			t.Fatalf("with the flag: Pause set %t, Wake set %t; want both", nc.Discovery.Pause != nil, nc.Discovery.Wake != nil)
		}
		if len(opts) != 1 {
			t.Fatalf("with the flag: %d node options, want 1 (the pause's network service)", len(opts))
		}
		if nc.Discovery.Pause() {
			t.Error("paused before the node started")
		}
		var cfg node.Config
		if err := cfg.Apply(opts...); err != nil {
			t.Fatalf("applying the node option: %v", err)
		}
		if len(cfg.NetworkServices) != 1 {
			t.Errorf("%d network services, want 1", len(cfg.NetworkServices))
		}
	}, "--discovery-pause-when-connected")
}

func TestTheDHTRefreshIntervalFlag(t *testing.T) {
	dhtOptions := func(args ...string) int {
		var n int
		runWithFlags(t, func(c *cli.Context) { n = len(ConfigFromContext(c).Discovery.DHTOptions) }, args...)
		return n
	}
	if got := dhtOptions(); got != 0 {
		t.Errorf("without the flag: %d DHT options, want 0 (the library default)", got)
	}
	if got := dhtOptions("--dht-refresh-interval", "3600"); got != 1 {
		t.Errorf("with the flag: %d DHT options, want 1", got)
	}
}
