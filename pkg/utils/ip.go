// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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
