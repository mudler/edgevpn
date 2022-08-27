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

package service_test

import (
	"time"

	client "github.com/mudler/edgevpn/api/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/api/client/service"
)

var _ = Describe("Service", func() {
	c := client.NewClient(client.WithHost(testInstance))
	s := NewClient("foo", c)
	Context("Retrieves nodes", func() {
		PIt("Detect nodes", func() {
			Eventually(func() []string {
				n, _ := s.ActiveNodes()
				return n
			},
				100*time.Second, 1*time.Second).ShouldNot(BeEmpty())
		})
	})

	Context("Advertize nodes", func() {
		It("Detect nodes", func() {
			n, err := s.AdvertizingNodes()
			Expect(len(n)).To(Equal(0))
			Expect(err).ToNot(HaveOccurred())

			s.Advertize("foo")

			Eventually(func() []string {
				n, _ := s.AdvertizingNodes()
				return n
			},
				100*time.Second, 1*time.Second).Should(Equal([]string{"foo"}))
		})
	})
})
