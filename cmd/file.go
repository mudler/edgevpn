package cmd

import (
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func FileSend() cli.Command {
	return cli.Command{
		Name:        "file-send",
		Aliases:     []string{"fs"},
		Usage:       "Serve a file to the network",
		Description: `Serve a file to the network without connecting over VPN`,
		UsageText:   "edgevpn file-send --name 'unique-id' --path '/src/path'",
		Flags: append(CommonFlags,
			cli.StringFlag{
				Name:     "name",
				Required: true,
				Usage: `Unique name of the file to be served over the network. 
This is also the ID used to refer when receiving it.`,
			},
			cli.StringFlag{
				Name:     "path",
				Usage:    `File to serve`,
				Required: true,
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
		Aliases:     []string{"fr"},
		Usage:       "Receive a file which is served from the network",
		Description: `Receive a file from the network without connecting over VPN`,
		UsageText:   "edgevpn file-receive --name 'unique-id' --path '/dst/path'",
		Flags: append(CommonFlags,
			cli.StringFlag{
				Name:     "name",
				Usage:    `Unique name of the file to be received over the network.`,
				Required: true,
			},
			cli.StringFlag{
				Name:     "path",
				Usage:    `Destination where to save the file`,
				Required: true,
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

			return e.ReceiveFile(ledger, c.String("name"), c.String("path"))
		},
	}
}
