package utils

import (
	"time"

	backoff "github.com/cenkalti/backoff/v4"
)

type expBackoffOpt func(e *backoff.ExponentialBackOff)

func BackoffInitialInterval(i time.Duration) expBackoffOpt {
	return func(e *backoff.ExponentialBackOff) {
		e.InitialInterval = i
	}
}
func BackoffRandomizationFactor(i float64) expBackoffOpt {
	return func(e *backoff.ExponentialBackOff) {
		e.RandomizationFactor = i
	}
}
func BackoffMultiplier(i float64) expBackoffOpt {
	return func(e *backoff.ExponentialBackOff) {
		e.Multiplier = i
	}
}

func BackoffMaxInterval(i time.Duration) expBackoffOpt {
	return func(e *backoff.ExponentialBackOff) {
		e.MaxInterval = i
	}
}

func BackoffMaxElapsedTime(i time.Duration) expBackoffOpt {
	return func(e *backoff.ExponentialBackOff) {
		e.MaxElapsedTime = i
	}
}

// defaultRandomizationFactor is the jitter applied to every backoff interval:
// each tick fires somewhere in [interval*(1-f), interval*(1+f)].
const defaultRandomizationFactor = 0.5

// MaxIntervalJitterFactor is the largest multiple of a ticker's configured max
// interval that a single tick can take. Anything timing out on a periodic
// announcement driven by NewBackoffTicker has to allow for it: a ticker with a
// 120s max interval can legitimately stay silent for 180s.
const MaxIntervalJitterFactor = 1 + defaultRandomizationFactor

func newExpBackoff(o ...expBackoffOpt) backoff.BackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     5 * time.Second,
		RandomizationFactor: defaultRandomizationFactor,
		Multiplier:          2,
		MaxInterval:         2 * time.Minute,
		MaxElapsedTime:      0,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	for _, opt := range o {
		opt(b)
	}
	b.Reset()
	return b
}

func NewBackoffTicker(o ...expBackoffOpt) *backoff.Ticker {
	return backoff.NewTicker(newExpBackoff(o...))
}
