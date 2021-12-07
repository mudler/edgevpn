//go:build !windows
// +build !windows

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
