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
	"errors"
	"time"

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/urfave/cli"
)

func cliNamePath(c *cli.Context) (name, path string, err error) {
	name = c.Args().Get(0)
	path = c.Args().Get(1)
	if name == "" && c.String("name") == "" {
		err = errors.New("Either a file UUID as first argument or with --name needs to be provided")
		return
	}
	if path == "" && c.String("path") == "" {
		err = errors.New("Either a file UUID as first argument or with --name needs to be provided")
		return
	}
	if c.String("name") != "" {
		name = c.String("name")
	}
	if c.String("path") != "" {
		path = c.String("path")
	}
	return name, path, nil
}

func FileSend() cli.Command {
	return cli.Command{
		Name:        "file-send",
		Aliases:     []string{"fs"},
		Usage:       "Serve a file to the network",
		Description: `Serve a file to the network without connecting over VPN`,
		UsageText:   "edgevpn file-send unique-id /src/path",
		Flags: append(CommonFlags,
			cli.StringFlag{
				Name:     "name",
				Required: true,
				Usage: `Unique name of the file to be served over the network. 
This is also the ID used to refer when receiving it.`,
			},
			cli.StringFlag{
				Name:     "path",
				Usage:    `File to serve`,
				Required: true,
			},
		),
		Action: func(c *cli.Context) error {
			name, path, err := cliNamePath(c)
			if err != nil {
				return err
			}
			o, _, ll := cliToOpts(c)

			// Needed to unblock connections with low activity
			o = append(o,
				services.Alive(
					time.Duration(c.Int("aliveness-healthcheck-interval"))*time.Second,
					time.Duration(c.Int("aliveness-healthcheck-scrub-interval"))*time.Second,
					time.Duration(c.Int("aliveness-healthcheck-max-interval"))*time.Second)...)

			opts, err := services.ShareFile(ll, time.Duration(c.Int("ledger-announce-interval"))*time.Second, name, path)
			if err != nil {
				return err
			}
			o = append(o, opts...)

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

			for {
				time.Sleep(2 * time.Second)
			}
		},
	}
}

func FileReceive() cli.Command {
	return cli.Command{
		Name:        "file-receive",
		Aliases:     []string{"fr"},
		Usage:       "Receive a file which is served from the network",
		Description: `Receive a file from the network without connecting over VPN`,
		UsageText:   "edgevpn file-receive unique-id /dst/path",
		Flags: append(CommonFlags,
			cli.StringFlag{
				Name:  "name",
				Usage: `Unique name of the file to be received over the network.`,
			},
			cli.StringFlag{
				Name:  "path",
				Usage: `Destination where to save the file`,
			},
		),
		Action: func(c *cli.Context) error {
			name, path, err := cliNamePath(c)
			if err != nil {
				return err
			}
			o, _, ll := cliToOpts(c)
			// Needed to unblock connections with low activity
			o = append(o,
				services.Alive(
					time.Duration(c.Int("aliveness-healthcheck-interval"))*time.Second,
					time.Duration(c.Int("aliveness-healthcheck-scrub-interval"))*time.Second,
					time.Duration(c.Int("aliveness-healthcheck-max-interval"))*time.Second)...)
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

			ledger, _ := e.Ledger()

			return services.ReceiveFile(context.Background(), ledger, e, ll, time.Duration(c.Int("ledger-announce-interval"))*time.Second, name, path)
		},
	}
}
