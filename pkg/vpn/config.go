// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package vpn

import (
	"time"

	"github.com/fumiama/water"
	"github.com/ipfs/go-log"
)

type Config struct {
	Interface        *water.Interface
	InterfaceName    string
	InterfaceAddress string
	RouterAddress    string
	InterfaceMTU     int
	MTU              int
	DeviceType       water.DeviceType

	LedgerAnnounceTime time.Duration
	Logger             log.StandardLogger

	NetLinkBootstrap bool

	// Frame timeout
	Timeout time.Duration

	Concurrency       int
	ChannelBufferSize int
	MaxStreams        int
	lowProfile        bool
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

func WithMaxStreams(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.MaxStreams = i
		return nil
	}
}

var LowProfile Option = func(cfg *Config) error {
	cfg.lowProfile = true

	return nil
}

func WithInterface(i *water.Interface) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Interface = i
		return nil
	}
}

func NetLinkBootstrap(b bool) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.NetLinkBootstrap = b
		return nil
	}
}

func WithTimeout(s string) Option {
	return func(cfg *Config) error {
		d, err := time.ParseDuration(s)
		cfg.Timeout = d
		return err
	}
}

func Logger(l log.StandardLogger) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.Logger = l
		return nil
	}
}
func WithRouterAddress(i string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.RouterAddress = i
		return nil
	}
}

func WithLedgerAnnounceTime(t time.Duration) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.LedgerAnnounceTime = t
		return nil
	}
}

func WithConcurrency(i int) Option {
	return func(cfg *Config) error {
		cfg.Concurrency = i
		return nil
	}
}

func WithChannelBufferSize(i int) Option {
	return func(cfg *Config) error {
		cfg.ChannelBufferSize = i
		return nil
	}
}

func WithInterfaceMTU(i int) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.InterfaceMTU = i
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

func WithInterfaceAddress(i string) func(cfg *Config) error {
	return func(cfg *Config) error {
		cfg.InterfaceAddress = i
		return nil
	}
}
