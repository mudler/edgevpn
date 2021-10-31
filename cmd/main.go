package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const Copyright string = `	edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.`

var CommonFlags []cli.Flag = []cli.Flag{
	&cli.StringFlag{
		Name:   "config",
		Usage:  "Specify a path to a edgevpn config file",
		EnvVar: "EDGEVPNCONFIG",
	},
	&cli.IntFlag{
		Name:   "mtu",
		Usage:  "Specify a mtu",
		EnvVar: "EDGEVPNMTU",
		Value:  1200,
	},
	&cli.StringFlag{
		Name:   "log-level",
		Usage:  "Specify loglevel",
		EnvVar: "EDGEVPNLOGLEVEL",
		Value:  "info",
	},
	&cli.StringFlag{
		Name:   "libp2p-log-level",
		Usage:  "Specify libp2p loglevel",
		EnvVar: "EDGEVPNLIBP2PLOGLEVEL",
		Value:  "fatal",
	},
	&cli.StringFlag{
		Name:   "token",
		Usage:  "Specify an edgevpn token in place of a config file",
		EnvVar: "EDGEVPNTOKEN",
	}}

func MainFlags() []cli.Flag {
	return append([]cli.Flag{
		&cli.BoolFlag{
			Name:  "g",
			Usage: "Generates a new configuration and prints it on screen",
		},
		&cli.BoolFlag{
			Name:  "b",
			Usage: "Encodes the new config in base64, so it can be used as a token",
		},
		&cli.StringFlag{
			Name:   "address",
			Usage:  "VPN virtual address",
			EnvVar: "ADDRESS",
			Value:  "10.1.0.1/24",
		},
		&cli.StringFlag{
			Name:   "interface",
			Usage:  "Interface name",
			Value:  "edgevpn0",
			EnvVar: "IFACE",
		}}, CommonFlags...)
}

func Main() func(c *cli.Context) error {
	return func(c *cli.Context) error {
		if c.Bool("g") {
			// Generates a new config and exit
			newData, err := edgevpn.GenerateNewConnectionData()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			bytesData, err := yaml.Marshal(newData)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if c.Bool("b") {
				fmt.Print(base64.StdEncoding.EncodeToString(bytesData))
			} else {
				fmt.Println(string(bytesData))
			}

			os.Exit(0)
		}

		e := edgevpn.New(cliToOpts(c)...)

		displayStart(e)

		if err := e.Start(); err != nil {
			e.Logger().Fatal(err.Error())
		}

		return nil
	}
}
