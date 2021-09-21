package edgevpn

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	hub "github.com/mudler/edgevpn/pkg/hub"
	"github.com/songgao/water"
	"go.uber.org/zap"
)

type Config struct {
	// ExchangeKey is a Symmetric key used to seal the messages
	ExchangeKey string

	// RoomName is the gossip room where all peers are subscribed to
	RoomName string

	// ListenAddresses is the discovery peer initial bootstrap addresses
	ListenAddresses discovery.AddrList

	// Insecure disables secure p2p e2e encrypted communication
	Insecure bool

	// Handlers are a list of handlers subscribed to messages received by the vpn interface
	Handlers []Handler

	MaxMessageSize   int
	MTU              int
	SealKeyInterval  int
	Interface        *water.Interface
	InterfaceName    string
	InterfaceAddress string
	InterfaceMTU     int
	DeviceType       water.DeviceType
	ServiceDiscovery []ServiceDiscovery
	Logger           *zap.Logger

	SealKeyLength int

	NetLinkBootstrap bool

	// Handle is a handle consumed by HumanInterfaces to handle received messages
	Handle  func(bool, *hub.Message)
	Options []libp2p.Option
}

type Handler func(*hub.Message) error

type ServiceDiscovery interface {
	Run(*zap.Logger, context.Context, host.Host) error
	Option(context.Context) func(c *libp2p.Config) error
}

type Option func(cfg *Config) error

// Apply applies the given options to the config, returning the first error
// encountered (if any).
func (cfg *Config) Apply(opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}
