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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/config"
	nodeConfig "github.com/mudler/edgevpn/pkg/config"
	"github.com/multiformats/go-multiaddr"

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
	&cli.BoolTFlag{
		Name:   "bootstrap-iface",
		Usage:  "Setup interface on startup (need privileges)",
		EnvVar: "EDGEVPNBOOTSTRAPIFACE",
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
		Value:  720,
	},
	&cli.IntFlag{
		Name:   "ledger-announce-interval",
		Usage:  "Ledger announce interval time",
		EnvVar: "EDGEVPNLEDGERINTERVAL",
		Value:  10,
	},
	&cli.StringFlag{
		Name:   "autorelay-discovery-interval",
		Usage:  "Autorelay discovery interval",
		EnvVar: "EDGEVPNAUTORELAYDISCOVERYINTERVAL",
		Value:  "5m",
	},
	&cli.BoolFlag{
		Name:   "autorelay-static-only",
		Usage:  "Use only defined static relays",
		EnvVar: "EDGEVPNAUTORELAYSTATICONLY",
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
		Value:  0,
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
	&cli.BoolFlag{
		Name:   "low-profile",
		Usage:  "Enable low profile. Lowers connections usage",
		EnvVar: "EDGEVPNLOWPROFILE",
	},
	&cli.BoolFlag{
		Name:   "mplex-multiplexer",
		Usage:  "Enable mplex multiplexer.",
		EnvVar: "EDGEVPNMPLEX",
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
		Name:   "autorelay-static-peer",
		Usage:  "List of autorelay static peers to use",
		EnvVar: "EDGEVPNAUTORELAYPEERS",
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
		Usage:  "Specify a resource limit config (json)",
		EnvVar: "LIMITFILE",
	},
	&cli.StringFlag{
		Name:   "limit-scope",
		Usage:  "Specify a limit scope",
		EnvVar: "LIMITSCOPE",
		Value:  "system",
	},
	&cli.BoolFlag{
		Name:   "resource-limit",
		Usage:  "Enable resource manager. (Experimental) All options prefixed with limit requires resource manager to be enabled",
		EnvVar: "LIMITENABLE",
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
	&cli.BoolFlag{
		Name:   "peerguard",
		Usage:  "Enable peerguard. (Experimental)",
		EnvVar: "PEERGUARD",
	},
	&cli.BoolFlag{
		Name:   "privkey-cache",
		Usage:  "Enable privkey caching. (Experimental)",
		EnvVar: "EDGEVPNPRIVKEYCACHE",
	},
	&cli.StringFlag{
		Name:   "privkey-cache-dir",
		Usage:  "Specify a directory used to store the generated privkey",
		EnvVar: "EDGEVPNPRIVKEYCACHEDIR",
		Value:  stateDir(),
	},
	&cli.StringSliceFlag{
		Name:   "static-peertable",
		Usage:  "List of static peers to use (in `ip:peerid` format)",
		EnvVar: "EDGEVPNSTATICPEERTABLE",
	},
	&cli.StringSliceFlag{
		Name:   "whitelist",
		Usage:  "List of peers in the whitelist",
		EnvVar: "EDGEVPNWHITELIST",
	},
	&cli.BoolFlag{
		Name:   "peergate",
		Usage:  "Enable peergating. (Experimental)",
		EnvVar: "PEERGATE",
	},
	&cli.BoolFlag{
		Name:   "peergate-autoclean",
		Usage:  "Enable peergating autoclean. (Experimental)",
		EnvVar: "PEERGATE_AUTOCLEAN",
	},
	&cli.BoolFlag{
		Name:   "peergate-relaxed",
		Usage:  "Enable peergating relaxation. (Experimental)",
		EnvVar: "PEERGATE_RELAXED",
	},
	&cli.StringFlag{
		Name:   "peergate-auth",
		Usage:  "Peergate auth",
		EnvVar: "PEERGATE_AUTH",
		Value:  "",
	},
	&cli.IntFlag{
		Name:   "peergate-interval",
		Usage:  "Peergater interval time",
		EnvVar: "EDGEVPNPEERGATEINTERVAL",
		Value:  120,
	},
}

func stateDir() string {
	baseDir := ".edgevpn"
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, baseDir)
	}

	dir, _ := os.Getwd()
	return filepath.Join(dir, baseDir)
}

func displayStart(ll *logger.Logger) {
	ll.Info(Copyright)

	ll.Infof("Version: %s commit: %s", internal.Version, internal.Commit)
}

