package cmd

import (
	"github.com/ipfs/go-log"
	"github.com/mudler/edgevpn/internal"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/peterbourgon/diskv"
	"github.com/songgao/water"
	"github.com/urfave/cli"
)

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
