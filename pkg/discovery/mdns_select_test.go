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

package discovery_test

import (
	"errors"
	"net"

	ma "github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/pkg/discovery"
)

// mustIPNet parses an address in CIDR notation and keeps the host part, the
// way interface addresses are reported.
func mustIPNet(cidr string) net.Addr {
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	n.IP = ip
	return n
}

func interfaceNameList(ifaces []net.Interface) []string {
	out := []string{}
	for _, i := range ifaces {
		out = append(out, i.Name)
	}
	return out
}

func ipStringList(ips []net.IP) []string {
	out := []string{}
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

func multiaddrStringList(addrs []ma.Multiaddr) []string {
	out := []string{}
	for _, a := range addrs {
		out = append(out, a.String())
	}
	return out
}

var _ = Describe("mDNS interface selection", func() {
	var (
		ifaces  []net.Interface
		addrs   map[string][]net.Addr
		addrsOf func(net.Interface) ([]net.Addr, error)
	)

	BeforeEach(func() {
		up := net.FlagUp | net.FlagMulticast
		ifaces = []net.Interface{
			{Index: 1, Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
			{Index: 2, Name: "eth0", Flags: up},
			{Index: 3, Name: "wlan0", Flags: up},
			{Index: 4, Name: "tun0", Flags: net.FlagUp | net.FlagPointToPoint},
			{Index: 5, Name: "wlan1", Flags: net.FlagMulticast},
		}
		addrs = map[string][]net.Addr{
			"lo":    {mustIPNet("127.0.0.1/8"), mustIPNet("::1/128")},
			"eth0":  {mustIPNet("10.0.2.15/24"), mustIPNet("fe80::1/64")},
			"wlan0": {mustIPNet("10.0.2.16/24"), mustIPNet("fec0::16/64"), mustIPNet("fe80::2/64")},
			"tun0":  {mustIPNet("10.1.0.4/24")},
			"wlan1": {mustIPNet("192.168.1.5/24")},
		}
		addrsOf = func(i net.Interface) ([]net.Addr, error) {
			return addrs[i.Name], nil
		}
	})

	It("uses up, multicast interfaces and only their routable addresses when no filter is set", func() {
		sel, ips, errs := SelectMDNSInterfaces(ifaces, addrsOf, nil)
		Expect(errs).To(BeEmpty())
		Expect(interfaceNameList(sel)).To(Equal([]string{"eth0", "wlan0"}))
		Expect(ipStringList(ips)).To(Equal([]string{"10.0.2.15", "10.0.2.16", "fec0::16"}))
	})

	It("restricts the interfaces and addresses to those the filter accepts", func() {
		onlyWlan := func(i net.Interface) bool { return i.Name == "wlan0" }
		sel, ips, errs := SelectMDNSInterfaces(ifaces, addrsOf, onlyWlan)
		Expect(errs).To(BeEmpty())
		Expect(interfaceNameList(sel)).To(Equal([]string{"wlan0"}))
		Expect(ipStringList(ips)).To(Equal([]string{"10.0.2.16", "fec0::16"}))
	})

	It("does not let the filter add interfaces that are down or lack multicast", func() {
		all := func(net.Interface) bool { return true }
		sel, ips, errs := SelectMDNSInterfaces(ifaces, addrsOf, all)
		Expect(errs).To(BeEmpty())
		Expect(interfaceNameList(sel)).To(Equal([]string{"eth0", "wlan0"}))
		Expect(ipStringList(ips)).To(Equal([]string{"10.0.2.15", "10.0.2.16", "fec0::16"}))
	})

	It("returns no interfaces when the filter rejects all of them", func() {
		none := func(net.Interface) bool { return false }
		sel, ips, errs := SelectMDNSInterfaces(ifaces, addrsOf, none)
		Expect(errs).To(BeEmpty())
		Expect(sel).To(BeEmpty())
		Expect(ips).To(BeEmpty())
	})

	It("skips an interface whose address lookup fails and keeps the others", func() {
		failOnEth0 := func(i net.Interface) ([]net.Addr, error) {
			if i.Name == "eth0" {
				return nil, errors.New("boom")
			}
			return addrs[i.Name], nil
		}
		sel, ips, errs := SelectMDNSInterfaces(ifaces, failOnEth0, nil)
		Expect(errs).To(HaveLen(1))
		Expect(errs[0]).To(MatchError(And(ContainSubstring("eth0"), ContainSubstring("boom"))))
		Expect(interfaceNameList(sel)).To(Equal([]string{"wlan0"}))
		Expect(ipStringList(ips)).To(Equal([]string{"10.0.2.16", "fec0::16"}))
	})
})

var _ = Describe("mDNS listen address expansion", func() {
	var ips []net.IP

	BeforeEach(func() {
		ips = []net.IP{net.ParseIP("10.0.2.16"), net.ParseIP("fec0::16")}
	})

	It("expands an IPv4 wildcard listen address to the IPv4 addresses only", func() {
		listen := []ma.Multiaddr{ma.StringCast("/ip4/0.0.0.0/tcp/34211")}
		Expect(multiaddrStringList(ExpandMDNSListenAddrs(listen, ips))).To(Equal([]string{
			"/ip4/10.0.2.16/tcp/34211",
		}))
	})

	It("expands an IPv6 wildcard listen address to the IPv6 addresses only", func() {
		listen := []ma.Multiaddr{ma.StringCast("/ip6/::/udp/4001/quic-v1")}
		Expect(multiaddrStringList(ExpandMDNSListenAddrs(listen, ips))).To(Equal([]string{
			"/ip6/fec0::16/udp/4001/quic-v1",
		}))
	})

	It("keeps specific listen addresses and drops non-IP ones", func() {
		listen := []ma.Multiaddr{
			ma.StringCast("/ip4/10.0.2.16/tcp/5000"),
			ma.StringCast("/p2p-circuit"),
		}
		Expect(multiaddrStringList(ExpandMDNSListenAddrs(listen, ips))).To(Equal([]string{
			"/ip4/10.0.2.16/tcp/5000",
		}))
	})
})
