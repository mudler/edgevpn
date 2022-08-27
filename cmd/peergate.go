/*
Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
