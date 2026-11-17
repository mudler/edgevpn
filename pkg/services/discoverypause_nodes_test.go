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

package services_test

import (
	"context"
	"time"

	"github.com/ipfs/go-log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	. "github.com/mudler/edgevpn/pkg/services"
)

var _ = Describe("Discovery pause", func() {
	token := node.GenerateNewConnectionData().Base64()
	logg := logger.New(log.LevelError)

	opts := append(
		Alive(5*time.Second, 100*time.Second, 15*time.Minute),
		node.WithDiscoveryInterval(10*time.Second),
		node.FromBase64(true, true, token, nil, nil),
		node.Logger(logg))

	It("pauses while connected to the only other node, and resumes once it leaves", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		otherCtx, stopOther := context.WithCancel(ctx)
		defer stopOther()

		p := NewDiscoveryPause(logg, 5*time.Second)
		e1, err := node.New(append(opts, node.WithStore(&blockchain.MemoryStore{}), node.WithNetworkService(p.NetworkService()))...)
		Expect(err).ToNot(HaveOccurred())
		e2, err := node.New(append(opts, node.WithStore(&blockchain.MemoryStore{}))...)
		Expect(err).ToNot(HaveOccurred())

		Expect(p.Pause()).To(BeFalse(), "paused before the node started")
		Expect(e1.Start(ctx)).To(Succeed())
		Expect(e2.Start(otherCtx)).To(Succeed())

		Eventually(p.Pause, 100*time.Second, time.Second).Should(BeTrue())

		stopOther()
		Expect(e2.Host().Close()).To(Succeed())
		Eventually(p.Pause, 30*time.Second, time.Second).Should(BeFalse())
	})
})
