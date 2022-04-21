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
	"fmt"

	"github.com/mudler/edgevpn/pkg/trustzone/authprovider/ecdsa"
	"github.com/urfave/cli"
)

func Peergate() cli.Command {
	return cli.Command{
		Name:        "peergater",
		Usage:       "peergater ecdsa-genkey",
		Description: `Peergater auth utilities`,
		Subcommands: cli.Commands{
			{
				Name: "ecdsa-genkey",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "privkey",
					},
					&cli.BoolFlag{
						Name: "pubkey",
					},
				},
				Action: func(c *cli.Context) error {
					priv, pub, err := ecdsa.GenerateKeys()
					if !c.Bool("privkey") && !c.Bool("pubkey") {
						fmt.Printf("Private key: %s\n", string(priv))
						fmt.Printf("Public key: %s\n", string(pub))
					} else if c.Bool("privkey") {
						fmt.Printf(string(priv))
					} else if c.Bool("pubkey") {
						fmt.Printf(string(pub))
					}
					return err
				},
			},
		},
	}
}
