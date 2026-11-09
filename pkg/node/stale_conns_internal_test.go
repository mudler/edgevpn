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

package node

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

type fakeConn struct {
	network.Conn
	stat   network.ConnStats
	remote ma.Multiaddr
}

func (c fakeConn) Stat() network.ConnStats       { return c.stat }
func (c fakeConn) RemoteMultiaddr() ma.Multiaddr { return c.remote }

func TestIsRelayed(t *testing.T) {
	circuit := ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1/p2p/12D3KooWFhw67EU1Th6CSFeQWVRnug3RLVB6LavsNsFrszgkNjzp/p2p-circuit")
	direct := ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1")
	limited := network.ConnStats{Stats: network.Stats{Limited: true}}

	for _, tc := range []struct {
		name string
		conn fakeConn
		want bool
	}{
		{"direct", fakeConn{remote: direct}, false},
		{"limited circuit", fakeConn{stat: limited, remote: circuit}, true},
		{"unlimited circuit", fakeConn{remote: circuit}, true},
		{"limited without circuit address", fakeConn{stat: limited, remote: direct}, true},
	} {
		if got := isRelayed(tc.conn); got != tc.want {
			t.Errorf("%s: isRelayed = %v, want %v", tc.name, got, tc.want)
		}
	}
}
