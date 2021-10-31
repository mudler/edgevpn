package cmd

import (
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func Join() cli.Command {
	return cli.Command{
		Name:        "join",
		Description: "join the network without activating any interface",
		Flags:       CommonFlags,
		Action: func(c *cli.Context) error {
			e := edgevpn.New(cliToOpts(c)...)

			displayStart(e)

			mw, err := e.MessageWriter()
			if err != nil {
				return err
			}

			ledger := blockchain.New(mw, 1000)

			// Join the node to the network, using our ledger
			if err := e.Join(ledger); err != nil {
				return err
			}

			for {
			}
		},
	}
}
