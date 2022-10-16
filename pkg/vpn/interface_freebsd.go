//go:build freebsd
// +build freebsd

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
	"github.com/mudler/water"
	"os/exec"
)

func createInterface(c *Config) (*water.Interface, error) {
	config := water.Config{
		DeviceType: c.DeviceType,
	}
	config.Name = c.InterfaceName

	return water.New(config)
}

func prepareInterface(c *Config) error {
	err := sh(fmt.Sprintf("ifconfig %s create", c.InterfaceName))
	if err != nil {
		return err
	}
	err = sh(fmt.Sprintf("ifconfig %s inet %s %s netmask %s", c.InterfaceName, c.InterfaceAddress, c.InterfaceAddress, "255.255.255.0"))
	if err != nil {
		return err
	}
	return sh(fmt.Sprintf("ifconfig %s up", c.InterfaceName))
}

func sh(c string) (err error) {
	_, err = exec.Command("/bin/sh", "-c", c).CombinedOutput()
	return
}
