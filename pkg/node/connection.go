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

package node

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	mrand "math/rand"
	"net"

	internalCrypto "github.com/mudler/edgevpn/pkg/crypto"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	conngater "github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	hub "github.com/mudler/edgevpn/pkg/hub"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// Host returns the libp2p peer host
func (e *Node) Host() host.Host {
	return e.host
}

// ConnectionGater returns the underlying libp2p conngater
func (e *Node) ConnectionGater() *conngater.BasicConnectionGater {
	return e.cg
}

// BlockSubnet blocks the CIDR subnet from connections
func (e *Node) BlockSubnet(cidr string) error {
	// Avoid to loopback traffic by trying to connect to nodes in via VPN
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	return e.ConnectionGater().BlockSubnet(n)
}

func GenPrivKey(seed int64) (crypto.PrivKey, error) {
	var r io.Reader
	if seed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(seed))
	}
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 4096, r)
	return prvKey, err
}

func (e *Node) genHost(ctx context.Context) (host.Host, error) {
	var prvKey crypto.PrivKey

	opts := e.config.Options

	cg, err := conngater.NewBasicConnectionGater(nil)
	if err != nil {
		return nil, err
	}

	e.cg = cg

	if e.config.InterfaceAddress != "" {
		e.BlockSubnet(e.config.InterfaceAddress)
	}

	for _, b := range e.config.Blacklist {
		_, net, err := net.ParseCIDR(b)
		if err != nil {
			// Assume it's a peerID
			cg.BlockPeer(peer.ID(b))
		}
		if net != nil {
			cg.BlockSubnet(net)
		}
	}

	// generate privkey if not specified
	if len(e.config.PrivateKey) > 0 {
		prvKey, err = crypto.UnmarshalPrivateKey(e.config.PrivateKey)
	} else {
		prvKey, err = GenPrivKey(e.seed)
	}

	if err != nil {
		return nil, err
	}

	opts = append(opts, libp2p.ConnectionGater(cg), libp2p.Identity(prvKey))
	// Do not enable metrics for now
	opts = append(opts, libp2p.DisableMetrics())

	addrs := []multiaddr.Multiaddr{}
	for _, l := range e.config.ListenAddresses {
		addrs = append(addrs, []multiaddr.Multiaddr(l)...)
	}
	opts = append(opts, libp2p.ListenAddrs(addrs...))

	for _, d := range e.config.ServiceDiscovery {
		opts = append(opts, d.Option(ctx))
	}

	opts = append(opts, e.config.AdditionalOptions...)

	if e.config.Insecure {
		e.config.Logger.Info("Disabling Security transport layer")
		opts = append(opts, libp2p.NoSecurity)
	}

	opts = append(opts, FallbackDefaults)

	return libp2p.NewWithoutDefaults(opts...)
}

// FallbackDefaults applies default options to the libp2p node if and only if no
// other relevant options have been applied. will be appended to the options
// passed into New.
var FallbackDefaults libp2p.Option = func(cfg *libp2p.Config) error {
	for _, def := range defaults {
		if !def.fallback(cfg) {
			continue
		}
		if err := cfg.Apply(def.opt); err != nil {
			return err
		}
	}
	return nil
}

var defaultUDPBlackHoleDetector = func(cfg *libp2p.Config) error {
	// A black hole is a binary property. On a network if UDP dials are blocked, all dials will
	// fail. So a low success rate of 5 out 100 dials is good enough.
	return cfg.Apply(libp2p.UDPBlackHoleSuccessCounter(&swarm.BlackHoleSuccessCounter{N: 100, MinSuccesses: 5, Name: "UDP"}))
}

var defaultIPv6BlackHoleDetector = func(cfg *libp2p.Config) error {
	// A black hole is a binary property. On a network if there is no IPv6 connectivity, all
	// dials will fail. So a low success rate of 5 out 100 dials is good enough.
	return cfg.Apply(libp2p.IPv6BlackHoleSuccessCounter(&swarm.BlackHoleSuccessCounter{N: 100, MinSuccesses: 5, Name: "IPv6"}))
}

