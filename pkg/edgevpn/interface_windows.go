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

package edgevpn

import (
	"fmt"
	"log"
	"net"
	"os/exec"

	"github.com/songgao/water"
)

func (e *EdgeVPN) prepareInterface() error {
	err := netsh("interface", "ip", "set", "address", "name=", e.config.InterfaceName, "static", e.config.InterfaceAddress)
	if err != nil {
		log.Println(err)
	}
	err = netsh("interface", "ipv4", "set", "subinterface", e.config.InterfaceName, "mtu=", fmt.Sprintf("%d", e.config.InterfaceMTU))
	if err != nil {
		log.Println(err)
	}
	return nil
}

func (e *EdgeVPN) createInterface() (*water.Interface, error) {
	// TUN on Windows requires address and network to be set on device creation stage
	// We also set network to 0.0.0.0/0 so we able to reach networks behind the node
	// https://github.com/songgao/water/blob/master/params_windows.go
	// https://gitlab.com/openconnect/openconnect/-/blob/master/tun-win32.c
	ip, _, err := net.ParseCIDR(e.config.InterfaceAddress)
	if err != nil {
		return nil, err
	}
	network := net.IPNet{
		IP:   ip,
		Mask: net.IPv4Mask(0, 0, 0, 0),
	}
	config := water.Config{
		DeviceType: e.config.DeviceType,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID:   "tap0901",
			InterfaceName: e.config.InterfaceName,
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
