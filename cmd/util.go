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
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/config"
	nodeConfig "github.com/mudler/edgevpn/pkg/config"

	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/vpn"
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
		Name:   "packet-mtu",
		Usage:  "Specify a mtu",
		EnvVar: "EDGEVPNPACKETMTU",
		Value:  1420,
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
	&cli.IntFlag{
		Name:   "max-connections",
		Usage:  "Max connections",
		EnvVar: "EDGEVPNMAXCONNS",
		Value:  100,
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
	&cli.BoolTFlag{
		Name:   "low-profile",
		Usage:  "Enable low profile. Lowers connections usage",
		EnvVar: "EDGEVPNLOWPROFILE",
	},
	&cli.BoolTFlag{
		Name:   "low-profile-vpn",
		Usage:  "Enable low profile on VPN",
		EnvVar: "EDGEVPNLOWPROFILEVPN",
	},
	&cli.IntFlag{
		Name:   "max-streams",
		Usage:  "Number of concurrent streams",
		Value:  100,
		EnvVar: "EDGEVPNMAXSTREAMS",
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
	&cli.StringSliceFlag{
		Name:   "blacklist",
		Usage:  "List of peers/cidr to gate",
		EnvVar: "EDGEVPNBLACKLIST",
	},
	&cli.StringFlag{
		Name:   "token",
		Usage:  "Specify an edgevpn token in place of a config file",
		EnvVar: "EDGEVPNTOKEN",
	},
	&cli.StringFlag{
		Name:   "limit-file",
		Usage:  "Specify an limit config (json)",
		EnvVar: "LIMITFILE",
	},
	&cli.StringFlag{
		Name:   "limit-scope",
		Usage:  "Specify a limit scope",
		EnvVar: "LIMITSCOPE",
		Value:  "system",
	},
	&cli.BoolFlag{
		Name:   "limit-config",
		Usage:  "Enable inline resource limit configuration",
		EnvVar: "LIMITCONFIG",
	},
	&cli.BoolFlag{
		Name:   "limit-enable",
		Usage:  "Enable resource manager. (Experimental) All options prefixed with limit requires resource manager to be enabled",
		EnvVar: "LIMITENABLE",
	},
	&cli.BoolFlag{
		Name:   "limit-config-dynamic",
		Usage:  "Enable dynamic resource limit configuration",
		EnvVar: "LIMITCONFIGDYNAMIC",
	},
	&cli.Int64Flag{
		Name:   "limit-config-memory",
		Usage:  "Memory resource limit configuration",
		EnvVar: "LIMITCONFIGMEMORY",
		Value:  128,
	},
	&cli.Float64Flag{
		Name:   "limit-config-memory-fraction",
		Usage:  "Fraction memory resource limit configuration (dynamic)",
		EnvVar: "LIMITCONFIGMEMORYFRACTION",
		Value:  10,
	},
	&cli.Int64Flag{
		Name:   "limit-config-min-memory",
		Usage:  "Minimum memory resource limit configuration (dynamic)",
		EnvVar: "LIMITCONFIGMINMEMORY",
		Value:  10,
	},
	&cli.Int64Flag{
		Name:   "limit-config-max-memory",
		Usage:  "Maximum memory resource limit configuration (dynamic)",
		EnvVar: "LIMITCONFIGMAXMEMORY",
		Value:  200,
	},
	&cli.IntFlag{
		Name:   "limit-config-streams",
		Usage:  "Streams resource limit configuration",
		EnvVar: "LIMITCONFIGSTREAMS",
		Value:  200,
	},
	&cli.IntFlag{
		Name:   "limit-config-streams-inbound",
		Usage:  "Inbound streams resource limit configuration",
		EnvVar: "LIMITCONFIGSTREAMSINBOUND",
		Value:  30,
	},
	&cli.IntFlag{
		Name:   "limit-config-streams-outbound",
		Usage:  "Outbound streams resource limit configuration",
		EnvVar: "LIMITCONFIGSTREAMSOUTBOUND",
		Value:  30,
	},
	&cli.IntFlag{
		Name:   "limit-config-conn",
		Usage:  "Connections resource limit configuration",
		EnvVar: "LIMITCONFIGCONNS",
		Value:  200,
	},
	&cli.IntFlag{
		Name:   "limit-config-conn-inbound",
		Usage:  "Inbound connections resource limit configuration",
		EnvVar: "LIMITCONFIGCONNSINBOUND",
		Value:  30,
	},
	&cli.IntFlag{
		Name:   "limit-config-conn-outbound",
		Usage:  "Outbound connections resource limit configuration",
		EnvVar: "LIMITCONFIGCONNSOUTBOUND",
		Value:  30,
	},
	&cli.IntFlag{
		Name:   "limit-config-fd",
		Usage:  "Max fd resource limit configuration",
		EnvVar: "LIMITCONFIGFD",
		Value:  30,
	},
}

func displayStart(ll *logger.Logger) {
	ll.Info(Copyright)

	ll.Infof("Version: %s commit: %s", internal.Version, internal.Commit)
}

func cliToOpts(c *cli.Context) ([]node.Option, []vpn.Option, *logger.Logger) {

	var limitConfig *node.NetLimitConfig

	if c.Bool("limit-config") {
		limitConfig = &node.NetLimitConfig{
			Dynamic:         c.Bool("limit-config-dynamic"),
			Memory:          c.Int64("limit-config-memory"),
			MinMemory:       c.Int64("limit-config-min-memory"),
			MaxMemory:       c.Int64("limit-config-max-memory"),
			MemoryFraction:  c.Float64("limit-config-memory-fraction"),
			Streams:         c.Int("limit-config-streams"),
			StreamsInbound:  c.Int("limit-config-streams-inbound"),
			StreamsOutbound: c.Int("limit-config-streams-outbound"),
			Conns:           c.Int("limit-config-conn"),
			ConnsInbound:    c.Int("limit-config-conn-inbound"),
			ConnsOutbound:   c.Int("limit-config-conn-outbound"),
			FD:              c.Int("limit-config-fd"),
		}
	}

	nc := nodeConfig.Config{
		NetworkConfig:     c.String("config"),
		NetworkToken:      c.String("token"),
		Address:           c.String("address"),
		Router:            c.String("router"),
		Interface:         c.String("interface"),
		Libp2pLogLevel:    c.String("libp2p-log-level"),
		LogLevel:          c.String("log-level"),
		LowProfile:        c.Bool("low-profile"),
		VPNLowProfile:     c.Bool("low-profile-vpn"),
		Blacklist:         c.StringSlice("blacklist"),
		Concurrency:       c.Int("concurrency"),
		FrameTimeout:      c.String("timeout"),
		ChannelBufferSize: c.Int("channel-buffer-size"),
		InterfaceMTU:      c.Int("mtu"),
		PacketMTU:         c.Int("packet-mtu"),
		Ledger: config.Ledger{
			StateDir:         c.String("ledger-state"),
			AnnounceInterval: time.Duration(c.Int("ledger-announce-interval")) * time.Second,
			SyncInterval:     time.Duration(c.Int("ledger-syncronization-interval")) * time.Second,
		},
		NAT: config.NAT{
			Service:           c.Bool("natservice"),
			Map:               c.Bool("natmap"),
			RateLimit:         c.Bool("nat-ratelimit"),
			RateLimitGlobal:   c.Int("nat-ratelimit-global"),
			RateLimitPeer:     c.Int("nat-ratelimit-peer"),
			RateLimitInterval: time.Duration(c.Int("nat-ratelimit-interval")) * time.Second,
		},
		Discovery: config.Discovery{
			BootstrapPeers: c.StringSlice("discovery-bootstrap-peers"),
			DHT:            c.Bool("dht"),
			MDNS:           c.Bool("mdns"),
			Interval:       time.Duration(c.Int("discovery-interval")) * time.Second,
		},
		Connection: config.Connection{
			AutoRelay:      c.Bool("autorelay"),
			MaxConnections: c.Int("max-connections"),
			MaxStreams:     c.Int("max-streams"),
			HolePunch:      c.Bool("holepunch"),
		},
		Limit: config.ResourceLimit{
			Enable:      c.Bool("limit-enable"),
			FileLimit:   c.String("limit-file"),
			Scope:       c.String("limit-scope"),
			MaxConns:    c.Int("max-connections"), // Turn to 0 to use other way of limiting. Files take precedence
			LimitConfig: limitConfig,
		},
	}

	lvl, err := log.LevelFromString(nc.LogLevel)
	if err != nil {
		lvl = log.LevelError
	}
	llger := logger.New(lvl)

	nodeOpts, vpnOpts, err := nc.ToOpts(llger)
	if err != nil {
		llger.Fatal(err.Error())
	}

	return nodeOpts, vpnOpts, llger
}
