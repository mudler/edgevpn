package client_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClient(t *testing.T) {
	if testInstance == "" {
		fmt.Println("a testing instance has to be defined with TEST_INSTANCE")
		os.Exit(1)
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}
