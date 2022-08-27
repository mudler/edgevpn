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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
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
