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

	hub "github.com/mudler/edgevpn/pkg/hub"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/xlzd/gotp"
)

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

	opts := []libp2p.Option{
		libp2p.ListenAddrs([]multiaddr.Multiaddr(e.config.ListenAddresses)...),
		libp2p.Identity(prvKey),
		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.ConnectionGater(cg),
	}

	for _, d := range e.config.ServiceDiscovery {
		opts = append(opts, d.Option(ctx))
	}

	opts = append(opts, e.config.Options...)

	if e.config.Insecure {
		opts = append(opts, libp2p.NoSecurity)
	}

	return libp2p.New(ctx, opts...)
}

func (e *EdgeVPN) sealkey() string {
	totp := gotp.NewTOTP(e.config.ExchangeKey, 6, e.config.SealKeyInterval, nil)
	return totp.Now()
}

func (e *EdgeVPN) handleEvents(ctx context.Context) {
	for {
		select {
		case m := <-e.inputCh:
			if err := m.Seal(e.sealkey()); err != nil {
				e.config.Logger.Sugar().Warn(err.Error())
			}
			e.handleOutgoingMessage(m)
		case m := <-e.HubRoom.Messages:
			if err := m.Unseal(e.sealkey()); err != nil {
				e.config.Logger.Sugar().Warn(err.Error())
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
		h(m)
	}
}

func (e *EdgeVPN) handleOutgoingMessage(m *hub.Message) {
	err := e.HubRoom.PublishMessage(m)
	if err != nil {
		e.config.Logger.Sugar().Warnf("publish error: %s", err)
	}
}
