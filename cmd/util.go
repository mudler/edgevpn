package cmd

import (
	"github.com/ipfs/go-log"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/songgao/water"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

func cliToOpts(l *zap.Logger, c *cli.Context) []edgevpn.Option {
	config := c.String("config")
	address := c.String("address")
	iface := c.String("interface")
	token := c.String("token")
	if config == "" &&
		token == "" {
		l.Sugar().Fatal("EDGEVPNCONFIG or EDGEVPNTOKEN not supplied. At least a config file is required")
	}
	return []edgevpn.Option{
		edgevpn.Logger(l),
		edgevpn.LogLevel(log.LevelInfo),
		edgevpn.MaxMessageSize(2 << 20), // 2MB
		edgevpn.WithInterfaceMTU(1450),
		edgevpn.WithPacketMTU(1420),
		edgevpn.WithInterfaceAddress(address),
		edgevpn.WithInterfaceName(iface),
		edgevpn.WithMaxBlockChainSize(1000),
		edgevpn.WithInterfaceType(water.TUN),
		edgevpn.NetLinkBootstrap(true),
		edgevpn.FromBase64(token),
		edgevpn.FromYaml(config),
	}
}
