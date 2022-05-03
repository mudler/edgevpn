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

func newExpBackoff(o ...expBackoffOpt) backoff.BackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     5 * time.Second,
		RandomizationFactor: 0.5,
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
