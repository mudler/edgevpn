//go:build darwin
// +build darwin

/*
Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vpn

import (
	"github.com/mudler/water"
	"github.com/vishvananda/netlink"
)

func createInterface(c *Config) (*water.Interface, error) {
	config := water.Config{
		DeviceType: c.DeviceType,
	}
	config.Name = c.InterfaceName

	return water.New(config)
}

func prepareInterface(c *Config) error {
	link, err := netlink.LinkByName(c.InterfaceName)
	if err != nil {
		return err
	}
	addr, err := netlink.ParseAddr(c.InterfaceAddress)
	if err != nil {
		return err
	}

	err = netlink.LinkSetMTU(link, c.InterfaceMTU)
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
