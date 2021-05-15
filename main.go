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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ipfs/go-log/v2"
	"github.com/songgao/water"

	internal "github.com/mudler/edgevpn/internal"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	edgevpn "github.com/mudler/edgevpn/pkg/edgevpn"
)

func main() {
	help := flag.Bool("h", false, "Display Help")
	genKeys := flag.Bool("g", false, "Generate pub/priv keys")

	l, _ := zap.NewProduction()
	defer l.Sync() // flushes buffer, if any

	opts := []edgevpn.Option{
		edgevpn.Logger(l),
		edgevpn.LogLevel(log.LevelInfo),
		edgevpn.MaxMessageSize(2 << 20), // 2MB
		edgevpn.WithMTU(1500),
		edgevpn.WithInterfaceMTU(1300),
		edgevpn.WithInterfaceAddress(os.Getenv("ADDRESS")),
		edgevpn.WithInterfaceName(os.Getenv("IFACE")),
		edgevpn.WithInterfaceType(water.TAP),
	}

	opts = append(opts, edgevpn.FromYaml(os.Getenv("EDGEVPNCONFIG")))
	flag.Parse()

	e := edgevpn.New(opts...)

	if *help {
		fmt.Println("edgevpn uses libp2p to build an immutable trusted p2p network")
		fmt.Println("")
		fmt.Println()
		fmt.Println("Usage: Run './edgevpn in two different terminals. Let them connect to the bootstrap nodes, announce themselves and connect to the peers")
		flag.PrintDefaults()
		return
	}

	if *genKeys {
		newData, err := edgevpn.GenerateNewConnectionData()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		bytesData, err := yaml.Marshal(newData)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println(string(bytesData))
		os.Exit(0)
	}

	l.Sugar().Info(`
	edgevpn  Copyright (C) 2021 Ettore Di Giacinto
	This program comes with ABSOLUTELY NO WARRANTY.
	This is free software, and you are welcome to redistribute it
	under certain conditions.
	`)

	l.Sugar().Infof("Version: %s commit: %s", internal.Version, internal.Commit)
	if os.Getenv("EDGEVPNCONFIG") == "" {
		l.Sugar().Fatal("EDGEVPNCONFIG not supplied. config file is required")
	}
	l.Sugar().Info("Start")

	if err := e.Start(); err != nil {
		l.Sugar().Info("Failed")
		os.Exit(1)
	}

}
