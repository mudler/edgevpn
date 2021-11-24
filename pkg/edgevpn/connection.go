package edgevpn

import (
	"context"
	"crypto/rand"
	"io"
	mrand "math/rand"
	"net"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	conngater "github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/mudler/edgevpn/pkg/discovery"
	hub "github.com/mudler/edgevpn/pkg/hub"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/xlzd/gotp"
)

const (
	Protocol        = "/edgevpn/0.1"
	ServiceProtocol = "/edgevpn/service/0.1"
	FileProtocol    = "/edgevpn/file/0.1"
)

var defaultLibp2pOptions = []libp2p.Option{
	libp2p.EnableNATService(),
	libp2p.NATPortMap(),
}

func (e *EdgeVPN) Host() host.Host {
	return e.host
}

func (e *EdgeVPN) genHost(ctx context.Context) (host.Host, error) {
	var r io.Reader
	if e.seed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(e.seed))
	}
	// Creates a new RSA key pair for this host.
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	opts := defaultLibp2pOptions

	if len(e.config.Options) != 0 {
		opts = e.config.Options
	}

	if e.config.InterfaceAddress != "" {
		// Avoid to loopback traffic by trying to connect to nodes in via VPN
		_, vpnNetwork, err := net.ParseCIDR(e.config.InterfaceAddress)
		if err != nil {
			return nil, err
		}
		cg, err := conngater.NewBasicConnectionGater(nil)
		if err != nil {
			return nil, err
		}
		if err := cg.BlockSubnet(vpnNetwork); err != nil {
			return nil, err
		}
		opts = append(opts, libp2p.ConnectionGater(cg))
	}

	opts = append(opts, libp2p.Identity(prvKey))

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

	for _, d := range e.config.ServiceDiscovery {
		switch d.(type) {
		case *discovery.DHT:
			opts = append(opts, libp2p.EnableAutoRelay())
			e.config.Logger.Info("DHT Discovery enabled, enabling autorelay")
		}
	}

	opts = append(opts, libp2p.FallbackDefaults)

	return libp2p.New(ctx, opts...)
}

func (e *EdgeVPN) sealkey() string {
	return gotp.NewTOTP(e.config.ExchangeKey, e.config.SealKeyLength, e.config.SealKeyInterval, nil).Now()
}

func (e *EdgeVPN) handleEvents(ctx context.Context) {
	for {
		select {
		case m := <-e.inputCh:
			if err := m.Seal(e.sealkey()); err != nil {
				e.config.Logger.Warn(err.Error())
			}
			e.handleOutgoingMessage(m)
		case m := <-e.HubRoom.Messages:
			if err := m.Unseal(e.sealkey()); err != nil {
				e.config.Logger.Warn(err.Error())
			}
			e.handleReceivedMessage(m)
		case <-ctx.Done():
			return
		case <-e.doneCh:
			return
		}
	}
}

func (e *EdgeVPN) handleReceivedMessage(m *hub.Message) {
	for _, h := range e.config.Handlers {
		if err := h(m); err != nil {
			e.config.Logger.Warnf("handler error: %s", err)
		}
	}
}

func (e *EdgeVPN) handleOutgoingMessage(m *hub.Message) {
	err := e.HubRoom.PublishMessage(m)
	if err != nil {
		e.config.Logger.Warnf("publish error: %s", err)
	}
}
