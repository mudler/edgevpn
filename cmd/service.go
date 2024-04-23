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
	"github.com/urfave/cli/v2"
)

func cliNameAddress(c *cli.Context) (name, address string, err error) {
	name = c.Args().Get(0)
	address = c.Args().Get(1)
	if name == "" && c.String("name") == "" {
		err = errors.New("Either a file UUID as first argument or with --name needs to be provided")
		return
	}
	if address == "" && c.String("address") == "" {
		err = errors.New("Either a file UUID as first argument or with --name needs to be provided")
		return
	}
	if c.String("name") != "" {
		name = c.String("name")
	}
	if c.String("address") != "" {
		address = c.String("address")
	}
	return name, address, nil
}

func ServiceAdd() *cli.Command {
	return &cli.Command{
		Name:    "service-add",
		Aliases: []string{"sa"},
		Usage:   "Expose a service to the network without creating a VPN",
		Description: `Expose a local or a remote endpoint connection as a service in the VPN. 
		The host will act as a proxy between the service and the connection`,
		UsageText: "edgevpn service-add unique-id ip:port",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:  "name",
				Usage: `Unique name of the service to be server over the network.`,
			},
			&cli.StringFlag{
				Name: "address",
				Usage: `Remote address that the service is running to. That can be a remote webserver, a local SSH server, etc.
For example, '192.168.1.1:80', or '127.0.0.1:22'.`,
			},
		),
		Action: func(c *cli.Context) error {
			name, address, err := cliNameAddress(c)
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

			o = append(o, services.RegisterService(ll, time.Duration(c.Int("ledger-announce-interval"))*time.Second, name, address)...)

			e, err := node.New(o...)
			if err != nil {
				return err
			}

			displayStart(ll)
			go handleStopSignals()

			// Join the node to the network, using our ledger
			if err := e.Start(context.Background()); err != nil {
				return err
			}

			for {
				time.Sleep(2 * time.Second)
			}
		},
	}
}

func ServiceConnect() *cli.Command {
	return &cli.Command{
		Aliases: []string{"sc"},
		Usage:   "Connects to a service in the network without creating a VPN",
		Name:    "service-connect",
		Description: `Bind a local port to connect to a remote service in the network.
Creates a local listener which connects over the service in the network without creating a VPN.
`,
		UsageText: "edgevpn service-connect unique-id (ip):port",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:  "name",
				Usage: `Unique name of the service in the network.`,
			},
			&cli.StringFlag{
				Name: "address",
				Usage: `Address where to bind locally. E.g. ':8080'. A proxy will be created
to the service over the network`,
			},
		),
		Action: func(c *cli.Context) error {
			name, address, err := cliNameAddress(c)
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

			e, err := node.New(
				append(o,
					node.WithNetworkService(
						services.ConnectNetworkService(
							time.Duration(c.Int("ledger-announce-interval"))*time.Second,
							name,
							address,
						),
					),
				)...,
			)
			if err != nil {
				return err
			}
			displayStart(ll)
			go handleStopSignals()

			// starts the node
			return e.Start(context.Background())
		},
	}
}
