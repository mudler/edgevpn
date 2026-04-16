//go:build android

package discovery

// Android-specific mDNS implementation that avoids net.Interfaces() (blocked by
// SELinux on Android) by using anet.Interfaces() instead.
//
// go-libp2p's mdns.NewMdnsService always passes nil to zeroconf.RegisterProxy and
// zeroconf.Browse, which causes both to call listMulticastInterfaces() → net.Interfaces()
// → syscall.NetlinkRIB → bind() on netlink_route_socket → EACCES on Android.
//
// We bypass this by calling zeroconf.RegisterProxy and zeroconf.Browse ourselves
// with explicit interfaces obtained from anet, which uses sendto() instead of bind()
// on the netlink socket.
//
// We also avoid h.Addrs() at startup because the address manager's background goroutine
// may not have populated currentAddrs.localAddrs yet. Instead we derive IPs directly from
// anet.InterfaceAddrs() and ports from h.Network().ListenAddresses().

import (
	"context"
	"math/rand"
	"net"
	"strings"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/zeroconf/v2"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/wlynxg/anet"
)

const (
	mdnsServiceName = "_p2p._udp"
	mdnsDomain      = "local"
	dnsaddrPrefix   = "dnsaddr="
)

func (d *MDNS) Run(l log.StandardLogger, ctx context.Context, h host.Host) error {
	serviceName := d.DiscoveryServiceTag
	if serviceName == "" {
		serviceName = mdnsServiceName
	}
	l.Infof("mdns(android): starting, service=%s", serviceName)

	// Get network interfaces without calling net.Interfaces() (blocked on Android).
	ifaces, err := anet.Interfaces()
	if err != nil {
		return err
	}

	// Keep only interfaces that are up and support multicast.
	var mcIfaces []net.Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		mcIfaces = append(mcIfaces, iface)
	}
	if len(mcIfaces) == 0 {
		l.Warnf("mdns(android): no multicast-capable interfaces found (WiFi off?), mDNS skipped")
		return nil
	}
	l.Infof("mdns(android): using interfaces: %v", ifaceNames(mcIfaces))

	// Get real interface IPs via anet (avoids netlinkrib bind() call).
	// These are used both for the mDNS A/AAAA records and for expanding
	// wildcard/loopback listen addresses into routable addresses.
	ifAddrs, err := anet.InterfaceAddrs()
	if err != nil {
		return err
	}
	var routableIPs []net.IP
	for _, a := range ifAddrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		routableIPs = append(routableIPs, ip)
	}
	if len(routableIPs) == 0 {
		l.Warnf("mdns(android): no routable interface addresses found, mDNS skipped")
		return nil
	}

	// Build multiaddrs for each routable IP (without port — used as base for expansion).
	var routableMaddrs []ma.Multiaddr
	for _, ip := range routableIPs {
		maddr, err := manet.FromIP(ip)
		if err != nil {
			continue
		}
		routableMaddrs = append(routableMaddrs, maddr)
	}

	// Get the raw listen addresses (e.g. /ip4/0.0.0.0/tcp/PORT or /ip4/127.0.0.1/tcp/PORT).
	// These may be wildcards or loopback; we expand them to real IPs below.
	listenAddrs := h.Network().ListenAddresses()

	// Expand wildcard/loopback listen addresses to routable IPs, keeping transport/port.
	var expandedListenAddrs []ma.Multiaddr
	for _, la := range listenAddrs {
		ip, err := manet.ToIP(la)
		if err != nil {
			// Non-IP address (e.g. /p2p-circuit); skip for mDNS.
			continue
		}
		if !ip.IsUnspecified() && !ip.IsLoopback() {
			// Already a routable address.
			expandedListenAddrs = append(expandedListenAddrs, la)
			continue
		}
		// Wildcard or loopback → expand to real IPs with same transport/port.
		_, transport := ma.SplitFirst(la)
		for _, rAddr := range routableMaddrs {
			expanded := rAddr
			if transport != nil {
				expanded = rAddr.Encapsulate(transport)
			}
			expandedListenAddrs = append(expandedListenAddrs, expanded)
		}
	}
	if len(expandedListenAddrs) == 0 {
		l.Warnf("mdns(android): no listen addresses to advertise, mDNS skipped")
		return nil
	}

	// Build p2p multiaddrs (with /p2p/PeerID suffix) for TXT records.
	p2pAddrs, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{
		ID:    h.ID(),
		Addrs: expandedListenAddrs,
	})
	if err != nil {
		return err
	}

	var txts []string
	for _, addr := range p2pAddrs {
		if isSuitableForMDNS(addr) {
			txts = append(txts, dnsaddrPrefix+addr.String())
		}
	}

	// Build the IP strings for the mDNS A/AAAA records.
	ips := ipsToStrings(routableIPs)

	peerName := randomString(32 + rand.Intn(32))

	l.Infof("mdns(android): advertising IPs=%v txts=%v", ips, txts)
	server, err := zeroconf.RegisterProxy(
		peerName,
		serviceName,
		mdnsDomain,
		4001, // port is carried in TXT records; this value is required but ignored by libp2p peers
		peerName,
		ips,
		txts,
		mcIfaces,
	)
	if err != nil {
		return err
	}
	l.Infof("mdns(android): registered proxy, browsing for peers...")

	// Browse for peers on the same service. SelectIfaces bypasses listMulticastInterfaces().
	entryChan := make(chan *zeroconf.ServiceEntry, 1000)

	errCh := make(chan error, 1)
	go func() {
		errCh <- zeroconf.Browse(ctx, serviceName, mdnsDomain, entryChan, zeroconf.SelectIfaces(mcIfaces))
	}()

	notifee := &discoveryNotifee{h: h, c: l}

	// Run the peer-handling loop in the background so Run() returns immediately,
	// matching the non-android implementation (mdns_run.go) which is also non-blocking.
	// startNetwork() must return for the ledger and VPN network services to start.
	go func() {
		defer server.Shutdown()
		for {
			select {
			case entry, ok := <-entryChan:
				if !ok {
					return
				}
				var addrs []ma.Multiaddr
				for _, txt := range entry.Text {
					if !strings.HasPrefix(txt, dnsaddrPrefix) {
						continue
					}
					addr, err := ma.NewMultiaddr(txt[len(dnsaddrPrefix):])
					if err != nil {
						continue
					}
					addrs = append(addrs, addr)
				}
				infos, err := peer.AddrInfosFromP2pAddrs(addrs...)
				if err != nil {
					continue
				}
				for _, info := range infos {
					if info.ID == h.ID() {
						continue
					}
					l.Infof("mdns(android): found peer %s addrs=%v", info.ID, info.Addrs)
					go notifee.HandlePeerFound(info)
				}
			case err := <-errCh:
				if err != nil {
					l.Warnf("mdns(android): browse error: %v", err)
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// ifaceNames returns the names of the given interfaces for logging.
func ifaceNames(ifaces []net.Interface) []string {
	names := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		names = append(names, iface.Name)
	}
	return names
}

// ipsToStrings converts net.IP slice to string slice (IPv4 as dotted decimal, IPv6 as standard).
func ipsToStrings(ips []net.IP) []string {
	ss := make([]string, 0, len(ips))
	for _, ip := range ips {
		ss = append(ss, ip.String())
	}
	return ss
}

// isSuitableForMDNS mirrors the same function from go-libp2p's mdns package.
// It filters multiaddrs to those suitable for LAN mDNS advertisement.
func isSuitableForMDNS(addr ma.Multiaddr) bool {
	if addr == nil {
		return false
	}
	first, _ := ma.SplitFirst(addr)
	if first == nil {
		return false
	}
	switch first.Protocol().Code {
	case ma.P_IP4, ma.P_IP6:
		// ok
	case ma.P_DNS, ma.P_DNS4, ma.P_DNS6, ma.P_DNSADDR:
		if !strings.HasSuffix(strings.ToLower(first.Value()), ".local") {
			return false
		}
	default:
		return false
	}
	// Reject circuit relay and browser-only transports.
	unsuitable := false
	ma.ForEach(addr, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_CIRCUIT, ma.P_WEBTRANSPORT, ma.P_WEBRTC, ma.P_WEBRTC_DIRECT, ma.P_P2P_WEBRTC_DIRECT, ma.P_WS, ma.P_WSS:
			unsuitable = true
			return false
		}
		return true
	})
	return !unsuitable
}

// randomString generates a random lowercase alphanumeric string of length l.
func randomString(l int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	s := make([]byte, 0, l)
	for i := 0; i < l; i++ {
		s = append(s, alphabet[rand.Intn(len(alphabet))])
	}
	return string(s)
}
