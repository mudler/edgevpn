package edgevpn

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	hub "github.com/mudler/edgevpn/pkg/hub"
	"github.com/songgao/water"
)

type Config struct {
	// ExchangeKey is a Symmetric key used to seal the messages
	ExchangeKey string

	// RoomName is the gossip room where all peers are subscribed to
	RoomName string

	// ListenAddresses is the discovery peer initial bootstrap addresses
	ListenAddresses []discovery.AddrList

	// Insecure disables secure p2p e2e encrypted communication
	Insecure bool

	// Handlers are a list of handlers subscribed to messages received by the vpn interface
	Handlers []Handler

	MaxMessageSize   int
	SealKeyInterval  int
	Interface        *water.Interface
	InterfaceName    string
	InterfaceAddress string
	InterfaceMTU     int
	MTU              int
	DeviceType       water.DeviceType
	ServiceDiscovery []ServiceDiscovery
	Logger           log.StandardLogger

	SealKeyLength int

	MaxBlockChainLength int

	NetLinkBootstrap bool

	// Handle is a handle consumed by HumanInterfaces to handle received messages
	Handle                     func(bool, *hub.Message)
	StreamHandlers             map[protocol.ID]StreamHandler
	AdditionalOptions, Options []libp2p.Option

	LedgerSyncronizationTime, LedgerAnnounceTime time.Duration
}

type StreamHandler func(stream network.Stream)

type Handler func(*hub.Message) error

type ServiceDiscovery interface {
	Run(log.StandardLogger, context.Context, host.Host) error
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
