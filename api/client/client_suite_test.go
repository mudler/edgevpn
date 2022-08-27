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

package client_test

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

func TestClient(t *testing.T) {
	if testInstance == "" {
		fmt.Println("a testing instance has to be defined with TEST_INSTANCE")
		os.Exit(1)
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var _ = BeforeSuite(func() {
	// Start the test suite only if we have some machines connected

	Eventually(func() (int, error) {
		c := NewClient(WithHost(testInstance))
		m, err := c.Machines()
		return len(m), err
	}, 100*time.Second, 1*time.Second).Should(BeNumerically(">=", 0))
})
