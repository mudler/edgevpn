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

package config

import (
	"fmt"
	"math/bits"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	connmanager "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/crypto"
	"github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/trustzone"
	"github.com/mudler/edgevpn/pkg/trustzone/authprovider/ecdsa"
	"github.com/mudler/edgevpn/pkg/vpn"
	"github.com/mudler/water"
	"github.com/multiformats/go-multiaddr"
	"github.com/peterbourgon/diskv"
)

// Config is the config struct for the node and the default EdgeVPN services
// It is used to generate opts for the node and the services before start.
type Config struct {
	NetworkConfig, NetworkToken                string
	Address                                    string
	ListenMaddrs                               []string
	DHTAnnounceMaddrs                          []multiaddr.Multiaddr
	Router                                     string
	Interface                                  string
	Libp2pLogLevel, LogLevel                   string
	LowProfile, BootstrapIface                 bool
	Blacklist                                  []string
	Concurrency                                int
	FrameTimeout                               string
	ChannelBufferSize, InterfaceMTU, PacketMTU int
	NAT                                        NAT
	Connection                                 Connection
	Discovery                                  Discovery
	Ledger                                     Ledger
	Limit                                      ResourceLimit
	Privkey                                    []byte
	// PeerGuard (experimental)
	// enable peerguardian and add specific auth options
	PeerGuard PeerGuard

	Whitelist []multiaddr.Multiaddr
}

type PeerGuard struct {
	Enable      bool
	Relaxed     bool
	Autocleanup bool
	PeerGate    bool
	// AuthProviders in the freemap form:
	// ecdsa:
	//   private_key: "foo_bar"
	AuthProviders map[string]map[string]interface{}
	SyncInterval  time.Duration
}

type ResourceLimit struct {
	FileLimit   string
	LimitConfig *rcmgr.PartialLimitConfig
	Scope       string
	MaxConns    int
	StaticMin   int64
	StaticMax   int64
	Enable      bool
}

// Ledger is the ledger configuration structure
type Ledger struct {
	AnnounceInterval, SyncInterval time.Duration
	StateDir                       string
}

// Discovery allows to enable/disable discovery and
// set bootstrap peers
type Discovery struct {
	DHT, MDNS      bool
	BootstrapPeers []string
	Interval       time.Duration
}

// Connection is the configuration section
// relative to the connection services
type Connection struct {
	HolePunch bool
	AutoRelay bool

	AutoRelayDiscoveryInterval time.Duration
	StaticRelays               []string
	OnlyStaticRelays           bool

	PeerTable map[string]peer.ID

	MaxConnections int

	LowWater  int
	HighWater int

	// RelayService configures circuit-v2 relay-service resource limits
	// applied when this node acts as a relay for other peers.
	RelayService RelayService
}

// RelayService holds the circuit-v2 relay-service resource limits
// applied to this node when it serves as a relay for other peers.
//
// Higher values let cluster peers carry larger relayed transfers
// (e.g. model files for distributed inference) at the cost of a
// larger memory footprint per relay client. Lower values are safer
// for resource-constrained deployments.
type RelayService struct {
	// Disabled, when true, turns off the circuit-v2 relay *service* on
	// this node so it no longer accepts reservations from other peers.
	// The relay *client* (the ability to reserve slots on OTHER relays
	// via AutoRelay) stays on regardless — turning the service off does
	// not prevent this node from using third-party relays to traverse
	// its own NAT. Default (zero value) is false → service is offered,
	// preserving back-compat for programmatic callers.
	Disabled bool
	// MaxData is the byte limit (per direction) for a single relayed
	// connection before it is reset. libp2p default is 128 KiB.
	MaxData int64
	// MaxDuration is the time limit before a relayed connection is reset.
	// libp2p default is 2 minutes.
	MaxDuration time.Duration
	// MaxCircuits is the maximum number of open relay circuits per peer.
	// libp2p default is 16.
	MaxCircuits int
	// ReservationTTL is the duration of a relay reservation.
	// libp2p default is 1 hour.
	ReservationTTL time.Duration
	// BufferSize is the per-circuit relayed connection buffer size in bytes.
	// libp2p default is 2048.
	BufferSize int
}

