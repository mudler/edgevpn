package cmd

import (
	"github.com/urfave/cli/v2"
)

// NewApp builds the EdgeVPN CLI.
//
// It is the single definition of the command-line surface: main.go runs it,
// and internal/docsgen walks it to produce the reference documentation. Keeping
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
