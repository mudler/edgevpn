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
	"time"

	"github.com/mudler/edgevpn/api"
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

			// Join the node to the network, using our ledger
			if err := e.Join(); err != nil {
				return err
			}
			ledger, _ := e.Ledger()
			return api.API(c.String("listen"), 5*time.Second, 20*time.Second, ledger)
		},
	}
}
