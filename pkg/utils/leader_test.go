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

var _ = Describe("Leader utilities", func() {
	Context("Leader", func() {
		It("returns the correct leader", func() {
			Expect(Leader([]string{"a", "b", "c", "d"})).To(Equal("b"))
			Expect(Leader([]string{"a", "b", "c", "d", "e", "f", "G", "bb"})).To(Equal("b"))
			Expect(Leader([]string{"a", "b", "c", "d", "e", "f", "G", "bb", "z", "b1", "b2"})).To(Equal("z"))
			Expect(Leader([]string{"1", "2", "3", "4", "5"})).To(Equal("2"))
			Expect(Leader([]string{"1", "2", "3", "4", "5", "6", "7", "21", "22"})).To(Equal("22"))
		})
	})
})
