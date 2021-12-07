package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/mudler/edgevpn/api"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const Copyright string = `	edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.`

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
		&cli.BoolFlag{
			Name:  "api",
			Usage: "Starts also the API daemon locally for inspecting the network status",
		},
		&cli.StringFlag{
			Name:  "api-listen",
			Value: ":8080",
			Usage: "API listening port",
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
			newData := edgevpn.GenerateNewConnectionData()
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

		ledger, err := e.Ledger()
		if err != nil {
			return err
		}

		if c.Bool("api") {
			go api.API(c.String("api-listen"), 5*time.Second, 20*time.Second, ledger)
		}

		if err := e.Start(); err != nil {
			e.Logger().Fatal(err.Error())
		}

		return nil
	}
}
