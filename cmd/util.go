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
	"runtime"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/vpn"
	"github.com/peterbourgon/diskv"
	"github.com/songgao/water"
	"github.com/urfave/cli"
)

var CommonFlags []cli.Flag = []cli.Flag{
	&cli.StringFlag{
		Name:   "config",
		Usage:  "Specify a path to a edgevpn config file",
		EnvVar: "EDGEVPNCONFIG",
	},
	&cli.StringFlag{
		Name:   "timeout",
		Usage:  "Specify a default timeout for connection stream",
		EnvVar: "EDGEVPNTIMEOUT",
		Value:  "15s",
	},
	&cli.IntFlag{
		Name:   "mtu",
		Usage:  "Specify a mtu",
		EnvVar: "EDGEVPNMTU",
		Value:  1200,
	},
	&cli.IntFlag{
		Name:   "channel-buffer-size",
		Usage:  "Specify a channel buffer size",
		EnvVar: "EDGEVPNCHANNELBUFFERSIZE",
		Value:  0,
	},
	&cli.IntFlag{
		Name:   "discovery-interval",
		Usage:  "DHT discovery interval time",
		EnvVar: "EDGEVPNDHTINTERVAL",
		Value:  120,
	},
	&cli.IntFlag{
		Name:   "ledger-announce-interval",
		Usage:  "Ledger announce interval time",
		EnvVar: "EDGEVPNLEDGERINTERVAL",
		Value:  10,
	},
	&cli.IntFlag{
		Name:   "ledger-syncronization-interval",
		Usage:  "Ledger syncronization interval time",
		EnvVar: "EDGEVPNLEDGERSYNCINTERVAL",
		Value:  10,
	},
	&cli.IntFlag{
		Name:   "nat-ratelimit-global",
		Usage:  "Rate limit global requests",
		EnvVar: "EDGEVPNNATRATELIMITGLOBAL",
		Value:  10,
	},
	&cli.IntFlag{
		Name:   "nat-ratelimit-peer",
		Usage:  "Rate limit perr requests",
		EnvVar: "EDGEVPNNATRATELIMITPEER",
		Value:  10,
	},
	&cli.IntFlag{
		Name:   "nat-ratelimit-interval",
		Usage:  "Rate limit interval",
		EnvVar: "EDGEVPNNATRATELIMITINTERVAL",
		Value:  60,
	},
	&cli.BoolTFlag{
		Name:   "nat-ratelimit",
		Usage:  "Changes the default rate limiting configured in helping other peers determine their reachability status",
		EnvVar: "EDGEVPNNATRATELIMIT",
	},
	&cli.StringFlag{
		Name:   "ledger-state",
		Usage:  "Specify a ledger state directory",
		EnvVar: "EDGEVPNLEDGERSTATE",
	},
	&cli.BoolTFlag{
		Name:   "mdns",
		Usage:  "Enable mDNS for peer discovery",
		EnvVar: "EDGEVPNMDNS",
	},
	&cli.BoolTFlag{
		Name:   "autorelay",
		Usage:  "Automatically act as a relay if the node can accept inbound connections",
		EnvVar: "EDGEVPNAUTORELAY",
	},
	&cli.IntFlag{
		Name:  "concurrency",
		Usage: "Number of concurrent requests to serve",
		Value: runtime.NumCPU(),
	},
	&cli.BoolTFlag{
		Name:   "holepunch",
		Usage:  "Automatically try holepunching when possible",
		EnvVar: "EDGEVPNHOLEPUNCH",
	},
	&cli.BoolTFlag{
		Name:   "natservice",
		Usage:  "Tries to determine reachability status of nodes",
		EnvVar: "EDGEVPNNATSERVICE",
	},
	&cli.BoolTFlag{
		Name:   "natmap",
		Usage:  "Tries to open a port in the firewall via upnp",
		EnvVar: "EDGEVPNNATMAP",
	},
	&cli.BoolTFlag{
		Name:   "dht",
		Usage:  "Enable DHT for peer discovery",
		EnvVar: "EDGEVPNDHT",
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
	&cli.StringSliceFlag{
		Name:   "discovery-bootstrap-peers",
		Usage:  "List of discovery peers to use",
		EnvVar: "EDGEVPNBOOTSTRAPPEERS",
	},
	&cli.StringFlag{
		Name:   "token",
		Usage:  "Specify an edgevpn token in place of a config file",
		EnvVar: "EDGEVPNTOKEN",
	}}

func displayStart(e *node.Node) {
	e.Logger().Info(Copyright)

	e.Logger().Infof("Version: %s commit: %s", internal.Version, internal.Commit)
}

func cliToOpts(c *cli.Context) ([]node.Option, []vpn.Option) {
	config := c.String("config")
	address := c.String("address")
	router := c.String("router")
	iface := c.String("interface")
	logLevel := c.String("log-level")
	libp2plogLevel := c.String("libp2p-log-level")
	dht, mDNS := c.Bool("dht"), c.Bool("mdns")

	ledgerState := c.String("ledger-state")

	addrsList := discovery.AddrList{}
	peers := c.StringSlice("discovery-bootstrap-peers")

	lvl, err := log.LevelFromString(logLevel)
	if err != nil {
		lvl = log.LevelError
	}

	llger := logger.New(lvl)

	libp2plvl, err := log.LevelFromString(libp2plogLevel)
	if err != nil {
		libp2plvl = log.LevelFatal
	}

	token := c.String("token")
	if config == "" &&
		token == "" {
		llger.Fatal("EDGEVPNCONFIG or EDGEVPNTOKEN not supplied. At least a config file is required")
	}

	for _, p := range peers {
		if err := addrsList.Set(p); err != nil {
			llger.Fatal("Failed reading bootstrap peer list", err.Error())
		}
	}

	opts := []node.Option{
		node.WithDiscoveryInterval(time.Duration(c.Int("discovery-interval")) * time.Second),
		node.WithLedgerAnnounceTime(time.Duration(c.Int("ledger-announce-interval")) * time.Second),
		node.WithLedgerInterval(time.Duration(c.Int("ledger-syncronization-interval")) * time.Second),
		node.Logger(llger),
		node.WithDiscoveryBootstrapPeers(addrsList),
		node.LibP2PLogLevel(libp2plvl),
		node.WithInterfaceAddress(address),
		node.FromBase64(mDNS, dht, token),
		node.FromYaml(mDNS, dht, config),
	}

	vpnOpts := []vpn.Option{
		vpn.WithConcurrency(c.Int("concurrency")),
		vpn.WithInterfaceAddress(address),
		vpn.WithLedgerAnnounceTime(time.Duration(c.Int("ledger-announce-interval")) * time.Second),
		vpn.Logger(llger),
		vpn.WithTimeout(c.String("timeout")),
		vpn.WithInterfaceType(water.TUN),
		vpn.NetLinkBootstrap(true),
		vpn.WithChannelBufferSize(c.Int("channel-buffer-size")),
		vpn.WithInterfaceMTU(c.Int("mtu")),
		vpn.WithPacketMTU(1420),
		vpn.WithRouterAddress(router),
		vpn.WithInterfaceName(iface),
	}

	libp2pOpts := []libp2p.Option{libp2p.UserAgent("edgevpn")}

	if c.Bool("autorelay") {
		libp2pOpts = append(libp2pOpts, libp2p.EnableAutoRelay())
	}

	if c.Bool("nat-ratelimit") {
		libp2pOpts = append(libp2pOpts, libp2p.AutoNATServiceRateLimit(
			c.Int("nat-ratelimit-global"),
			c.Int("nat-ratelimit-peer"),
			time.Duration(c.Int("nat-ratelimit-interval"))*time.Second,
		))
	}

	if c.Bool("holepunch") {
		libp2pOpts = append(libp2pOpts, libp2p.EnableHolePunching())
	}

	if c.Bool("natservice") {
		libp2pOpts = append(libp2pOpts, libp2p.EnableNATService())
	}

	if c.Bool("natmap") {
		libp2pOpts = append(libp2pOpts, libp2p.NATPortMap())
	}

	opts = append(opts, node.WithLibp2pOptions(libp2pOpts...))

	if ledgerState != "" {
		opts = append(opts, node.WithStore(blockchain.NewDiskStore(diskv.New(diskv.Options{
			BasePath:     ledgerState,
			CacheSizeMax: uint64(50), // 50MB
		}))))
	} else {
		opts = append(opts, node.WithStore(&blockchain.MemoryStore{}))

	}

	return opts, vpnOpts
}
