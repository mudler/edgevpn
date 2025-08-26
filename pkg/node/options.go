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
	"encoding/base64"
	"io/ioutil"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/utils"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// WithLibp2pOptions Overrides defaults options
func WithLibp2pOptions(i ...libp2p.Option) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Options = i
		return nil
	}
}

func WithSealer(i Sealer) Option {
	return func(cfg *Config) error {
		cfg.Sealer = i
		return nil
	}
}

func WithLibp2pAdditionalOptions(i ...libp2p.Option) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.AdditionalOptions = append(cfg.AdditionalOptions, i...)
		return nil
	}
}

func WithNetworkService(ns ...NetworkService) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.NetworkServices = append(cfg.NetworkServices, ns...)
		return nil
	}
}

func WithInterfaceAddress(i string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.InterfaceAddress = i
		return nil
	}
}

func WithBlacklist(i ...string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Blacklist = i
		return nil
	}
}

func Logger(l log.StandardLogger) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Logger = l
		return nil
	}
}
func WithStore(s blockchain.Store) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Store = s
		return nil
	}
}

// Handlers adds a handler to the list that is called on each received message
func Handlers(h ...Handler) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Handlers = append(cfg.Handlers, h...)
		return nil
	}
}

// GenericChannelHandlers adds a handler to the list that is called on each received message in the generic channel (not the one allocated for the blockchain)
func GenericChannelHandlers(h ...Handler) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.GenericChannelHandler = append(cfg.GenericChannelHandler, h...)
		return nil
	}
}

// WithStreamHandler adds a handler to the list that is called on each received message
func WithStreamHandler(id protocol.Protocol, h StreamHandler) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.StreamHandlers[id] = h
		return nil
	}
}

// DiscoveryService Adds the service given as argument to the discovery services
func DiscoveryService(s ...ServiceDiscovery) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.ServiceDiscovery = append(cfg.ServiceDiscovery, s...)
		return nil
	}
}

// EnableGenericHub enables an additional generic hub between peers.
// This can be used to exchange messages between peers that are not related to any
// blockchain event. For instance, messages could be used for authentication, or for other sort
// of application.
var EnableGenericHub = func(cfg *Config) error {
	cfg.GenericHub = true
	return nil
}

func ListenAddresses(ss ...string) func(cfg *Config) error {
	return func(cfg *Config) error {
		for _, s := range ss {
			a := &discovery.AddrList{}
			err := a.Set(s)
			if err != nil {
				return err
			}
			cfg.ListenAddresses = append(cfg.ListenAddresses, *a)
		}
		return nil
	}
}

func Insecure(b bool) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Insecure = b
		return nil
	}
}

func ExchangeKeys(s string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.ExchangeKey = s
		return nil
	}
}

func RoomName(s string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.RoomName = s
		return nil
	}
}

func SealKeyInterval(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.SealKeyInterval = i
		return nil
	}
}

func SealKeyLength(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.SealKeyLength = i
		return nil
	}
}

func LibP2PLogLevel(l log.LogLevel) func(cfg *Config) error {
	return func(cfg *Config) error {
		log.SetAllLoggers(l)
		return nil
	}
}

func MaxMessageSize(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.MaxMessageSize = i
		return nil
	}
}

func WithPeerGater(d Gater) Option {
	return func(cfg *Config) error {
		cfg.PeerGater = d
		return nil
	}
}

func WithLedgerAnnounceTime(t time.Duration) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.LedgerAnnounceTime = t
		return nil
	}
}

func WithLedgerInterval(t time.Duration) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.LedgerSyncronizationTime = t
		return nil
	}
}

func WithDiscoveryInterval(t time.Duration) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.DiscoveryInterval = t
		return nil
	}
}

func WithDiscoveryBootstrapPeers(a discovery.AddrList) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.DiscoveryBootstrapPeers = a
		return nil
	}
}

func WithPrivKey(b []byte) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.PrivateKey = b
		return nil
	}
}

func WithStaticPeer(ip string, p peer.ID) func(cfg *Config) error {
	return func(cfg *Config) error {
		if cfg.PeerTable == nil {
			cfg.PeerTable = make(map[string]peer.ID)
		}
		cfg.PeerTable[ip] = p
		return nil
	}
}

type OTPConfig struct {
	Interval int    `yaml:"interval"`
	Key      string `yaml:"key"`
	Length   int    `yaml:"length"`
}

type OTP struct {
	DHT    OTPConfig `yaml:"dht"`
	Crypto OTPConfig `yaml:"crypto"`
}

