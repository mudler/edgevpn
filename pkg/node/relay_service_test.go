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

package node_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/libp2p/go-libp2p"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/config"
	"github.com/mudler/edgevpn/pkg/node"
)

// TestRelayServiceKnobsApplied builds a libp2p relay using the exact
// option site used inside pkg/config (libp2p.EnableRelayService /
// relayv2.WithResources) and verifies the configured knobs propagate
// all the way into the relayv2.Relay's internal Resources struct.
//
// relayv2.Relay exposes no public getter for its Resources; the
// deliverable expressly permits reflection here.
func TestRelayServiceKnobsApplied(t *testing.T) {
	custom := config.RelayService{
		MaxData:        2 << 30, // 2 GiB
		MaxDuration:    45 * time.Minute,
		MaxCircuits:    128,
		ReservationTTL: 2 * time.Hour,
		BufferSize:     128 << 10,
	}

	res := config.RelayServiceResources(custom)

	// Verify the helper itself produces the right struct.
	if res.MaxCircuits != custom.MaxCircuits {
		t.Errorf("MaxCircuits: got %d, want %d", res.MaxCircuits, custom.MaxCircuits)
	}
	if res.BufferSize != custom.BufferSize {
		t.Errorf("BufferSize: got %d, want %d", res.BufferSize, custom.BufferSize)
	}
	if res.ReservationTTL != custom.ReservationTTL {
		t.Errorf("ReservationTTL: got %s, want %s", res.ReservationTTL, custom.ReservationTTL)
	}
	if res.Limit == nil || res.Limit.Duration != custom.MaxDuration {
		t.Errorf("Limit.Duration: got %v, want %s", res.Limit, custom.MaxDuration)
	}
	if res.Limit == nil || res.Limit.Data != custom.MaxData {
		t.Errorf("Limit.Data: got %v, want %d", res.Limit, custom.MaxData)
	}

	// Build a real libp2p host and a relayv2.Relay on top of it using
	// the same WithResources option call that pkg/config emits.
	h, err := libp2p.New(
		libp2p.NoListenAddrs,
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		t.Fatalf("libp2p.New: %v", err)
	}
	defer h.Close()

	relay, err := relayv2.New(h, relayv2.WithResources(res))
	if err != nil {
		t.Fatalf("relayv2.New: %v", err)
	}
	defer relay.Close()

	got := readRelayResources(t, relay)

	if got.MaxCircuits != custom.MaxCircuits {
		t.Errorf("relay rc.MaxCircuits: got %d, want %d", got.MaxCircuits, custom.MaxCircuits)
	}
	if got.BufferSize != custom.BufferSize {
		t.Errorf("relay rc.BufferSize: got %d, want %d", got.BufferSize, custom.BufferSize)
	}
	if got.ReservationTTL != custom.ReservationTTL {
		t.Errorf("relay rc.ReservationTTL: got %s, want %s", got.ReservationTTL, custom.ReservationTTL)
	}
	if got.Limit == nil {
		t.Fatal("relay rc.Limit is nil")
	}
	if got.Limit.Duration != custom.MaxDuration {
		t.Errorf("relay rc.Limit.Duration: got %s, want %s", got.Limit.Duration, custom.MaxDuration)
	}
	if got.Limit.Data != custom.MaxData {
		t.Errorf("relay rc.Limit.Data: got %d, want %d", got.Limit.Data, custom.MaxData)
	}
}

// TestNodeAcceptsCustomRelayKnobs is a plumbing smoke test: pkg/config
// must accept custom relay-service knobs end-to-end and translate them
// into valid pkg/node options that produce a constructible Node.
func TestNodeAcceptsCustomRelayKnobs(t *testing.T) {
	token := node.GenerateNewConnectionData(25).Base64()

	cfg := &config.Config{
		NetworkToken: token,
		Connection: config.Connection{
			AutoRelay: false, // keep the test minimal
			RelayService: config.RelayService{
				MaxData:        4 << 30,
				MaxDuration:    1 * time.Hour,
				MaxCircuits:    256,
				ReservationTTL: 4 * time.Hour,
				BufferSize:     256 << 10,
			},
		},
		Discovery: config.Discovery{DHT: false, MDNS: false},
	}

	nodeOpts, _, err := cfg.ToOpts(nil)
	if err != nil {
		t.Fatalf("ToOpts: %v", err)
	}
	nodeOpts = append(nodeOpts, node.WithStore(&blockchain.MemoryStore{}))

	n, err := node.New(nodeOpts...)
	if err != nil {
		t.Fatalf("node.New: %v", err)
	}
	if n == nil {
		t.Fatal("node.New returned nil node")
	}
}

// readRelayResources reads the unexported `rc` field of relayv2.Relay via
// reflection. There is no public getter; the deliverable expressly allows
// this approach.
func readRelayResources(t *testing.T, r *relayv2.Relay) relayv2.Resources {
	t.Helper()
	v := reflect.ValueOf(r).Elem().FieldByName("rc")
	if !v.IsValid() {
		t.Fatal("relayv2.Relay has no field named rc; libp2p layout changed")
	}
	// Bypass unexported-field access restrictions by reconstructing a
	// settable reflect.Value at the same memory address.
	rc := reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface().(relayv2.Resources)
	return rc
}
