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
			o, _ := cliToOpts(c)
			e := node.New(o...)

			displayStart(e)

			ledger, err := e.Ledger()
			if err != nil {
				return err
			}

			services.SendFile(context.Background(), ledger, e, e.Logger(), time.Duration(c.Int("ledger-announce-interval"))*time.Second, name, path)

			// Start the node to the network, using our ledger
			if err := e.Start(context.Background()); err != nil {
				return err
			}

			for {
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
			o, _ := cliToOpts(c)
			e := node.New(o...)

			displayStart(e)

			// Start the node to the network, using our ledger
			if err := e.Start(context.Background()); err != nil {
				return err
			}

			ledger, _ := e.Ledger()

			return services.ReceiveFile(context.Background(), ledger, e, e.Logger(), time.Duration(c.Int("ledger-announce-interval"))*time.Second, name, path)
		},
	}
}
