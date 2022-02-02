// The MIT License (MIT)

// Copyright (c) 2017 Whyrusleeping

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// This package is a port of go-libp2p-connmgr, but adapted for streams

package stream

import (
	"errors"
	"time"
)

// config is the configuration struct for the basic connection manager.
type config struct {
	highWater     int
	lowWater      int
	gracePeriod   time.Duration
	silencePeriod time.Duration
	decayer       *DecayerCfg
	emergencyTrim bool
}

// Option represents an option for the basic connection manager.
type Option func(*config) error

// DecayerConfig applies a configuration for the decayer.
func DecayerConfig(opts *DecayerCfg) Option {
	return func(cfg *config) error {
		cfg.decayer = opts
		return nil
	}
}

// WithGracePeriod sets the grace period.
// The grace period is the time a newly opened connection is given before it becomes
// subject to pruning.
func WithGracePeriod(p time.Duration) Option {
	return func(cfg *config) error {
		if p < 0 {
			return errors.New("grace period must be non-negative")
		}
		cfg.gracePeriod = p
		return nil
	}
}

// WithSilencePeriod sets the silence period.
// The connection manager will perform a cleanup once per silence period
// if the number of connections surpasses the high watermark.
func WithSilencePeriod(p time.Duration) Option {
	return func(cfg *config) error {
		if p <= 0 {
			return errors.New("silence period must be non-zero")
		}
		cfg.silencePeriod = p
		return nil
	}
}
