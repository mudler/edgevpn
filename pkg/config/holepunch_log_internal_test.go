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

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/test"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
)

func TestDescribeHolePunch(t *testing.T) {
	remote := test.RandPeerIDFatal(t)
	prefix := "hole punch with " + remote.String() + ": "
	for _, tc := range []struct {
		evt  interface{}
		want string
	}{
		{&holepunch.DirectDialEvt{Success: false, EllapsedTime: time.Second, Error: "timeout"}, "direct dial failed after 1s: timeout"},
		{&holepunch.StartHolePunchEvt{RemoteAddrs: []string{"/ip6/::1/udp/1/quic-v1", "/ip4/192.0.2.4/tcp/2"}, RTT: 150 * time.Millisecond}, "started, round trip 150ms, remote addresses [/ip6/::1/udp/1/quic-v1 /ip4/192.0.2.4/tcp/2]"},
		{&holepunch.HolePunchAttemptEvt{Attempt: 2}, "attempt 2"},
		{&holepunch.EndHolePunchEvt{Success: true, EllapsedTime: 300 * time.Millisecond}, "succeeded in 300ms"},
		{&holepunch.ProtocolErrorEvt{Error: "bad message"}, "protocol error: bad message"},
	} {
		got := describeHolePunch(&holepunch.Event{Remote: remote, Evt: tc.evt})
		if got != prefix+tc.want {
			t.Errorf("got %q, want %q", got, prefix+tc.want)
		}
	}
}

// levelRecorder records which level each line was logged at. Any method it
// does not override panics.
type levelRecorder struct {
	log.StandardLogger
	levels []string
}

func (r *levelRecorder) Debug(...interface{}) { r.levels = append(r.levels, "debug") }
func (r *levelRecorder) Info(...interface{})  { r.levels = append(r.levels, "info") }

// Hole punch events come with every new connection through a relay, so they
// stay out of the log at the default level.
func TestHolePunchEventsAreLoggedAtDebugLevel(t *testing.T) {
	r := &levelRecorder{}
	holePunchLogger{l: r}.Trace(&holepunch.Event{Remote: test.RandPeerIDFatal(t), Evt: &holepunch.HolePunchAttemptEvt{Attempt: 1}})
	if len(r.levels) != 1 || r.levels[0] != "debug" {
		t.Fatalf("logged at %v, want one line at debug", r.levels)
	}
}
