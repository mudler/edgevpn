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

package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/pkg/utils"
)

var _ = Describe("IP", func() {
	Context("NextIP", func() {
		It("gives a new IP", func() {
			Expect(NextIP("10.1.1.0", []string{"1.1.0.1"})).To(Equal("1.1.0.2"))
		})
		It("return default", func() {
			Expect(NextIP("10.1.1.0", []string{})).To(Equal("10.1.1.0"))
		})
	})
})
