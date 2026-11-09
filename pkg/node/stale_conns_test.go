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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/mudler/edgevpn/pkg/node"
)

// testGrace is short so the tests do not wait for the default grace; each
// test lets the Wi-Fi connection age past it before B switches networks.
const testGrace = 100 * time.Millisecond

func TestStaleDirectConnReplacedAfterHandover(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))
	time.Sleep(2 * testGrace)

	n.switchB(t, ctx)

	took := waitFor(t, 5*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	t.Logf("direct connection on the mobile address after %s", took)
}

// Neither B's mobile address nor A's address accepts incoming connections,
// so the direct connection can only come back through libp2p's hole punch.
func TestStaleDirectConnReplacedByHolePunch(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, false)
	watch(t, n.a, node.WithStaleConnsGrace(testGrace))
	time.Sleep(2 * testGrace)

	n.switchB(t, ctx)

	took := waitFor(t, 10*time.Second, func() bool { return n.aHasDirectConnOn(simBMobileAddr) }, func() string {
		return fmt.Sprintf("A has no direct connection on B's mobile address; A conns: %v", simConns(n.a, n.b.ID()))
	})
	waitFor(t, time.Second, n.aHolePunch.succeeded, func() string {
		return fmt.Sprintf("the direct connection came back without a successful hole punch; A events: %s", n.aHolePunch)
	})
	t.Logf("hole-punched connection on the mobile address after %s", took)
}

func TestDirectConnYoungerThanGraceIsKept(t *testing.T) {
	if testing.Short() {
		t.Skip("simulated network handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	n := newSimHandover(t, ctx, true)
	watch(t, n.a, node.WithStaleConnsGrace(time.Hour))

	n.switchB(t, ctx)
	time.Sleep(time.Second)

	if !n.aHasDirectConnOn(simBWifiAddr) {
		t.Fatalf("A closed the Wi-Fi connection that is younger than grace; A conns: %v", simConns(n.a, n.b.ID()))
	}
}

func watch(t *testing.T, h host.Host, opts ...node.StaleConnsOption) {
	t.Helper()
	w := node.WatchStaleDirectConns(h, opts...)
	t.Cleanup(func() { w.Close() })
}
