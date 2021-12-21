package cmd

import (
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/mudler/edgevpn/pkg/logger"
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
	&cli.IntFlag{
		Name:   "mtu",
		Usage:  "Specify a mtu",
		EnvVar: "EDGEVPNMTU",
		Value:  1200,
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

func displayStart(e *edgevpn.EdgeVPN) {
	e.Logger().Info(Copyright)

	e.Logger().Infof("Version: %s commit: %s", internal.Version, internal.Commit)
}

func cliToOpts(c *cli.Context) []edgevpn.Option {
	config := c.String("config")
	address := c.String("address")
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

	opts := []edgevpn.Option{
		edgevpn.WithDiscoveryInterval(time.Duration(c.Int("discovery-interval")) * time.Second),
		edgevpn.WithLedgerAnnounceTime(time.Duration(c.Int("ledger-announce-interval")) * time.Second),
		edgevpn.WithLedgerInterval(time.Duration(c.Int("ledger-syncronization-interval")) * time.Second),
		edgevpn.Logger(llger),
		edgevpn.WithDiscoveryBootstrapPeers(addrsList),
		edgevpn.LibP2PLogLevel(libp2plvl),
		edgevpn.WithInterfaceMTU(c.Int("mtu")),
		edgevpn.WithPacketMTU(1420),
		edgevpn.WithInterfaceAddress(address),
		edgevpn.WithInterfaceName(iface),
		edgevpn.WithInterfaceType(water.TUN),
		edgevpn.NetLinkBootstrap(true),
		edgevpn.FromBase64(mDNS, dht, token),
		edgevpn.FromYaml(mDNS, dht, config),
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

	opts = append(opts, edgevpn.WithLibp2pOptions(libp2pOpts...))

	if ledgerState != "" {
		opts = append(opts, edgevpn.WithStore(blockchain.NewDiskStore(diskv.New(diskv.Options{
			BasePath:     ledgerState,
			CacheSizeMax: uint64(50), // 50MB
		}))))
	} else {
		opts = append(opts, edgevpn.WithStore(&blockchain.MemoryStore{}))

	}

	return opts
}
