package cmd

import (
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func FileSend() cli.Command {
	return cli.Command{
		Name:        "file-send",
		Description: "send a file to the network",
		Flags: append(CommonFlags,
			cli.StringFlag{Name: "name"},
			cli.StringFlag{Name: "path"},
		),
		Action: func(c *cli.Context) error {
			e := edgevpn.New(cliToOpts(c)...)

			mw, err := e.MessageWriter()
			if err != nil {
				return err
			}

			ledger := blockchain.New(mw, 1000)

			// Join the node to the network, using our ledger
			e.SendFile(ledger, c.String("name"), c.String("path"))
			// Join the node to the network, using our ledger
			if err := e.Join(ledger); err != nil {
				return err
			}

			for {
			}
		},
	}
}

func FileReceive() cli.Command {
	return cli.Command{
		Name:        "file-receive",
		Description: "receive a file locally",
		Flags: append(CommonFlags,
			cli.StringFlag{Name: "name"},
			cli.StringFlag{Name: "path"},
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

			return e.ReceiveFile(ledger, c.String("name"), c.String("path"))
		},
	}
}
