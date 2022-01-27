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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/mudler/edgevpn/api"
	edgevpn "github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/vpn"
	"github.com/urfave/cli"
)

const Copyright string = `	edgevpn  Copyright (C) 2021-2022 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.`

func MainFlags() []cli.Flag {
	basedir, _ := os.UserHomeDir()
	if basedir == "" {
		basedir = os.TempDir()
	}

	return append([]cli.Flag{
		&cli.IntFlag{
			Name:  "key-otp-interval",
			Usage: "Tweaks default otp interval (in seconds) when generating new tokens",
			Value: 9000,
		},
		&cli.BoolFlag{
			Name:  "g",
			Usage: "Generates a new configuration and prints it on screen",
		},
		&cli.BoolFlag{
			Name:  "b",
			Usage: "Encodes the new config in base64, so it can be used as a token",
		},
		&cli.BoolFlag{
			Name:   "api",
			Usage:  "Starts also the API daemon locally for inspecting the network status",
			EnvVar: "API",
		},
		&cli.StringFlag{
			Name:   "api-listen",
			Value:  ":8080",
			Usage:  "API listening port",
			EnvVar: "APILISTEN",
		},
		&cli.BoolFlag{
			Name:   "dhcp",
			Usage:  "Enables p2p ip negotiation (experimental)",
			EnvVar: "DHCP",
		},
		&cli.StringFlag{
			Name:   "lease-dir",
			Value:  filepath.Join(basedir, ".edgevpn", "leases"),
			Usage:  "DHCP leases directory",
			EnvVar: "DHCPLEASEDIR",
		},
		&cli.StringFlag{
			Name:   "address",
			Usage:  "VPN virtual address",
			EnvVar: "ADDRESS",
			Value:  "10.1.0.1/24",
		},
		&cli.StringFlag{
			Name:   "dns",
			Usage:  "DNS listening address. Empty to disable dns server",
			EnvVar: "DNSADDRESS",
			Value:  "",
		},
		&cli.BoolTFlag{
			Name:   "dns-forwarder",
			Usage:  "Enables dns forwarding",
			EnvVar: "DNSFORWARD",
		},
		&cli.IntFlag{
			Name:   "dns-cache-size",
			Usage:  "DNS LRU cache size",
			EnvVar: "DNSCACHESIZE",
			Value:  200,
		},
		&cli.IntFlag{
			Name:   "aliveness-healthcheck-interval",
			Usage:  "Healthcheck interval",
			EnvVar: "HEALTHCHECKINTERVAL",
			Value:  120,
		},
		&cli.IntFlag{
			Name:   "aliveness-healthcheck-scrub-interval",
			Usage:  "Healthcheck scrub interval",
			EnvVar: "HEALTHCHECKSCRUBINTERVAL",
			Value:  600,
		},
		&cli.IntFlag{
			Name:   "aliveness-healthcheck-max-interval",
			Usage:  "Healthcheck max interval. Threshold after a node is determined offline",
			EnvVar: "HEALTHCHECKMAXINTERVAL",
			Value:  900,
		},
		&cli.StringSliceFlag{
			Name:   "dns-forward-server",
			Usage:  "List of DNS forward server, e.g. 8.8.8.8:53, 192.168.1.1:53 ...",
			EnvVar: "DNSFORWARDSERVER",
			Value:  &cli.StringSlice{"8.8.8.8:53", "1.1.1.1:53"},
		},
		&cli.StringFlag{
			Name:   "router",
			Usage:  "Sends all packets to this node",
			EnvVar: "ROUTER",
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
			newData := edgevpn.GenerateNewConnectionData(c.Int("key-otp-interval"))
			if c.Bool("b") {
				fmt.Print(newData.Base64())
			} else {
				fmt.Println(newData.YAML())
			}

			os.Exit(0)
		}
		o, vpnOpts, ll := cliToOpts(c)

		o = append(o,
			services.Alive(
				time.Duration(c.Int("aliveness-healthcheck-interval"))*time.Second,
				time.Duration(c.Int("aliveness-healthcheck-scrub-interval"))*time.Second,
				time.Duration(c.Int("aliveness-healthcheck-max-interval"))*time.Second)...)
		if c.Bool("dhcp") {
			// Adds DHCP server
			address, _, err := net.ParseCIDR(c.String("address"))
			if err != nil {
				return err
			}
			nodeOpts, vO := vpn.DHCP(ll, 15*time.Minute, c.String("lease-dir"), address.String())
			o = append(o, nodeOpts...)
			vpnOpts = append(vpnOpts, vO...)
		}

		dns := c.String("dns")
		if dns != "" {
			// Adds DNS Server
			o = append(o,
				services.DNS(dns,
					c.Bool("dns-forwarder"),
					c.StringSlice("dns-forward-server"),
					c.Int("dns-cache-size"),
				)...)
		}

		opts, err := vpn.Register(vpnOpts...)
		if err != nil {
			return err
		}

		e := edgevpn.New(append(o, opts...)...)

		displayStart(ll)

		ctx := context.Background()
		if c.Bool("api") {
			go api.API(ctx, c.String("api-listen"), 5*time.Second, 20*time.Second, e)
		}

		return e.Start(ctx)
	}
}
