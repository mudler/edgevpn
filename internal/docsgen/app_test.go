package main

import (
	"strconv"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/mudler/edgevpn/cmd"
)

// The --concurrency default is runtime.NumCPU(), so its value differs between
// the machine that generated the committed reference and the CI runner that
// checks it. The documented default has to be the same on both.
func TestConcurrencyDefaultDoesNotDependOnTheCPUCount(t *testing.T) {
	app := cmd.NewApp("")
	flags := append([]cli.Flag{}, app.Flags...)
	for _, c := range app.Commands {
		flags = append(flags, c.Flags...)
	}

	found := 0
	for _, f := range flags {
		df, ok := documented(f)
		if !ok || flagNames(f) != "`--concurrency`" {
			continue
		}
		found++
		got := defaultText(df)
		if _, err := strconv.Atoi(got); err == nil {
			t.Errorf("--concurrency is documented with the numeric default %q, which is this machine's CPU count", got)
		}
		if got == "" {
			t.Error("--concurrency has no documented default")
		}
	}
	if found == 0 {
		t.Fatal("no documented --concurrency flag found")
	}
}
