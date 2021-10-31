package cmd

import (
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
)

func ServiceAdd() cli.Command {
	return cli.Command{
		Name:    "service-add",
		Aliases: []string{"sa"},
		Usage:   "Expose a service to the network without creating a VPN",
		Description: `Expose a local or a remote endpoint connection as a service in the VPN. 
		The host will act as a proxy between the service and the connection`,
		UsageText: "edgevpn service-add --name 'unique-id' --remoteaddress 'ip:port'",
		Flags: append(CommonFlags,
			cli.StringFlag{
				Name:     "name",
				Usage:    `Unique name of the service to be server over the network.`,
				Required: true,
			},
			cli.StringFlag{
				Name:     "remoteaddress",
				Required: true,
				Usage: `Remote address that the service is running to. That can be a remote webserver, a local SSH server, etc.
For example, '192.168.1.1:80', or '127.0.0.1:22'.`,
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
		Aliases: []string{"sc"},
		Usage:   "Connects to a service in the network without creating a VPN",
		Name:    "service-connect",
		Description: `Bind a local port to connect to a remote service in the network.
Creates a local listener which connects over the service in the network without creating a VPN.
`,
		UsageText: "edgevpn service-connect --name 'unique-id' --srcaddress '(ip):port'",
		Flags: append(CommonFlags,
			cli.StringFlag{
				Name:     "name",
				Usage:    `Unique name of the service in the network.`,
				Required: true,
			},
			cli.StringFlag{
				Name: "srcaddress",
				Usage: `Address where to bind locally. E.g. ':8080'. A proxy will be created
to the service over the network`,
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

			return e.ConnectToService(ledger, c.String("name"), c.String("srcaddress"))
		},
	}
}
