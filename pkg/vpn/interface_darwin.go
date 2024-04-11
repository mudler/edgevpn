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
	"net"
	"os/exec"
	"strconv"

	"github.com/mudler/water"
)

func createInterface(c *Config) (*water.Interface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = c.InterfaceName

	return water.New(config)
}

func prepareInterface(c *Config) error {
	iface, err := net.InterfaceByName(c.InterfaceName)
	if err != nil {
		return err
	}

	ip, ipNet, err := net.ParseCIDR(c.InterfaceAddress)
	if err != nil {
		return err
	}

	// Set the MTU using the `ifconfig` command, since the `net` package does not provide a way to set the MTU.
	mtu := strconv.Itoa(c.InterfaceMTU)
	cmd := exec.Command("ifconfig", iface.Name, "mtu", mtu)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Add the address to the interface. This is not directly possible with the `net` package,
	// so we use the `ifconfig` command.
	if ip.To4() == nil {
		// IPV6
		cmd = exec.Command("ifconfig", iface.Name, "inet6", ip.String())
	} else {
		// IPv4
		cmd = exec.Command("ifconfig", iface.Name, "inet", ip.String(), ip.String())
	}
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Bring up the interface. This is not directly possible with the `net` package,
	// so we use the `ifconfig` command.
	cmd = exec.Command("ifconfig", iface.Name, "up")
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Add route
	cmd = exec.Command("route", "-n", "add", "-net", ipNet.String(), ip.String())
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
