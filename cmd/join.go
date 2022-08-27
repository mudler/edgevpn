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
	"context"

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/urfave/cli"
)

func Start() cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "Start the network without activating any interface",
		Description: `Connect over the p2p network without establishing a VPN.
Useful for setting up relays or hop nodes to improve the network connectivity.`,
		UsageText: "edgevpn start",
		Flags:     CommonFlags,
		Action: func(c *cli.Context) error {
			o, _, ll := cliToOpts(c)
			e, err := node.New(o...)
			if err != nil {
				return err
			}

			displayStart(ll)
			go handleStopSignals()

			// Start the node to the network, using our ledger
			if err := e.Start(context.Background()); err != nil {
				return err
			}

			ll.Info("Joining p2p network")
			<-context.Background().Done()
			return nil
		},
	}
}
