//go:build windows
// +build windows

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
	"fmt"
	"log"
	"os/exec"


	"github.com/mudler/water"
	"github.com/fumiama/water"
	"github.com/fumiama/wintun"
	"golang.org/x/sys/windows"
	"github.com/google/uuid"
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
	// Use deterministic GUID based on interface name, so we
	// don't flood system with Network Profiles
	uuid, _ := uuid.FromBytes([]byte(c.InterfaceName))
	guid, _ := windows.GUIDFromString("{" + uuid.String() + "}")
	// Create an adapter with deterministic GUID which water will
	// take via wintun.OpenAdapter()
	wintun.CreateAdapter(c.InterfaceName, "WaterWintun", &guid)
	config := water.Config{
		DeviceType: c.DeviceType,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "WaterWintun",
			InterfaceName: c.InterfaceName,
		},
	}

	return water.New(config)
}

func netsh(args ...string) (err error) {
	cmd := exec.Command("netsh", args...)
	err = cmd.Run()
	return
}
