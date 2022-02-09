// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
		It("Detect nodes", func() {
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
