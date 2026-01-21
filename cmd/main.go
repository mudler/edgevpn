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
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"

	"github.com/mudler/edgevpn/api"
	"github.com/mudler/edgevpn/pkg/node"
	edgevpn "github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/vpn"
	"github.com/urfave/cli/v2"
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
			Value: 360,
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
			Name:  "p",
			Usage: "Generates a ED25519 private key, converts it to protobuf serialized form, and encodes as base64 string",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "Starts API with pprof attached",
		},
		&cli.BoolFlag{
			Name:    "api",
			Usage:   "Starts also the API daemon locally for inspecting the network status",
			EnvVars: []string{"API"},
		},
		&cli.StringFlag{
			Name:    "api-listen",
			Value:   "127.0.0.1:8080",
			Usage:   "API listening port",
			EnvVars: []string{"APILISTEN"},
		},
		&cli.BoolFlag{
			Name:    "dhcp",
			Usage:   "Enables p2p ip negotiation (experimental)",
			EnvVars: []string{"DHCP"},
		},
		&cli.BoolFlag{
			Name:    "transient-conn",
			Usage:   "Allow transient connections",
			EnvVars: []string{"TRANSIENTCONN"},
		},
		&cli.StringFlag{
			Name:    "lease-dir",
			Value:   filepath.Join(basedir, ".edgevpn", "leases"),
			Usage:   "DHCP leases directory",
			EnvVars: []string{"DHCPLEASEDIR"},
		},
		&cli.StringFlag{
			Name:    "address",
			Usage:   "VPN virtual address",
			EnvVars: []string{"ADDRESS"},
			Value:   "10.1.0.1/24",
		},
		&cli.StringFlag{
			Name:    "dns",
			Usage:   "DNS listening address. Empty to disable dns server",
			EnvVars: []string{"DNSADDRESS"},
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "dns-forwarder",
			Usage:   "Enables dns forwarding",
			EnvVars: []string{"DNSFORWARD"},
			Value:   true,
		},
		&cli.BoolFlag{
			Name:    "egress",
			Usage:   "Enables nodes for egress",
			EnvVars: []string{"EGRESS"},
		},
		&cli.IntFlag{
			Name:    "egress-announce-time",
			Usage:   "Egress announce time (s)",
			EnvVars: []string{"EGRESSANNOUNCE"},
			Value:   200,
		},
		&cli.IntFlag{
			Name:    "dns-cache-size",
			Usage:   "DNS LRU cache size",
			EnvVars: []string{"DNSCACHESIZE"},
			Value:   200,
		},
		&cli.StringSliceFlag{
			Name:    "dns-forward-server",
			Usage:   "List of DNS forward server, e.g. 8.8.8.8:53, 192.168.1.1:53 ...",
			EnvVars: []string{"DNSFORWARDSERVER"},
			Value:   cli.NewStringSlice("8.8.8.8:53", "1.1.1.1:53"),
		},
		&cli.StringFlag{
			Name:    "router",
			Usage:   "Sends all packets to this node",
			EnvVars: []string{"ROUTER"},
		},
		&cli.StringFlag{
			Name:    "interface",
			Usage:   "Interface name",
			Value:   "edgevpn0",
			EnvVars: []string{"IFACE"},
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

		if c.Bool("p") {
			// Generates a new protobuf encoded priv key and exit
			privkey, err := node.GenPrivKey(0)
			if err != nil {
				return err
			}

			protoKey, err := crypto.MarshalPrivateKey(privkey)
			if err != nil {
				return err
			}

			fmt.Printf("Private key: %s\n", base64.StdEncoding.EncodeToString(protoKey))
			os.Exit(0)
		}
		o, vpnOpts, ll := cliToOpts(c)

		// Egress and DHCP needs the Alive service
		// DHCP needs alive services enabled to all nodes, also those with a static IP.
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

		if c.Bool("egress") {
			o = append(o, services.Egress(time.Duration(c.Int("egress-announce-time"))*time.Second)...)
		}

		dns := c.String("dns")
		if dns != "" {
			// Adds DNS Server
			o = append(o,
				services.DNS(ll, dns,
					c.Bool("dns-forwarder"),
					c.StringSlice("dns-forward-server"),
					c.Int("dns-cache-size"),
				)...)
		}

		bwc := metrics.NewBandwidthCounter()
		if c.Bool("api") {
			o = append(o, node.WithLibp2pAdditionalOptions(libp2p.BandwidthReporter(bwc)))
		}

		opts, err := vpn.Register(vpnOpts...)
		if err != nil {
			return err
		}

		e, err := edgevpn.New(append(o, opts...)...)
		if err != nil {
			return err
		}

		displayStart(ll)

		ctx := context.Background()

		if c.Bool("transient-conn") {
			ctx = network.WithAllowLimitedConn(ctx, "accept")
		}

		if c.Bool("api") {
			go api.API(ctx, c.String("api-listen"), 5*time.Second, 20*time.Second, e, bwc, c.Bool("debug"))
		}
		go handleStopSignals()
		return e.Start(ctx)
	}
}
