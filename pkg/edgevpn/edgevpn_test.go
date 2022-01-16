// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package edgevpn_test

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	. "github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/mudler/edgevpn/pkg/logger"
)

var _ = Describe("EdgeVPN", func() {
	token := GenerateNewConnectionData().Base64()

	l := Logger(logger.New(log.LevelFatal))

	e := New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)
	e2 := New(FromBase64(true, true, token), WithStore(&blockchain.MemoryStore{}), l)

	Context("Connection", func() {
		It("see each other node ID", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e.Join(ctx)
			e2.Join(ctx)

			Eventually(func() []peer.ID {
				return e.Host().Network().Peers()
			}, 100*time.Second, 1*time.Second).Should(ContainElement(e2.Host().ID()))
		})
	})
})