type YAMLConnectionConfig struct {
	OTP OTP `yaml:"otp"`

	RoomName       string `yaml:"room"`
	Rendezvous     string `yaml:"rendezvous"`
	MDNS           string `yaml:"mdns"`
	MaxMessageSize int    `yaml:"max_message_size"`

	TrustedPeerIDS     []string `yaml:"trusted_peer_ids"`
	ProtectedStoreKeys []string `yaml:"protected_store_keys"`
}

// Base64 returns the base64 string representation of the connection
func (y YAMLConnectionConfig) Base64() string {
	bytesData, _ := yaml.Marshal(y)
	return base64.StdEncoding.EncodeToString(bytesData)
}

// YAML returns the connection config as yaml string
func (y YAMLConnectionConfig) YAML() string {
	bytesData, _ := yaml.Marshal(y)
	return string(bytesData)
}

func (y YAMLConnectionConfig) copy(mdns, dht bool, cfg *Config, d *discovery.DHT, m *discovery.MDNS) {
	if d == nil {
		d = discovery.NewDHT()
	}
	if m == nil {
		m = &discovery.MDNS{}
	}

	d.RefreshDiscoveryTime = cfg.DiscoveryInterval
	d.OTPInterval = y.OTP.DHT.Interval
	d.OTPKey = y.OTP.DHT.Key
	d.KeyLength = y.OTP.DHT.Length
	d.RendezvousString = y.Rendezvous
	d.BootstrapPeers = cfg.DiscoveryBootstrapPeers

	m.DiscoveryServiceTag = y.MDNS
	cfg.ExchangeKey = y.OTP.Crypto.Key
	cfg.RoomName = y.RoomName
	cfg.SealKeyInterval = y.OTP.Crypto.Interval
	//	cfg.ServiceDiscovery = []ServiceDiscovery{d, m}
	if mdns {
		cfg.ServiceDiscovery = append(cfg.ServiceDiscovery, m)
	}
	if dht {
		cfg.ServiceDiscovery = append(cfg.ServiceDiscovery, d)
	}
	cfg.SealKeyLength = y.OTP.Crypto.Length
	cfg.MaxMessageSize = y.MaxMessageSize
	cfg.TrustedPeerIDS = y.TrustedPeerIDS
	cfg.ProtectedStoreKeys = y.ProtectedStoreKeys
}

const defaultKeyLength = 43

func GenerateNewConnectionData(i ...int) *YAMLConnectionConfig {
	defaultInterval := 9000
	maxMessSize := 20 << 20 // 20MB
	keyLength := defaultKeyLength

	if len(i) >= 3 {
		keyLength = i[2]
		defaultInterval = i[0]
		maxMessSize = i[1]
	} else if len(i) >= 2 {
		defaultInterval = i[0]
		maxMessSize = i[1]
	} else if len(i) == 1 {
		defaultInterval = i[0]
	}

	return &YAMLConnectionConfig{
		MaxMessageSize: maxMessSize,
		RoomName:       utils.RandStringRunes(keyLength),
		Rendezvous:     utils.RandStringRunes(keyLength),
		MDNS:           utils.RandStringRunes(keyLength),
		OTP: OTP{
			DHT: OTPConfig{
				Key:      utils.RandStringRunes(keyLength),
				Interval: defaultInterval,
				Length:   defaultKeyLength,
			},
			Crypto: OTPConfig{
				Key:      utils.RandStringRunes(keyLength),
				Interval: defaultInterval,
				Length:   defaultKeyLength,
			},
		},
	}
}

func FromYaml(enablemDNS, enableDHT bool, path string, d *discovery.DHT, m *discovery.MDNS) func(cfg *Config) error {
	return func(cfg *Config) error {
		if len(path) == 0 {
			return nil
		}
		t := YAMLConnectionConfig{}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrap(err, "reading yaml file")
		}

		if err := yaml.Unmarshal(data, &t); err != nil {
			return errors.Wrap(err, "parsing yaml")
		}

		t.copy(enablemDNS, enableDHT, cfg, d, m)
		return nil
	}
}

func FromBase64(enablemDNS, enableDHT bool, bb string, d *discovery.DHT, m *discovery.MDNS) func(cfg *Config) error {
	return func(cfg *Config) error {
		if len(bb) == 0 {
			return nil
		}
		configDec, err := base64.StdEncoding.DecodeString(bb)
		if err != nil {
			return err
		}
		t := YAMLConnectionConfig{}

		if err := yaml.Unmarshal(configDec, &t); err != nil {
			return errors.Wrap(err, "parsing yaml")
		}
		t.copy(enablemDNS, enableDHT, cfg, d, m)
		return nil
	}
}
