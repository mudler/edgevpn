//go:build windows
// +build windows

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

package vpn

import (
	"fmt"
	"log"
	"net"
	"os/exec"

	"github.com/fumiama/water"
)

func prepareInterface(c *Config) error {
	err := netsh("interface", "ip", "set", "address", "name=", c.InterfaceName, "static", c.InterfaceAddress)
	if err != nil {
		log.Println(err)
	}
	err = netsh("interface", "ipv4", "set", "subinterface", c.InterfaceName, "mtu=", fmt.Sprintf("%d", c.InterfaceMTU))
	if err != nil {
		log.Println(err)
	}
	return nil
}

func createInterface(c *Config) (*water.Interface, error) {
	// TUN on Windows requires address and network to be set on device creation stage
	// We also set network to 0.0.0.0/0 so we able to reach networks behind the node
	// https://github.com/fumiama/water//blob/master/params_windows.go
	// https://gitlab.com/openconnect/openconnect/-/blob/master/tun-win32.c
	ip, _, err := net.ParseCIDR(c.InterfaceAddress)
	if err != nil {
		return nil, err
	}
	network := net.IPNet{
		IP:   ip,
		Mask: net.IPv4Mask(0, 0, 0, 0),
	}
	config := water.Config{
		DeviceType: c.DeviceType,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID:   "tap0901",
			InterfaceName: c.InterfaceName,
			Network:       network.String(),
		},
	}

	return water.New(config)
}

func netsh(args ...string) (err error) {
	cmd := exec.Command("netsh", args...)
	err = cmd.Run()
	return
}
