// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"os"

	"github.com/ipfs/go-log/v2"
	"github.com/songgao/water"
	"github.com/urfave/cli"

	internal "github.com/mudler/edgevpn/internal"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/mudler/edgevpn/pkg/blockchain"
	edgevpn "github.com/mudler/edgevpn/pkg/edgevpn"
)

const copyRight string = `	edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.`

func cliToOpts(l *zap.Logger, c *cli.Context) []edgevpn.Option {
	config := c.String("config")
	address := c.String("address")
	iface := c.String("interface")
	token := c.String("token")
	if config == "" &&
		token == "" {
		l.Sugar().Fatal("EDGEVPNCONFIG or EDGEVPNTOKEN not supplied. At least a config file is required")
	}
	return []edgevpn.Option{
		edgevpn.Logger(l),
		edgevpn.LogLevel(log.LevelInfo),
		edgevpn.MaxMessageSize(2 << 20), // 2MB
		edgevpn.WithInterfaceMTU(1450),
		edgevpn.WithPacketMTU(1420),
		edgevpn.WithInterfaceAddress(address),
		edgevpn.WithInterfaceName(iface),
		edgevpn.WithMaxBlockChainSize(1000),
		edgevpn.WithInterfaceType(water.TUN),
		edgevpn.NetLinkBootstrap(true),
		edgevpn.FromBase64(token),
		edgevpn.FromYaml(config),
	}
}

func main() {
	l, _ := zap.NewProduction()
	defer l.Sync() // flushes buffer, if any

	common := []cli.Flag{&cli.StringFlag{
		Name:   "config",
		Usage:  "Specify a path to a edgevpn config file",
		EnvVar: "EDGEVPNCONFIG",
	},
		&cli.StringFlag{
			Name:   "token",
			Usage:  "Specify an edgevpn token in place of a config file",
			EnvVar: "EDGEVPNTOKEN",
		}}

	app := &cli.App{
		Name:        "edgevpn",
		Version:     internal.Version,
		Author:      "Ettore Di Giacinto",
		Usage:       "edgevpn --config /etc/edgevpn/config.yaml",
		Description: "edgevpn uses libp2p to build an immutable trusted blockchain addressable p2p network",
		Copyright:   copyRight,
		Flags: append([]cli.Flag{
			&cli.BoolFlag{
				Name:  "g",
				Usage: "Generates a new configuration and prints it on screen",
			},
			&cli.StringFlag{
				Name:   "address",
				Usage:  "VPN virtual address",
				EnvVar: "ADDRESS",
				Value:  "10.1.0.1/24",
			},
			&cli.StringFlag{
				Name:   "interface",
				Usage:  "Interface name",
				Value:  "edgevpn0",
				EnvVar: "IFACE",
			}}, common...),

		Commands: []cli.Command{{
			Name:        "join",
			Description: "join the network without activating any interface",
			Flags:       common,
			Action: func(c *cli.Context) error {
				e := edgevpn.New(cliToOpts(l, c)...)

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
		}},

		Action: func(c *cli.Context) error {
			if c.Bool("g") {
				// Generates a new config and exit
				newData, err := edgevpn.GenerateNewConnectionData()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				bytesData, err := yaml.Marshal(newData)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				fmt.Println(string(bytesData))
				os.Exit(0)
			}

			e := edgevpn.New(cliToOpts(l, c)...)

			l.Sugar().Info(copyRight)

			l.Sugar().Infof("Version: %s commit: %s", internal.Version, internal.Commit)

			l.Sugar().Info("Start")

			if err := e.Start(); err != nil {
				l.Sugar().Fatal(err.Error())
			}

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		l.Sugar().Fatal(err)
	}
}
