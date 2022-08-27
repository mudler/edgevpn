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
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
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
			&cli.BoolFlag{
				Name: "debug",
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

			bwc := metrics.NewBandwidthCounter()
			o = append(o, node.WithLibp2pAdditionalOptions(libp2p.BandwidthReporter(bwc)))

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

			return api.API(ctx, c.String("listen"), 5*time.Second, 20*time.Second, e, bwc, c.Bool("debug"))
		},
	}
}
