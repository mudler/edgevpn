package cmd

import (
	"time"

	"github.com/ipfs/go-log"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
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
	&cli.StringFlag{
		Name:   "ledger-state",
		Usage:  "Specify a ledger state directory",
		EnvVar: "EDGEVPNLEDGERSTATE",
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

	ledgerState := c.String("ledger-state")

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

	opts := []edgevpn.Option{
		edgevpn.WithDiscoveryInterval(time.Duration(c.Int("discovery-interval")) * time.Second),
		edgevpn.WithLedgerAnnounceTime(time.Duration(c.Int("ledger-announce-interval")) * time.Second),
		edgevpn.WithLedgerInterval(time.Duration(c.Int("ledger-syncronization-interval")) * time.Second),
		edgevpn.Logger(llger),
		edgevpn.LibP2PLogLevel(libp2plvl),
		edgevpn.WithInterfaceMTU(c.Int("mtu")),
		edgevpn.WithPacketMTU(1420),
		edgevpn.WithInterfaceAddress(address),
		edgevpn.WithInterfaceName(iface),
		edgevpn.WithInterfaceType(water.TUN),
		edgevpn.NetLinkBootstrap(true),
		edgevpn.FromBase64(token),
		edgevpn.FromYaml(config),
	}

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
