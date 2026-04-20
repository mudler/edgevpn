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
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/mudler/edgevpn/pkg/blockchain"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	hub "github.com/mudler/edgevpn/pkg/hub"
	protocol "github.com/mudler/edgevpn/pkg/protocol"
)

// Config is the node configuration
type Config struct {
	// ExchangeKey is a Symmetric key used to seal the messages
	ExchangeKey string

	// RoomName is the OTP token gossip room where all peers are subscribed to
	RoomName string

	// ListenAddresses is the discovery peer initial bootstrap addresses
	ListenAddresses []discovery.AddrList

	// Insecure disables secure p2p e2e encrypted communication
	Insecure bool

	// Handlers are a list of handlers subscribed to messages received by the vpn interface
	Handlers, GenericChannelHandler []Handler

	MaxMessageSize  int
	SealKeyInterval int

	ServiceDiscovery []ServiceDiscovery
	NetworkServices  []NetworkService
	Logger           log.StandardLogger

	SealKeyLength    int
	InterfaceAddress string

	Store blockchain.Store

	// Handle is a handle consumed by HumanInterfaces to handle received messages
	Handle                     func(bool, *hub.Message)
	StreamHandlers             map[protocol.Protocol]StreamHandler
	AdditionalOptions, Options []libp2p.Option

	DiscoveryInterval, LedgerSyncronizationTime, LedgerAnnounceTime time.Duration
	DiscoveryBootstrapPeers                                         discovery.AddrList

	Whitelist, Blacklist []string

	// GenericHub enables generic hub
	GenericHub bool

	PrivateKey []byte
	PeerTable  map[string]peer.ID

	Sealer    Sealer
	PeerGater Gater

	TrustedPeerIDS     []string
	ProtectedStoreKeys []string
}

type Gater interface {
	Gate(*Node, peer.ID) bool
	Enable()
	Disable()
	Enabled() bool
}

type Sealer interface {
	Seal(string, string) (string, error)
	Unseal(string, string) (string, error)
}

// NetworkService is a service running over the network. It takes a context, a node and a ledger
type NetworkService func(context.Context, Config, *Node, *blockchain.Ledger) error

type StreamHandler func(*Node, *blockchain.Ledger) func(stream network.Stream)

type Handler func(*blockchain.Ledger, *hub.Message, chan *hub.Message) error

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
