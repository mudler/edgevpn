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
	"testing"

	"github.com/urfave/cli/v2"
)

func proxyStringFlags(t *testing.T) map[string]string {
	t.Helper()

	values := map[string]string{}
	for _, f := range Proxy().Flags {
		if sf, ok := f.(*cli.StringFlag); ok {
			values[sf.Name] = sf.Value
		}
	}
	return values
}

// TestProxyAPIHasItsOwnListenAddress is the regression guard for the defect
// where `edgevpn proxy` handed the very same address to the proxy server and to
// the API server, so one of the two always lost the race to bind it.
func TestProxyAPIHasItsOwnListenAddress(t *testing.T) {
	values := proxyStringFlags(t)

	listen, ok := values["listen"]
	if !ok {
		t.Fatal("the proxy command has no --listen flag")
	}
	apiListen, ok := values["api-listen"]
	if !ok {
		t.Fatal("the proxy command has no --api-listen flag: the API server would bind the proxy's own --listen address")
	}

	if listenersConflict(listen, apiListen) {
		t.Fatalf("the default --listen (%q) and --api-listen (%q) compete for the same port", listen, apiListen)
	}
}

// TestProxyAPIIsOptIn keeps the proxy command consistent with the root command,
// which only starts the API when --api is passed.
func TestProxyAPIIsOptIn(t *testing.T) {
	for _, f := range Proxy().Flags {
		bf, ok := f.(*cli.BoolFlag)
		if !ok || bf.Name != "api" {
			continue
		}
		if bf.Value {
			t.Fatal("--api must default to off, as it does on the root command")
		}
		return
	}
	t.Fatal("the proxy command has no --api flag: it would start an API server unprompted")
}

func TestListenersConflict(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b string
		want bool
	}{
		{"identical wildcard", ":8080", ":8080", true},
		{"identical explicit", "127.0.0.1:8080", "127.0.0.1:8080", true},
		{"wildcard covers loopback", ":8080", "127.0.0.1:8080", true},
		{"loopback covered by wildcard", "127.0.0.1:8080", "0.0.0.0:8080", true},
		{"ipv6 wildcard covers loopback", "[::]:8080", "127.0.0.1:8080", true},
		{"different ports", ":8080", "127.0.0.1:8081", false},
		{"different ports, both wildcard", ":8080", ":8081", false},
		{"different hosts, same port", "127.0.0.1:8080", "10.1.0.1:8080", false},
		// Hosts have to be resolved, not string-matched: these name the
		// same socket by two different spellings.
		{"hostname alias for loopback", "localhost:8080", "127.0.0.1:8080", true},
		{"hostname alias, wildcard other side", "localhost:8080", ":8080", true},
		{"hostname alias on a different port", "localhost:8080", "127.0.0.1:8081", false},
		// Ports likewise: /etc/services names the same port as a number.
		{"named port against its number", ":http-alt", ":8080", true},
		{"named port against another port", ":http-alt", ":8081", false},
		{"named port and hostname alias", "localhost:http-alt", "127.0.0.1:8080", true},
		{"unix socket never clashes with tcp", ":8080", "unix:///run/edgevpn.sock", false},
		{"same unix socket clashes", "unix:///run/edgevpn.sock", "unix:///run/edgevpn.sock", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := listenersConflict(tc.a, tc.b); got != tc.want {
				t.Errorf("listenersConflict(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			// The relation is symmetric.
			if got := listenersConflict(tc.b, tc.a); got != tc.want {
				t.Errorf("listenersConflict(%q, %q) = %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}
}
