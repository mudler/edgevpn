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

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mudler/edgevpn/api"
	edgevpn "github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/vpn"
	"github.com/urfave/cli"
)

const Copyright string = `	edgevpn  Copyright (C) 2021-2022 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.`

func MainFlags() []cli.Flag {
	basedir, _ := os.UserHomeDir()
	if basedir == "" {
		basedir = os.TempDir()
	}

	return append([]cli.Flag{
		&cli.IntFlag{
			Name:  "key-otp-interval",
			Usage: "Tweaks default otp interval (in seconds) when generating new tokens",
			Value: 9000,
		},
		&cli.BoolFlag{
			Name:  "g",
			Usage: "Generates a new configuration and prints it on screen",
		},
		&cli.BoolFlag{
			Name:  "b",
			Usage: "Encodes the new config in base64, so it can be used as a token",
		},
		&cli.BoolFlag{
			Name:  "api",
			Usage: "Starts also the API daemon locally for inspecting the network status",
		},
		&cli.StringFlag{
			Name:  "api-listen",
			Value: ":8080",
			Usage: "API listening port",
		},
		&cli.StringFlag{
			Name:  "lease-dir",
			Value: filepath.Join(basedir, ".edgevpn", "leases"),
			Usage: "DHCP leases directory",
		},
		&cli.StringFlag{
			Name:   "address",
			Usage:  "VPN virtual address, e.g. 10.1.0.1/24. No address specified enables p2p ip negotiation (experimental)",
			EnvVar: "ADDRESS",
		},
		&cli.StringFlag{
			Name:   "router",
			Usage:  "Sends all packets to this node",
			EnvVar: "ROUTER",
		},
		&cli.StringFlag{
			Name:   "interface",
			Usage:  "Interface name",
			Value:  "edgevpn0",
			EnvVar: "IFACE",
		}}, CommonFlags...)
}

func Main() func(c *cli.Context) error {
	return func(c *cli.Context) error {
		if c.Bool("g") {
			// Generates a new config and exit
			newData := edgevpn.GenerateNewConnectionData(c.Int("key-otp-interval"))
			if c.Bool("b") {
				fmt.Print(newData.Base64())
			} else {
				fmt.Println(newData.YAML())
			}

			os.Exit(0)
		}
		o, vpnOpts, ll := cliToOpts(c)

		if c.String("address") == "" {
			nodeOpts, vO := vpn.DHCP(ll, 10*time.Second, c.String("lease-dir"))
			o = append(
				append(
					o,
					services.Alive(30*time.Second)...,
				),
				nodeOpts...,
			)
			vpnOpts = append(vpnOpts, vO...)
		}

		opts, err := vpn.Register(vpnOpts...)
		if err != nil {
			return err
		}

		e := edgevpn.New(append(o, opts...)...)

		displayStart(ll)

		ctx := context.Background()
		if c.Bool("api") {
			go api.API(ctx, c.String("api-listen"), 5*time.Second, 20*time.Second, e)
		}

		return e.Start(ctx)
	}
}
