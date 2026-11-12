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
	"fmt"
	"strings"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
)

// holePunchLogger traces libp2p's hole punching service (DCUtR, direct
// connection upgrade through a relay) at debug level. libp2p's own hole
// punching log does not reach the node's log.
type holePunchLogger struct{ l log.StandardLogger }

func (h holePunchLogger) Trace(e *holepunch.Event) {
	h.l.Debug(describeHolePunch(e))
}

// describeHolePunch renders one hole punching event as a log line. Before
// punching, libp2p tries a plain direct dial ("direct dial"); a punch then
// has a start, one or more attempts and an end.
func describeHolePunch(e *holepunch.Event) string {
	prefix := "hole punch with " + e.Remote.String() + ": "
	switch evt := e.Evt.(type) {
	case *holepunch.DirectDialEvt:
		return prefix + outcome("direct dial ", evt.Success, evt.EllapsedTime.String(), evt.Error)
	case *holepunch.StartHolePunchEvt:
		return prefix + fmt.Sprintf("started, round trip %s, remote addresses [%s]", evt.RTT, strings.Join(evt.RemoteAddrs, " "))
	case *holepunch.HolePunchAttemptEvt:
		return prefix + fmt.Sprintf("attempt %d", evt.Attempt)
	case *holepunch.EndHolePunchEvt:
		return prefix + outcome("", evt.Success, evt.EllapsedTime.String(), evt.Error)
	case *holepunch.ProtocolErrorEvt:
		return prefix + "protocol error: " + evt.Error
	}
	return prefix + e.Type
}

func outcome(what string, success bool, took, err string) string {
	if success {
		return what + "succeeded in " + took
	}
	return what + "failed after " + took + ": " + err
}
