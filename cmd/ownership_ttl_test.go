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
	"time"

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/utils"
	"github.com/urfave/cli/v2"
)

// flagIntDefault returns the declared default of an integer CommonFlag.
func flagIntDefault(t *testing.T, name string) int {
	t.Helper()
	for _, f := range CommonFlags {
		if i, ok := f.(*cli.IntFlag); ok && i.Name == name {
			return i.Value
		}
	}
	t.Fatalf("flag %q not found in CommonFlags", name)
	return 0
}

// worstCaseHeartbeat is the longest a healthy node can legitimately go between
// heartbeats: the alive service drives them from utils.NewBackoffTicker, whose
// steady-state interval is jittered up to MaxIntervalJitterFactor of the
// configured value.
func worstCaseHeartbeat(interval time.Duration) time.Duration {
	return time.Duration(float64(interval) * utils.MaxIntervalJitterFactor)
}

// The CLI flag default and the constant the rest of the code reasons about must
// not drift apart, or the invariant below would be checked against a number
// nobody actually runs with.
func TestHealthcheckIntervalFlagMatchesDefault(t *testing.T) {
	got := time.Duration(flagIntDefault(t, "aliveness-healthcheck-interval")) * time.Second
	if got != services.DefaultHealthcheckInterval {
		t.Errorf("--aliveness-healthcheck-interval default = %s, want %s (services.DefaultHealthcheckInterval)",
			got, services.DefaultHealthcheckInterval)
	}
}

// The invariant. A node is declared inactive purely on the age of its last
// heartbeat, so the liveness window has to exceed the worst-case gap between
// two heartbeats — with enough margin to survive one lost heartbeat, since a
// single dropped gossip message must not evict a live peer.
//
// This is what the old default got wrong: a 2m window against a 120s heartbeat
// that jitters to 180s expired healthy nodes routinely.
func TestDefaultOwnershipTTLSurvivesHeartbeatJitter(t *testing.T) {
	worst := worstCaseHeartbeat(services.DefaultHealthcheckInterval)

	if node.DefaultOwnershipTTL <= worst {
		t.Fatalf("DefaultOwnershipTTL = %s, but a healthy node can go %s between heartbeats: live nodes will be falsely expired",
			node.DefaultOwnershipTTL, worst)
	}
	// Strictly greater, not >=. IsLive tests parsed.Add(ttl).After(now), so a
	// node whose last heartbeat is exactly ttl old is already expired: equality
	// here would mean the invariant is satisfied at precisely the point it
	// fails.
	if node.DefaultOwnershipTTL <= 2*worst {
		t.Errorf("DefaultOwnershipTTL = %s, want more than %s (two worst-case heartbeat intervals, so one lost heartbeat is survivable with margin)",
			node.DefaultOwnershipTTL, 2*worst)
	}
}

// The same invariant has to hold when an operator tunes the heartbeat, which is
// why the TTL is derived from it rather than being a fixed number.
func TestOwnershipTTLForSurvivesHeartbeatJitter(t *testing.T) {
	for _, hb := range []time.Duration{
		10 * time.Second,
		30 * time.Second,
		services.DefaultHealthcheckInterval,
		5 * time.Minute,
		30 * time.Minute,
	} {
		ttl := node.OwnershipTTLFor(hb)
		worst := worstCaseHeartbeat(hb)
		if ttl <= 2*worst {
			t.Errorf("OwnershipTTLFor(%s) = %s, want more than %s (two worst-case intervals)", hb, ttl, 2*worst)
		}
	}
}

// On stock settings the derived value and the library fallback must agree, so
// a binary node and an embedded node behave the same.
func TestOwnershipTTLForDefaultMatchesConstant(t *testing.T) {
	if got := node.OwnershipTTLFor(services.DefaultHealthcheckInterval); got != node.DefaultOwnershipTTL {
		t.Errorf("OwnershipTTLFor(default heartbeat) = %s, want %s", got, node.DefaultOwnershipTTL)
	}
}

// --ownership-ttl 0 means "derive it"; a value the operator sets explicitly is
// used as-is.
func TestConfigFromContextDerivesOwnershipTTL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ttlFlag int
		hbFlag  int
		wantTTL time.Duration
	}{
		{"zero derives from the heartbeat", 0, 120, 8 * time.Minute},
		{"zero derives from a tuned heartbeat", 0, 300, 20 * time.Minute},
		{"explicit value is respected", 90, 120, 90 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveOwnershipTTL(
				time.Duration(tc.ttlFlag)*time.Second,
				time.Duration(tc.hbFlag)*time.Second,
			)
			if got != tc.wantTTL {
				t.Errorf("resolveOwnershipTTL = %s, want %s", got, tc.wantTTL)
			}
		})
	}
}
