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

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/urfave/cli"
)

func Start() cli.Command {
	return cli.Command{
		Name:  "Start",
		Usage: "Start the network without activating any interface",
		Description: `Connect over the p2p network without establishing a VPN.
Useful for setting up relays or hop nodes to improve the network connectivity.`,
		UsageText: "edgevpn Start",
		Flags:     CommonFlags,
		Action: func(c *cli.Context) error {
			o, _, ll := cliToOpts(c)
			e := node.New(o...)

			displayStart(ll)

			// Start the node to the network, using our ledger
			if err := e.Start(context.Background()); err != nil {
				return err
			}

			ll.Info("Joining p2p network")

			for {
			}
		},
	}
}
