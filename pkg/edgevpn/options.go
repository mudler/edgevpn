package edgevpn

import (
	"encoding/base64"
	"io/ioutil"

	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/utils"
	"github.com/pkg/errors"
	"github.com/songgao/water"
	"github.com/xlzd/gotp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

// WithLibp2pOptions Overrides defaults options
func WithLibp2pOptions(i ...libp2p.Option) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Options = append(cfg.Options, i...)
		return nil
	}
}

func WithLibp2pAdditionalOptions(i ...libp2p.Option) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.AdditionalOptions = append(cfg.Options, i...)
		return nil
	}
}

func WithInterface(i *water.Interface) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Interface = i
		return nil
	}
}

func WithInterfaceAddress(i string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.InterfaceAddress = i
		return nil
	}
}

func WithInterfaceMTU(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.InterfaceMTU = i
		return nil
	}
}

func WithMaxBlockChainSize(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.MaxBlockChainLength = i
		return nil
	}
}

func WithPacketMTU(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.MTU = i
		return nil
	}
}

func WithInterfaceType(d water.DeviceType) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.DeviceType = d
		return nil
	}
}

func WithInterfaceName(i string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.InterfaceName = i
		return nil
	}
}

func Logger(l *zap.Logger) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Logger = l
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

func NetLinkBootstrap(b bool) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.NetLinkBootstrap = b
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

func LogLevel(l log.LogLevel) func(cfg *Config) error {
	return func(cfg *Config) error {
		log.SetAllLoggers(l)
		log.SetLogLevel("edgevpn", zapcore.Level(l).String())
		log.SetLogLevel("pubsub", "fatal")
		log.SetLogLevel("dht/RtRefreshManager", "fatal")
		log.SetLogLevel("swarm2", "fatal")
		log.SetLogLevel("basichost", "fatal")
		log.SetLogLevel("relay", "fatal")
		log.SetLogLevel("dht", "fatal")
		log.SetLogLevel("mdns", "fatal")
		log.SetLogLevel("net/identify", "fatal")
		return nil
	}
}

func MaxMessageSize(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.MaxMessageSize = i
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

	RoomName   string `yaml:"room"`
	Rendezvous string `yaml:"rendezvous"`
	MDNS       string `yaml:"mdns"`
}

func (y YAMLConnectionConfig) copy(cfg *Config) {
	d := &discovery.DHT{
		RefreshDiscoveryTime: 60,
		OTPInterval:          y.OTP.DHT.Interval,
		OTPKey:               y.OTP.DHT.Key,
		KeyLength:            y.OTP.DHT.Length,
		RendezvousString:     y.Rendezvous,
	}
	m := &discovery.MDNS{DiscoveryServiceTag: y.MDNS}
	cfg.ExchangeKey = y.OTP.Crypto.Key
	cfg.RoomName = y.RoomName
	cfg.SealKeyInterval = y.OTP.Crypto.Interval
	cfg.ServiceDiscovery = []ServiceDiscovery{d, m}
	cfg.SealKeyLength = y.OTP.Crypto.Length
}

func GenerateNewConnectionData() (*YAMLConnectionConfig, error) {
	config := YAMLConnectionConfig{}

	config.RoomName = utils.RandStringRunes(23)
	config.Rendezvous = utils.RandStringRunes(23)
	config.MDNS = utils.RandStringRunes(23)

	config.OTP.DHT.Key = gotp.RandomSecret(16)
	config.OTP.Crypto.Key = gotp.RandomSecret(16)
	config.OTP.DHT.Interval = 9000
	config.OTP.Crypto.Interval = 9000
	config.OTP.Crypto.Length = 12
	config.OTP.DHT.Length = 12

	return &config, nil
}

func FromYaml(path string) func(cfg *Config) error {
	return func(cfg *Config) error {
		t := YAMLConnectionConfig{}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrap(err, "reading yaml file")
		}

		if err := yaml.Unmarshal(data, &t); err != nil {
			return errors.Wrap(err, "parsing yaml")
		}

		t.copy(cfg)
		return nil
	}
}

func FromBase64(bb string) func(cfg *Config) error {
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
		t.copy(cfg)
		return nil
	}
}