// NAT is the structure relative to NAT configuration settings
// It allows to enable/disable the service and NAT mapping, and rate limiting too.
type NAT struct {
	Service   bool
	Map       bool
	RateLimit bool

	RateLimitGlobal, RateLimitPeer int
	RateLimitInterval              time.Duration
}

// Validate returns error if the configuration is not valid
func (c Config) Validate() error {
	if c.NetworkConfig == "" &&
		c.NetworkToken == "" {
		return fmt.Errorf("EDGEVPNCONFIG or EDGEVPNTOKEN not supplied. At least a config file is required")
	}
	return nil
}

func peers2List(peers []string) discovery.AddrList {
	addrsList := discovery.AddrList{}
	for _, p := range peers {
		addrsList.Set(p)
	}
	return addrsList
}

func peers2AddrInfo(peers []string) []peer.AddrInfo {
	addrsList := []peer.AddrInfo{}
	for _, p := range peers {
		pi, err := peer.AddrInfoFromString(p)
		if err == nil {
			addrsList = append(addrsList, *pi)
		}

	}
	return addrsList
}

var infiniteResourceLimits = rcmgr.InfiniteLimits.ToPartialLimitConfig().System

// ToOpts returns node and vpn options from a configuration
func (c Config) ToOpts(l log.StandardLogger) ([]node.Option, []vpn.Option, error) {

	if err := c.Validate(); err != nil {
		return nil, nil, err
	}

	config := c.NetworkConfig
	address := c.Address
	router := c.Router
	iface := c.Interface
	logLevel := c.LogLevel
	libp2plogLevel := c.Libp2pLogLevel
	dhtE, mDNS := c.Discovery.DHT, c.Discovery.MDNS

	ledgerState := c.Ledger.StateDir

	peers := c.Discovery.BootstrapPeers

	lvl, err := log.LevelFromString(logLevel)
	if err != nil {
		lvl = log.LevelError
	}

	// Use the caller-provided logger if given, otherwise create a default one.
	llger := l
	if llger == nil {
		llger = logger.New(lvl)
	}

	libp2plvl, err := log.LevelFromString(libp2plogLevel)
	if err != nil {
		libp2plvl = log.LevelFatal
	}

	token := c.NetworkToken

	addrsList := peers2List(peers)

	dhtOpts := []dht.Option{}

	if c.LowProfile {
		dhtOpts = append(dhtOpts, dht.BucketSize(20))
	}
	if len(c.DHTAnnounceMaddrs) > 0 {
		dhtOpts = append(dhtOpts, dht.AddressFilter(
			func(m []multiaddr.Multiaddr) []multiaddr.Multiaddr {
				return c.DHTAnnounceMaddrs
			},
		),
		)
	}

	d := discovery.NewDHT(dhtOpts...)
	m := &discovery.MDNS{}

	opts := []node.Option{
		node.ListenAddresses(c.ListenMaddrs...),
		node.WithDiscoveryInterval(c.Discovery.Interval),
		node.WithLedgerAnnounceTime(c.Ledger.AnnounceInterval),
		node.WithLedgerInterval(c.Ledger.SyncInterval),
		node.Logger(llger),
		node.WithDiscoveryBootstrapPeers(addrsList),
		node.WithBlacklist(c.Blacklist...),
		node.LibP2PLogLevel(libp2plvl),
		node.WithInterfaceAddress(address),
		node.WithSealer(&crypto.AESSealer{}),
		node.FromBase64(mDNS, dhtE, token, d, m),
		node.FromYaml(mDNS, dhtE, config, d, m),
	}

	for ip, peer := range c.Connection.PeerTable {
		opts = append(opts, node.WithStaticPeer(ip, peer))
	}

	if len(c.Privkey) > 0 {
		opts = append(opts, node.WithPrivKey(c.Privkey))
	}

	vpnOpts := []vpn.Option{
		vpn.WithConcurrency(c.Concurrency),
		vpn.WithInterfaceAddress(address),
		vpn.WithLedgerAnnounceTime(c.Ledger.AnnounceInterval),
		vpn.Logger(llger),
		vpn.WithTimeout(c.FrameTimeout),
		vpn.WithInterfaceType(water.TUN),
		vpn.NetLinkBootstrap(c.BootstrapIface),
		vpn.WithChannelBufferSize(c.ChannelBufferSize),
		vpn.WithInterfaceMTU(c.InterfaceMTU),
		vpn.WithPacketMTU(c.PacketMTU),
		vpn.WithRouterAddress(router),
		vpn.WithInterfaceName(iface),
	}

	libp2pOpts := []libp2p.Option{libp2p.UserAgent("edgevpn")}

	// AutoRelay section configuration
	if c.Connection.AutoRelay {
		relayOpts := []autorelay.Option{}

		staticRelays := c.Connection.StaticRelays

		if c.Connection.AutoRelayDiscoveryInterval == 0 {
			c.Connection.AutoRelayDiscoveryInterval = 5 * time.Minute
		}
		// If no relays are specified and no discovery interval, then just use default static relays (to be deprecated)

		relayOpts = append(relayOpts, autorelay.WithPeerSource(d.FindClosePeers(llger, c.Connection.OnlyStaticRelays, staticRelays...)))

		libp2pOpts = append(libp2pOpts,
			libp2p.EnableAutoRelay(relayOpts...))
	}

	// Offer the circuit-v2 relay SERVICE unless explicitly disabled. Any
	// publicly-reachable cluster peer can carry relayed traffic for
	// NAT-traversed peers that fail to DCUtR hole-punch. Resources are
	// tuned via Connection.RelayService. Set RelayService.Disabled=true
	// to opt out of serving as a relay (resource-constrained nodes,
	// edge devices, untrusted environments). The relay CLIENT (the
	// ability to reserve slots on OTHER relays via AutoRelay) stays on
	// regardless — disabling the service does not prevent this node
	// from using third-party relays to traverse its own NAT.
	if !c.Connection.RelayService.Disabled {
		libp2pOpts = append(libp2pOpts,
			libp2p.EnableRelayService(relayv2.WithResources(RelayServiceResources(c.Connection.RelayService))))
	}

	if c.NAT.RateLimit {
		libp2pOpts = append(libp2pOpts, libp2p.AutoNATServiceRateLimit(
			c.NAT.RateLimitGlobal,
			c.NAT.RateLimitPeer,
			c.NAT.RateLimitInterval,
		))
	}

	if c.Connection.LowWater != 0 && c.Connection.HighWater != 0 {
		llger.Infof("connmanager water limits low: %d high: %d", c.Connection.LowWater, c.Connection.HighWater)

		cm, err := connmanager.NewConnManager(
			c.Connection.LowWater,
			c.Connection.HighWater,
			connmanager.WithGracePeriod(80*time.Second),
		)
		if err != nil {
			llger.Fatal("could not create connection manager")
		}

		libp2pOpts = append(libp2pOpts, libp2p.ConnectionManager(cm))
	} else {
		llger.Infof("connmanager disabled")
	}

	if !c.Limit.Enable || runtime.GOOS == "darwin" {
		llger.Info("go-libp2p resource manager protection disabled")
		libp2pOpts = append(libp2pOpts, libp2p.ResourceManager(&network.NullResourceManager{}))
	} else {
		llger.Info("go-libp2p resource manager protection enabled")

		var limiter rcmgr.Limiter

		if c.Limit.FileLimit != "" {
			limitFile, err := os.Open(c.Limit.FileLimit)
			if err != nil {
				return opts, vpnOpts, err
			}
			defer limitFile.Close()

			l, err := rcmgr.NewDefaultLimiterFromJSON(limitFile)
			if err != nil {
				return opts, vpnOpts, err
			}

			limiter = l
		} else if c.Limit.MaxConns == -1 {
			llger.Infof("max connections: unlimited")

			scalingLimits := rcmgr.DefaultLimits

			// Add limits around included libp2p protocols
			libp2p.SetDefaultServiceLimits(&scalingLimits)

			// Turn the scaling limits into a concrete set of limits using `.AutoScale`. This
			// scales the limits proportional to your system memory.
			scaledDefaultLimits := scalingLimits.AutoScale()

			// Tweak certain settings
			cfg := rcmgr.PartialLimitConfig{
				System: rcmgr.ResourceLimits{
					Memory: rcmgr.Unlimited64,
					FD:     rcmgr.Unlimited,

					Conns:         rcmgr.Unlimited,
					ConnsInbound:  rcmgr.Unlimited,
					ConnsOutbound: rcmgr.Unlimited,

					Streams:         rcmgr.Unlimited,
					StreamsOutbound: rcmgr.Unlimited,
					StreamsInbound:  rcmgr.Unlimited,
				},

				// Transient connections won't cause any memory to be accounted for by the resource manager/accountant.
				// Only established connections do.
				// As a result, we can't rely on System.Memory to protect us from a bunch of transient connection being opened.
				// We limit the same values as the System scope, but only allow the Transient scope to take 25% of what is allowed for the System scope.
				Transient: rcmgr.ResourceLimits{
					Memory:        rcmgr.Unlimited64,
					FD:            rcmgr.Unlimited,
					Conns:         rcmgr.Unlimited,
					ConnsInbound:  rcmgr.Unlimited,
					ConnsOutbound: rcmgr.Unlimited,

					Streams:         rcmgr.Unlimited,
					StreamsInbound:  rcmgr.Unlimited,
					StreamsOutbound: rcmgr.Unlimited,
				},

				// Lets get out of the way of the allow list functionality.
				// If someone specified "Swarm.ResourceMgr.Allowlist" we should let it go through.
				AllowlistedSystem: infiniteResourceLimits,

				AllowlistedTransient: infiniteResourceLimits,

				// Keep it simple by not having Service, ServicePeer, Protocol, ProtocolPeer, Conn, or Stream limits.
				ServiceDefault: infiniteResourceLimits,

				ServicePeerDefault: infiniteResourceLimits,

				ProtocolDefault: infiniteResourceLimits,

				ProtocolPeerDefault: infiniteResourceLimits,

				Conn: infiniteResourceLimits,

				Stream: infiniteResourceLimits,

				// Limit the resources consumed by a peer.
				// This doesn't protect us against intentional DoS attacks since an attacker can easily spin up multiple peers.
				// We specify this limit against unintentional DoS attacks (e.g., a peer has a bug and is sending too much traffic intentionally).
				// In that case we want to keep that peer's resource consumption contained.
				// To keep this simple, we only constrain inbound connections and streams.
				PeerDefault: rcmgr.ResourceLimits{
					Memory:          rcmgr.Unlimited64,
					FD:              rcmgr.Unlimited,
					Conns:           rcmgr.Unlimited,
					ConnsInbound:    rcmgr.DefaultLimit,
					ConnsOutbound:   rcmgr.Unlimited,
					Streams:         rcmgr.Unlimited,
					StreamsInbound:  rcmgr.DefaultLimit,
					StreamsOutbound: rcmgr.Unlimited,
				},
			}

			// Create our limits by using our cfg and replacing the default values with values from `scaledDefaultLimits`
			limits := cfg.Build(scaledDefaultLimits)

			// The resource manager expects a limiter, se we create one from our limits.
			limiter = rcmgr.NewFixedLimiter(limits)

		} else if c.Limit.MaxConns != 0 {
			min := int64(1 << 30)
			max := int64(4 << 30)
			if c.Limit.StaticMin != 0 {
				min = c.Limit.StaticMin
			}
			if c.Limit.StaticMax != 0 {
				max = c.Limit.StaticMax
			}
			maxconns := int(c.Limit.MaxConns)

			defaultLimits := rcmgr.DefaultLimits.Scale(min+max/2, logScale(2*maxconns))
			llger.Infof("max connections: %d", c.Limit.MaxConns)

			limiter = rcmgr.NewFixedLimiter(defaultLimits)
		} else {
			llger.Infof("max connections: defaults limits")

			defaults := rcmgr.DefaultLimits
			def := &defaults

			libp2p.SetDefaultServiceLimits(def)
			limiter = rcmgr.NewFixedLimiter(def.AutoScale())
		}

		rc, err := rcmgr.NewResourceManager(limiter, rcmgr.WithAllowlistedMultiaddrs(c.Whitelist))
		if err != nil {
			llger.Fatal("could not create resource manager")
		}

		libp2pOpts = append(libp2pOpts, libp2p.ResourceManager(rc))
	}

	if c.Connection.HolePunch {
		libp2pOpts = append(libp2pOpts, libp2p.EnableHolePunching())
	}

	if c.NAT.Service {
		libp2pOpts = append(libp2pOpts, libp2p.EnableNATService())
	}

	if c.NAT.Map {
		libp2pOpts = append(libp2pOpts, libp2p.NATPortMap())
	}

	opts = append(opts, node.WithLibp2pOptions(libp2pOpts...))

	if ledgerState != "" {
		opts = append(opts, node.WithStore(blockchain.NewDiskStore(diskv.New(diskv.Options{
			BasePath:     ledgerState,
			CacheSizeMax: uint64(50), // 50MB
		}))))
	} else {
		opts = append(opts, node.WithStore(&blockchain.MemoryStore{}))
	}

	if c.PeerGuard.Enable {
		pg := trustzone.NewPeerGater(c.PeerGuard.Relaxed)
		dur := c.PeerGuard.SyncInterval

		// Build up the authproviders for the peerguardian
		aps := []trustzone.AuthProvider{}
		for ap, providerOpts := range c.PeerGuard.AuthProviders {
			a, err := authProvider(llger, ap, providerOpts)
			if err != nil {
				return opts, vpnOpts, fmt.Errorf("invalid authprovider: %w", err)
			}
			aps = append(aps, a)
		}

		pguardian := trustzone.NewPeerGuardian(llger, aps...)

		opts = append(opts,
			node.WithNetworkService(
				pg.UpdaterService(dur),
				pguardian.Challenger(dur, c.PeerGuard.Autocleanup),
			),
			node.EnableGenericHub,
			node.GenericChannelHandlers(pguardian.ReceiveMessage),
		)
		// We always pass a PeerGater such will be registered to the API if necessary
		opts = append(opts, node.WithPeerGater(pg))
		// IF it's not enabled, we just disable it right away.
		if !c.PeerGuard.PeerGate {
			pg.Disable()
		}
	}

	return opts, vpnOpts, nil
}