// Complete list of default options and when to fallback on them.
//
// Please *DON'T* specify default options any other way. Putting this all here
// makes tracking defaults *much* easier.
// https://github.com/libp2p/go-libp2p/blob/2209ae05976df6a1cc2631c961f57549d109008c/defaults.go#L227
var defaults = []struct {
	fallback func(cfg *libp2p.Config) bool
	opt      libp2p.Option
}{
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.Transports == nil && cfg.ListenAddrs == nil },
		opt:      libp2p.DefaultListenAddrs,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.Transports == nil && cfg.PSK == nil },
		opt:      libp2p.DefaultTransports,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.Transports == nil && cfg.PSK != nil },
		opt:      libp2p.DefaultPrivateTransports,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.Muxers == nil },
		opt:      libp2p.DefaultMuxers,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return !cfg.Insecure && cfg.SecurityTransports == nil },
		opt:      libp2p.DefaultSecurity,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.PeerKey == nil },
		opt:      libp2p.RandomIdentity,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.Peerstore == nil },
		opt:      libp2p.DefaultPeerstore,
	},
	{
		fallback: func(cfg *libp2p.Config) bool { return !cfg.RelayCustom },
		opt:      libp2p.DefaultEnableRelay,
	},
	//{
	//	fallback: func(cfg *libp2p.Config) bool { return cfg.ResourceManager == nil },
	//	opt:      libp2p.DefaultResourceManager,
	//},
	{
		fallback: func(cfg *libp2p.Config) bool { return cfg.ConnManager == nil },
		// Filling the ConnManager is required, even if its a null one as libp2p will call functions of the
		// libp2p.Config.ConnManager so we need to have it not nil
		opt: libp2p.ConnectionManager(connmgr.NullConnMgr{}),
	},
	{
		fallback: func(cfg *libp2p.Config) bool {
			return !cfg.CustomUDPBlackHoleSuccessCounter && cfg.UDPBlackHoleSuccessCounter == nil
		},
		opt: defaultUDPBlackHoleDetector,
	},
	{
		fallback: func(cfg *libp2p.Config) bool {
			return !cfg.CustomIPv6BlackHoleSuccessCounter && cfg.IPv6BlackHoleSuccessCounter == nil
		},
		opt: defaultIPv6BlackHoleDetector,
	},
	//{
	//	fallback: func(cfg *libp2p.Config) bool { return !cfg.DisableMetrics && cfg.PrometheusRegisterer == nil },
	//	opt:      libp2p.DefaultPrometheusRegisterer,
	//},
}

func (e *Node) sealkey() string {
	return internalCrypto.MD5(internalCrypto.TOTP(sha256.New, e.config.SealKeyLength, e.config.SealKeyInterval, e.config.ExchangeKey))
}

func (e *Node) handleEvents(ctx context.Context, inputChannel chan *hub.Message, roomMessages chan *hub.Message, pub func(*hub.Message) error, handlers []Handler, peerGater bool) {
	for {
		select {
		case m := <-inputChannel:
			if m == nil {
				continue
			}
			c := m.Copy()
			str, err := e.config.Sealer.Seal(c.Message, e.sealkey())
			if err != nil {
				e.config.Logger.Warnf("%w from %s", err.Error(), c.SenderID)
			}
			c.Message = str

			if err := pub(c); err != nil {
				e.config.Logger.Warnf("publish error: %s", err)
			}

		case m := <-roomMessages:
			if m == nil {
				continue
			}

			if peerGater {
				if e.config.PeerGater != nil && e.config.PeerGater.Gate(e, peer.ID(m.SenderID)) {
					e.config.Logger.Warnf("gated message from %s", m.SenderID)
					continue
				}
			}
			if len(e.config.PeerTable) > 0 {
				found := false
				for _, p := range e.config.PeerTable {
					if p.String() == peer.ID(m.SenderID).String() {
						found = true
					}
				}
				if !found {
					e.config.Logger.Warnf("gated message from %s - not present in peertable", m.SenderID)
					continue
				}
			}

			c := m.Copy()
			str, err := e.config.Sealer.Unseal(c.Message, e.sealkey())
			if err != nil {
				e.config.Logger.Warnf("%w from %s", err.Error(), c.SenderID)
			}
			c.Message = str
			e.handleReceivedMessage(c, handlers, inputChannel)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Node) handleReceivedMessage(m *hub.Message, handlers []Handler, c chan *hub.Message) {
	for _, h := range handlers {
		if err := h(e.ledger, m, c); err != nil {
			e.config.Logger.Warnf("handler error: %s", err)
		}
	}
}
