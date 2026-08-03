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

package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/mudler/edgevpn/api"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/urfave/cli/v2"
)

// isWildcardHost reports whether a host part of a listen address covers every
// local address, and therefore every other host on the same port.
func isWildcardHost(host string) bool {
	switch host {
	case "", "0.0.0.0", "::":
		return true
	}
	return false
}

// resolvedListenAddr is a listen address reduced to what actually decides
// whether two servers can bind at the same time.
type resolvedListenAddr struct {
	isUnix   bool
	unixPath string

	port     int
	wildcard bool
	ips      []net.IP
}

// resolveListenAddr resolves an address the way the kernel will see it, so that
// two different spellings of the same socket - "localhost" and "127.0.0.1", or
// a service name and its port number - do not look distinct.
func resolveListenAddr(addr string) (resolvedListenAddr, error) {
	if strings.HasPrefix(addr, api.UnixSocketScheme) {
		return resolvedListenAddr{
			isUnix:   true,
			unixPath: strings.TrimPrefix(addr, api.UnixSocketScheme),
		}, nil
	}

	host, portName, err := net.SplitHostPort(addr)
	if err != nil {
		return resolvedListenAddr{}, err
	}

	// Resolves both "8080" and service names such as "http-alt".
	port, err := net.LookupPort("tcp", portName)
	if err != nil {
		return resolvedListenAddr{}, err
	}

	if isWildcardHost(host) {
		return resolvedListenAddr{port: port, wildcard: true}, nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return resolvedListenAddr{}, err
	}
	return resolvedListenAddr{port: port, ips: ips}, nil
}

// listenersConflict reports whether two listen addresses would compete to bind
// the same socket. A unix socket never competes with a TCP listener, and a
// wildcard host covers every address on its port.
func listenersConflict(a, b string) bool {
	resolvedA, errA := resolveListenAddr(a)
	resolvedB, errB := resolveListenAddr(b)
	if errA != nil || errB != nil {
		// Something we cannot resolve will fail at bind time anyway; until
		// then, only claim a conflict for an exact textual match.
		return a == b
	}

	if resolvedA.isUnix || resolvedB.isUnix {
		return resolvedA.isUnix && resolvedB.isUnix && resolvedA.unixPath == resolvedB.unixPath
	}

	if resolvedA.port != resolvedB.port {
		return false
	}
	if resolvedA.wildcard || resolvedB.wildcard {
		return true
	}

	for _, ipA := range resolvedA.ips {
		for _, ipB := range resolvedB.ips {
			if ipA.Equal(ipB) {
				return true
			}
		}
	}
	return false
}

func Proxy() *cli.Command {
	return &cli.Command{
		Name:        "proxy",
		Usage:       "Starts a local http proxy server to egress nodes",
		Description: `Start a proxy locally, providing an ingress point for the network.`,
		UsageText:   "edgevpn proxy",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:    "listen",
				Value:   ":8080",
				Usage:   "Listening address",
				EnvVars: []string{"PROXYLISTEN"},
			},
			&cli.BoolFlag{
				Name:    "api",
				Usage:   "Starts also the API daemon locally for inspecting the network status",
				EnvVars: []string{"API"},
			},
			&cli.StringFlag{
				Name:  "api-listen",
				Value: "127.0.0.1:8081",
				Usage: "API listen address, used only with --api. Must differ from --listen. Accepts a TCP host:port or a unix socket path with the 'unix://' prefix (e.g. unix:///run/edgevpn.sock). Socket mode defaults to 0660 and can be overridden via APILISTENUNIXMODE.",
				// Deliberately not 127.0.0.1:8080 like the root command:
				// that would collide with this command's own --listen default.
				EnvVars: []string{"APILISTEN"},
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Starts the API with pprof attached",
			},
			&cli.IntFlag{
				Name:    "interval",
				Usage:   "proxy announce time interval",
				EnvVars: []string{"PROXYINTERVAL"},
				Value:   120,
			},
			&cli.IntFlag{
				Name:    "dead-interval",
				Usage:   "interval (in seconds) wether detect egress nodes offline",
				EnvVars: []string{"PROXYDEADINTERVAL"},
				Value:   600,
			},
		),
		Action: func(c *cli.Context) error {
			listen := c.String("listen")
			apiListen := c.String("api-listen")

			// The proxy and the API are two separate servers: handing them
			// the same address means one of them silently loses the race to
			// bind it.
			if c.Bool("api") && listenersConflict(listen, apiListen) {
				return fmt.Errorf(
					"--listen (%q) and --api-listen (%q) would bind the same address: give the API a different one",
					listen, apiListen)
			}

			o, _, ll := cliToOpts(c)

			o = append(o, services.Proxy(
				time.Duration(c.Int("interval"))*time.Second,
				time.Duration(c.Int("dead-interval"))*time.Second,
				listen)...)

			bwc := metrics.NewBandwidthCounter()
			if c.Bool("api") {
				o = append(o, node.WithLibp2pAdditionalOptions(libp2p.BandwidthReporter(bwc)))
			}

			e, err := node.New(o...)
			if err != nil {
				return err
			}

			displayStart(ll)

			// Unlike the root command, nothing here blocks for the lifetime
			// of the process: the proxy serves in the background and there is
			// no VPN interface to read from. So the signal has to cancel a
			// real context rather than call os.Exit from a side goroutine -
			// api.API shuts its server down when this is cancelled.
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// Once we are shutting down, restore the default disposition so a
			// second signal still kills the process if shutdown stalls.
			go func() {
				<-ctx.Done()
				stop()
			}()

			// Start the node to the network, using our ledger. A failure to
			// bind the proxy's own listener surfaces here.
			if err := e.Start(ctx); err != nil {
				return err
			}

			if c.Bool("api") {
				return api.API(ctx, apiListen, 5*time.Second, 20*time.Second, e, bwc, c.Bool("debug"))
			}

			<-ctx.Done()
			return nil
		},
	}
}
