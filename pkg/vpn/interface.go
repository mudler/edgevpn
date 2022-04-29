//go:build !windows
// +build !windows

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
	"io"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/tun"
)

type tunWriter struct {
	tun.Device
	offset int
}

func newtunWriter(t tun.Device, offset int) io.WriteCloser {
	return tunWriter{Device: t, offset: offset}
}

func (t tunWriter) Write(b []byte) (int, error) {
	return t.Device.Write(b, t.offset)
}

func createInterface(c *Config) (tun.Device, error) {
	ifname := c.InterfaceName
	if ifname == "auto" {
		ifname = "\000"
	}
	return tun.CreateTUN(ifname, c.InterfaceMTU)
}

func prepareInterface(c *Config) error {
	fmt.Println("Preparing interface")

	link, err := netlink.LinkByName(c.InterfaceName)
	if err != nil {
		fmt.Println("link", err)

		return err
	}

	addr, err := netlink.ParseAddr(c.InterfaceAddress)
	if err != nil {
		fmt.Println("parse addr", err)

		return err
	}

	err = netlink.LinkSetMTU(link, c.InterfaceMTU)
	if err != nil {
		return err
	}

	fmt.Println(addr)
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		fmt.Println("add addr", err)

		return err
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return err
	}
	fmt.Println("done Preparing interface")

	return nil
}