func stringsToMultiAddr(peers []string) []multiaddr.Multiaddr {
	res := []multiaddr.Multiaddr{}
	for _, p := range peers {
		addr, err := multiaddr.NewMultiaddr(p)
		if err != nil {
			continue
		}
		res = append(res, addr)
	}
	return res
}

func cliToOpts(c *cli.Context) ([]node.Option, []vpn.Option, *logger.Logger) {

	var limitConfig *rcmgr.PartialLimitConfig

	autorelayInterval, err := time.ParseDuration(c.String("autorelay-discovery-interval"))
	if err != nil {
		autorelayInterval = 0
	}

	// Authproviders are supposed to be passed as a json object
	pa := c.String("peergate-auth")
	d := map[string]map[string]interface{}{}
	json.Unmarshal([]byte(pa), &d)

	nc := nodeConfig.Config{
		NetworkConfig:     c.String("config"),
		NetworkToken:      c.String("token"),
		Address:           c.String("address"),
		Router:            c.String("router"),
		Interface:         c.String("interface"),
		Libp2pLogLevel:    c.String("libp2p-log-level"),
		LogLevel:          c.String("log-level"),
		LowProfile:        c.Bool("low-profile"),
		Blacklist:         c.StringSlice("blacklist"),
		Concurrency:       c.Int("concurrency"),
		FrameTimeout:      c.String("timeout"),
		ChannelBufferSize: c.Int("channel-buffer-size"),
		InterfaceMTU:      c.Int("mtu"),
		PacketMTU:         c.Int("packet-mtu"),
		BootstrapIface:    c.Bool("bootstrap-iface"),
		Whitelist:         stringsToMultiAddr(c.StringSlice("whitelist")),
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
			AutoRelay:                  c.Bool("autorelay"),
			MaxConnections:             c.Int("max-connections"),
			HolePunch:                  c.Bool("holepunch"),
			Mplex:                      c.Bool("mplex-multiplexer"),
			StaticRelays:               c.StringSlice("autorelay-static-peer"),
			AutoRelayDiscoveryInterval: autorelayInterval,
			OnlyStaticRelays:           c.Bool("autorelay-static-only"),
		},
		Limit: config.ResourceLimit{
			Enable:      c.Bool("limit-enable"),
			FileLimit:   c.String("limit-file"),
			Scope:       c.String("limit-scope"),
			MaxConns:    c.Int("max-connections"), // Turn to 0 to use other way of limiting. Files take precedence
			LimitConfig: limitConfig,
		},
		PeerGuard: config.PeerGuard{
			Enable:        c.Bool("peerguard"),
			PeerGate:      c.Bool("peergate"),
			Relaxed:       c.Bool("peergate-relaxed"),
			Autocleanup:   c.Bool("peergate-autoclean"),
			SyncInterval:  time.Duration(c.Int("peergate-interval")) * time.Second,
			AuthProviders: d,
		},
	}

	lvl, err := log.LevelFromString(nc.LogLevel)
	if err != nil {
		lvl = log.LevelError
	}
	llger := logger.New(lvl)

	checkErr := func(e error) {
		if err != nil {
			llger.Fatal(err.Error())
		}
	}

	// Check if we have any privkey identity cached already
	if c.Bool("privkey-cache") {
		keyFile := filepath.Join(c.String("privkey-cache-dir"), "privkey")
		dat, err := os.ReadFile(keyFile)
		if err == nil && len(dat) > 0 {
			llger.Info("Reading key from", keyFile)

			nc.Privkey = dat
		} else {
			// generate, write
			llger.Info("Generating private key and saving it locally for later use in", keyFile)

			privkey, err := node.GenPrivKey(0)
			checkErr(err)

			r, err := crypto.MarshalPrivateKey(privkey)
			checkErr(err)

			err = os.MkdirAll(c.String("privkey-cache-dir"), 0600)
			checkErr(err)

			err = os.WriteFile(keyFile, r, 0600)
			checkErr(err)

			nc.Privkey = r
		}
	}

	for _, pt := range c.StringSlice("static-peertable") {
		dat := strings.Split(pt, ":")
		if len(dat) != 2 {
			checkErr(fmt.Errorf("wrong format for peertable entries. Want a list of ip/peerid separated by `:`. e.g. 10.1.0.1:... "))
		}
		if nc.Connection.PeerTable == nil {
			nc.Connection.PeerTable = make(map[string]peer.ID)
		}

		nc.Connection.PeerTable[dat[0]] = peer.ID(dat[1])
	}

	nodeOpts, vpnOpts, err := nc.ToOpts(llger)
	if err != nil {
		llger.Fatal(err.Error())
	}

	return nodeOpts, vpnOpts, llger
}

func handleStopSignals() {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	for range s {
		os.Exit(0)
	}
}