func authProvider(ll log.StandardLogger, s string, opts map[string]interface{}) (trustzone.AuthProvider, error) {
	switch strings.ToLower(s) {
	case "ecdsa":
		pk, exists := opts["private_key"]
		if !exists {
			return nil, fmt.Errorf("No private key provided")
		}
		return ecdsa.ECDSA521Provider(ll, fmt.Sprint(pk))
	}
	return nil, fmt.Errorf("not supported")
}

func logScale(val int) int {
	bitlen := bits.Len(uint(val))
	return 1 << bitlen
}

// Default circuit-v2 relay-service resource limits for edgevpn.
// These are deliberately much wider than libp2p's defaults so cluster
// peers can carry larger / longer relayed transfers (e.g. model files
// for distributed inference) when DCUtR hole-punching fails. Operators
// can override any of these via the Connection.RelayService config.
const (
	DefaultRelayServiceMaxData        int64         = 1 << 30 // 1 GiB
	DefaultRelayServiceMaxDuration    time.Duration = 30 * time.Minute
	DefaultRelayServiceMaxCircuits    int           = 64
	DefaultRelayServiceReservationTTL time.Duration = time.Hour
	DefaultRelayServiceBufferSize     int           = 64 << 10 // 64 KiB
)

// RelayServiceResources builds a relayv2.Resources struct from the
// configured knobs, falling back to edgevpn defaults (wider than
// libp2p's defaults) for any zero-valued field. libp2p's Resources
// struct is passed by value to relayv2.WithResources; it has no public
// constructor that merges with defaults, so we apply defaults here.
func RelayServiceResources(c RelayService) relayv2.Resources {
	res := relayv2.DefaultResources()

	if c.MaxCircuits > 0 {
		res.MaxCircuits = c.MaxCircuits
	} else {
		res.MaxCircuits = DefaultRelayServiceMaxCircuits
	}

	if c.BufferSize > 0 {
		res.BufferSize = c.BufferSize
	} else {
		res.BufferSize = DefaultRelayServiceBufferSize
	}

	if c.ReservationTTL > 0 {
		res.ReservationTTL = c.ReservationTTL
	} else {
		res.ReservationTTL = DefaultRelayServiceReservationTTL
	}

	limit := *res.Limit // copy so we don't mutate the DefaultLimit singleton
	if c.MaxDuration > 0 {
		limit.Duration = c.MaxDuration
	} else {
		limit.Duration = DefaultRelayServiceMaxDuration
	}
	if c.MaxData > 0 {
		limit.Data = c.MaxData
	} else {
		limit.Data = DefaultRelayServiceMaxData
	}
	res.Limit = &limit

	return res
}
