package edgevpn

import (
	"io/ioutil"

	"github.com/ipfs/go-log/v2"
	discovery "github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/utils"
	"github.com/pkg/errors"
	"github.com/songgao/water"
	"github.com/xlzd/gotp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

func IfaceWriter(i *water.Interface) Handler {
	return Handler(func(m *hub.Message) error {
		i.Write([]byte(m.Message))
		return nil
	})
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
		for _, cc := range h {
			cfg.Handlers = append(cfg.Handlers, cc)
		}
		return nil
	}
}

// DiscoveryService Adds the service given as argument to the discovery services
func DiscoveryService(s ...ServiceDiscovery) func(cfg *Config) error {
	return func(cfg *Config) error {
		for _, cc := range s {
			cfg.ServiceDiscovery = append(cfg.ServiceDiscovery, cc)
		}
		return nil
	}
}

func ListenAddresses(s string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.ListenAddresses.Set(s)
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

func WithMTU(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.MTU = i
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

func (y YAMLConnectionConfig) Fill(cfg *Config) {
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
		t.Fill(cfg)
		return nil
	}
}
