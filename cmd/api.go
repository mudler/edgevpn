package cmd

import (
	"github.com/mudler/edgevpn/api"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func API() cli.Command {
	return cli.Command{
		Name:  "api",
		Usage: "Starts an http server to display network informations",
		Description: `Start listening locally, providing an API for the network.
A simple UI interface is available to display network data.`,
		UsageText: "edgevpn api",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:  "listen",
				Value: ":8080",
				Usage: "Listening address",
			},
		),
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

			return api.API(c.String("listen"), ledger)
		},
	}
}
