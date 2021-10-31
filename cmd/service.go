package cmd

import (
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func ServiceAdd() cli.Command {
	return cli.Command{
		Name:        "service-add",
		Description: "expose a service to the network",
		Flags: append(CommonFlags,
			cli.StringFlag{Name: "name"},
			cli.StringFlag{Name: "remoteaddress"},
		),
		Action: func(c *cli.Context) error {
			e := edgevpn.New(cliToOpts(c)...)

			mw, err := e.MessageWriter()
			if err != nil {
				return err
			}

			ledger := blockchain.New(mw, 1000)

			// Join the node to the network, using our ledger
			e.ExposeService(ledger, c.String("name"), c.String("remoteaddress"))
			// Join the node to the network, using our ledger
			if err := e.Join(ledger); err != nil {
				return err
			}

			for {
			}
		},
	}
}

func ServiceConnect() cli.Command {
	return cli.Command{
		Name:        "service-connect",
		Description: "bind a local port to connect to a remote service",
		Flags: append(CommonFlags,
			cli.StringFlag{Name: "name"},

			cli.StringFlag{Name: "srcaddress"},
		),
		Action: func(c *cli.Context) error {
			e := edgevpn.New(cliToOpts(c)...)

			mw, err := e.MessageWriter()
			if err != nil {
				return err
			}

			ledger := blockchain.New(mw, 1000)

			// Join the node to the network, using our ledger
			if err := e.Join(ledger); err != nil {
				return err
			}

			return e.ConnectToService(ledger, c.String("name"), c.String("srcaddress"))
		},
	}
}
