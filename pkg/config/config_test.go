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

package config

import (
	"testing"
	"time"
)

// TestRelayServiceResourcesDefaults verifies that a zero-valued RelayService
// produces relayv2.Resources populated with edgevpn's wider defaults, not
// libp2p's conservative ones.
func TestRelayServiceResourcesDefaults(t *testing.T) {
	res := RelayServiceResources(RelayService{})

	if res.MaxCircuits != DefaultRelayServiceMaxCircuits {
		t.Errorf("MaxCircuits: got %d, want %d", res.MaxCircuits, DefaultRelayServiceMaxCircuits)
	}
	if res.BufferSize != DefaultRelayServiceBufferSize {
		t.Errorf("BufferSize: got %d, want %d", res.BufferSize, DefaultRelayServiceBufferSize)
	}
	if res.ReservationTTL != DefaultRelayServiceReservationTTL {
		t.Errorf("ReservationTTL: got %s, want %s", res.ReservationTTL, DefaultRelayServiceReservationTTL)
	}
	if res.Limit == nil {
		t.Fatal("Limit is nil")
	}
	if res.Limit.Duration != DefaultRelayServiceMaxDuration {
		t.Errorf("Limit.Duration: got %s, want %s", res.Limit.Duration, DefaultRelayServiceMaxDuration)
	}
	if res.Limit.Data != DefaultRelayServiceMaxData {
		t.Errorf("Limit.Data: got %d, want %d", res.Limit.Data, DefaultRelayServiceMaxData)
	}
}

// TestRelayServiceResourcesCustom verifies that explicit knob values override
// the defaults end-to-end into the relayv2.Resources struct.
func TestRelayServiceResourcesCustom(t *testing.T) {
	custom := RelayService{
		MaxData:        2 << 30, // 2 GiB
		MaxDuration:    45 * time.Minute,
		MaxCircuits:    128,
		ReservationTTL: 2 * time.Hour,
		BufferSize:     128 << 10, // 128 KiB
	}
	res := RelayServiceResources(custom)

	if res.MaxCircuits != 128 {
		t.Errorf("MaxCircuits: got %d, want 128", res.MaxCircuits)
	}
	if res.BufferSize != 128<<10 {
		t.Errorf("BufferSize: got %d, want %d", res.BufferSize, 128<<10)
	}
	if res.ReservationTTL != 2*time.Hour {
		t.Errorf("ReservationTTL: got %s, want %s", res.ReservationTTL, 2*time.Hour)
	}
	if res.Limit == nil {
		t.Fatal("Limit is nil")
	}
	if res.Limit.Duration != 45*time.Minute {
		t.Errorf("Limit.Duration: got %s, want %s", res.Limit.Duration, 45*time.Minute)
	}
	if res.Limit.Data != 2<<30 {
		t.Errorf("Limit.Data: got %d, want %d", res.Limit.Data, int64(2<<30))
	}
}

// TestRelayServiceResourcesDoesNotMutateDefaults guards against the
// DefaultLimit() singleton being mutated through res.Limit aliasing.
// Two independent calls with different overrides must not interfere.
func TestRelayServiceResourcesDoesNotMutateDefaults(t *testing.T) {
	a := RelayServiceResources(RelayService{MaxData: 1})
	b := RelayServiceResources(RelayService{MaxData: 2})

	if a.Limit.Data != 1 {
		t.Errorf("a.Limit.Data: got %d, want 1 (mutated by b?)", a.Limit.Data)
	}
	if b.Limit.Data != 2 {
		t.Errorf("b.Limit.Data: got %d, want 2", b.Limit.Data)
	}
	if a.Limit == b.Limit {
		t.Error("a.Limit and b.Limit share the same pointer; defaults are being aliased")
	}
}
