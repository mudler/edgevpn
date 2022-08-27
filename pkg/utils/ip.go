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

package utils

import (
	"net"
	"sort"

	"github.com/c-robinson/iplib"
)

func NextIP(defaultIP string, ips []string) string {
	if len(ips) == 0 {
		return defaultIP
	}

	r := []net.IP{}
	for _, i := range ips {
		ip := net.ParseIP(i)
		r = append(r, ip)
	}

	sort.Sort(iplib.ByIP(r))

	last := r[len(r)-1]

	return iplib.NextIP(last).String()
}
