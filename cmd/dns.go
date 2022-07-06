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

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/urfave/cli"
)

func DNS() cli.Command {
	return cli.Command{
		Name:        "dns",
		Usage:       "Starts a local dns server",
		Description: `Start a local dns server which uses the blockchain to resolve addresses`,
		UsageText:   "edgevpn dns",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:   "listen",
				Usage:  "DNS listening address. Empty to disable dns server",
				EnvVar: "DNSADDRESS",
				Value:  "",
			},
			&cli.BoolTFlag{
				Name:   "dns-forwarder",
				Usage:  "Enables dns forwarding",
				EnvVar: "DNSFORWARD",
			},
			&cli.IntFlag{
				Name:   "dns-cache-size",
				Usage:  "DNS LRU cache size",
				EnvVar: "DNSCACHESIZE",
				Value:  200,
			},
			&cli.StringSliceFlag{
				Name:   "dns-forward-server",
				Usage:  "List of DNS forward server, e.g. 8.8.8.8:53, 192.168.1.1:53 ...",
				EnvVar: "DNSFORWARDSERVER",
				Value:  &cli.StringSlice{"8.8.8.8:53", "1.1.1.1:53"},
			},
		),
		Action: func(c *cli.Context) error {
			o, _, ll := cliToOpts(c)

			dns := c.String("listen")
			// Adds DNS Server
			o = append(o,
				services.DNS(ll, dns,
					c.Bool("dns-forwarder"),
					c.StringSlice("dns-forward-server"),
					c.Int("dns-cache-size"),
				)...)

			e, err := node.New(o...)
			if err != nil {
				return err
			}

			displayStart(ll)
			go handleStopSignals()

			ctx := context.Background()
			// Start the node to the network, using our ledger
			if err := e.Start(ctx); err != nil {
				return err
			}

			for {
				time.Sleep(1 * time.Second)
			}
		},
	}
}
