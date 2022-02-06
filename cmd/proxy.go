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
	"time"

	"github.com/mudler/edgevpn/api"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/urfave/cli"
)

func Proxy() cli.Command {
	return cli.Command{
		Name:        "proxy",
		Usage:       "Starts a local http proxy server to egress nodes",
		Description: `Start a proxy locally, providing an ingress point for the network.`,
		UsageText:   "edgevpn proxy",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:   "listen",
				Value:  ":8080",
				Usage:  "Listening address",
				EnvVar: "PROXYLISTEN",
			},
			&cli.IntFlag{
				Name:   "interval",
				Usage:  "proxy announce time interval",
				EnvVar: "PROXYINTERVAL",
				Value:  120,
			},
			&cli.IntFlag{
				Name:   "dead-interval",
				Usage:  "interval (in seconds) wether detect egress nodes offline",
				EnvVar: "PROXYDEADINTERVAL",
				Value:  600,
			},
		),
		Action: func(c *cli.Context) error {
			o, _, ll := cliToOpts(c)

			o = append(o, services.Proxy(
				time.Duration(c.Int("interval"))*time.Second,
				time.Duration(c.Int("dead-interval"))*time.Second,
				c.String("listen"))...)
			e, err := node.New(o...)
			if err != nil {
				return err
			}

			displayStart(ll)

			ctx := context.Background()
			// Start the node to the network, using our ledger
			if err := e.Start(ctx); err != nil {
				return err
			}

			return api.API(ctx, c.String("listen"), 5*time.Second, 20*time.Second, e)
		},
	}
}
