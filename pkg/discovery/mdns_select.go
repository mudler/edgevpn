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

package discovery

import (
	"fmt"
	"net"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// selectMDNSInterfaces returns the interfaces mDNS should run on and the IPs it
// should advertise. An interface is used when it is up, supports multicast and,
// if filter is not nil, is accepted by filter. addrsOf returns the addresses of
// one interface. The advertised IPs are the addresses of the selected interfaces
// without loopback and link-local ones, so addresses of interfaces mDNS does not
// run on are not advertised. An interface whose address lookup fails is skipped,
// and its error is returned in errs so the caller can log it.
func selectMDNSInterfaces(
	ifaces []net.Interface,
	addrsOf func(net.Interface) ([]net.Addr, error),
	filter func(net.Interface) bool,
) (selected []net.Interface, ips []net.IP, errs []error) {
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		if filter != nil && !filter(iface) {
			continue
		}
		addrs, err := addrsOf(iface)
		if err != nil {
			errs = append(errs, fmt.Errorf("addresses of interface %s: %w", iface.Name, err))
			continue
		}
		selected = append(selected, iface)
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, ip)
		}
	}
	return selected, ips, errs
}

// expandMDNSListenAddrs turns the host's listen addresses into addresses to
// advertise over mDNS. A wildcard listen address (/ip4/0.0.0.0 or /ip6/::)
// becomes one address per IP of the same family in ips, with the same transport
// and port. A loopback listen address becomes one address per IP in ips. A
// specific IP address is kept as is, and addresses without an IP (such as
// /p2p-circuit) are dropped.
func expandMDNSListenAddrs(listenAddrs []ma.Multiaddr, ips []net.IP) []ma.Multiaddr {
	var bases []ma.Multiaddr
	for _, ip := range ips {
		maddr, err := manet.FromIP(ip)
		if err != nil {
			continue
		}
		bases = append(bases, maddr)
	}

	var expanded []ma.Multiaddr
	for _, la := range listenAddrs {
		ip, err := manet.ToIP(la)
		if err != nil {
			continue
		}
		switch {
		case ip.IsUnspecified():
			resolved, err := manet.ResolveUnspecifiedAddress(la, bases)
			if err != nil {
				// No IP of this address family to advertise.
				continue
			}
			expanded = append(expanded, resolved...)
		case ip.IsLoopback():
			_, transport := ma.SplitFirst(la)
			for _, base := range bases {
				addr := base
				if transport != nil {
					addr = base.Encapsulate(transport)
				}
				expanded = append(expanded, addr)
			}
		default:
			expanded = append(expanded, la)
		}
	}
	return expanded
}
