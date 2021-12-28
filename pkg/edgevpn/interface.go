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

package edgevpn

import (
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

func (e *EdgeVPN) createInterface() (*water.Interface, error) {
	config := water.Config{
		DeviceType: e.config.DeviceType,
	}
	config.Name = e.config.InterfaceName

	return water.New(config)
}

func (e *EdgeVPN) prepareInterface() error {
	link, err := netlink.LinkByName(e.config.InterfaceName)
	if err != nil {
		return err
	}
	addr, err := netlink.ParseAddr(e.config.InterfaceAddress)
	if err != nil {
		return err
	}

	err = netlink.LinkSetMTU(link, e.config.InterfaceMTU)
	if err != nil {
		return err
	}

	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return err
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return err
	}
	return nil
}
