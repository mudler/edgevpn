// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package node

import (
	"encoding/base64"
	"io/ioutil"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/mudler/edgevpn/pkg/blockchain"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/utils"
	"github.com/pkg/errors"
	"github.com/xlzd/gotp"
	"gopkg.in/yaml.v2"
)

// WithLibp2pOptions Overrides defaults options
func WithLibp2pOptions(i ...libp2p.Option) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Options = i
		return nil
	}
}

func WithLibp2pAdditionalOptions(i ...libp2p.Option) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.AdditionalOptions = append(cfg.Options, i...)
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

func (y YAMLConnectionConfig) copy(mdns, dht bool, cfg *Config) {
	d := &discovery.DHT{
		RefreshDiscoveryTime: cfg.DiscoveryInterval,
		OTPInterval:          y.OTP.DHT.Interval,
		OTPKey:               y.OTP.DHT.Key,
		KeyLength:            y.OTP.DHT.Length,
		RendezvousString:     y.Rendezvous,
		BootstrapPeers:       cfg.DiscoveryBootstrapPeers,
	}
	m := &discovery.MDNS{DiscoveryServiceTag: y.MDNS}
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
}

const defaultKeyLength = 32

func GenerateNewConnectionData(i ...int) *YAMLConnectionConfig {
	defaultInterval := 9000
	maxMessSize := 20 << 20 // 20MB
	if len(i) >= 2 {
		defaultInterval = i[0]
		maxMessSize = i[1]
	} else if len(i) == 1 {
		defaultInterval = i[0]
	}

	return &YAMLConnectionConfig{
		MaxMessageSize: maxMessSize,
		RoomName:       utils.RandStringRunes(defaultKeyLength),
		Rendezvous:     utils.RandStringRunes(defaultKeyLength),
		MDNS:           utils.RandStringRunes(defaultKeyLength),
		OTP: OTP{
			DHT: OTPConfig{
				Key:      gotp.RandomSecret(defaultKeyLength),
				Interval: defaultInterval,
				Length:   defaultKeyLength,
			},
			Crypto: OTPConfig{
				Key:      gotp.RandomSecret(defaultKeyLength),
				Interval: defaultInterval,
				Length:   defaultKeyLength,
			},
		},
	}
}

func FromYaml(enablemDNS, enableDHT bool, path string) func(cfg *Config) error {
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

		t.copy(enablemDNS, enableDHT, cfg)
		return nil
	}
}

func FromBase64(enablemDNS, enableDHT bool, bb string) func(cfg *Config) error {
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
		t.copy(enablemDNS, enableDHT, cfg)
		return nil
	}
}
