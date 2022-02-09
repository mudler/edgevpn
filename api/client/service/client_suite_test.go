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
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/mudler/edgevpn/api/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testInstance = os.Getenv("TEST_INSTANCE")

func TestService(t *testing.T) {
	if testInstance == "" {
		fmt.Println("a testing instance has to be defined with TEST_INSTANCE")
		os.Exit(1)
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Suite")
}

var _ = BeforeSuite(func() {
	// Start the test suite only if we have some machines connected

	Eventually(func() (int, error) {
		c := NewClient(WithHost(testInstance))
		m, err := c.Machines()
		return len(m), err
	}, 100*time.Second, 1*time.Second).Should(BeNumerically(">=", 0))
})
