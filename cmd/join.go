package cmd

import (
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func Join() cli.Command {
	return cli.Command{
		Name:  "join",
		Usage: "Join the network without activating any interface",
		Description: `Connect over the p2p network without establishing a VPN.
Useful for setting up relays or hop nodes to improve the network connectivity.`,
		UsageText: "edgevpn join",
		Flags:     CommonFlags,
		Action: func(c *cli.Context) error {
			e := edgevpn.New(cliToOpts(c)...)

			displayStart(e)

			// Join the node to the network, using our ledger
			if err := e.Join(); err != nil {
				return err
			}

			for {
			}
		},
	}
}
