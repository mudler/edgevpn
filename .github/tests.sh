#!/bin/bash

set -ex

go get github.com/onsi/ginkgo/v2
go get github.com/onsi/gomega/...

go mod tidy

go get github.com/onsi/ginkgo/v2/ginkgo/internal@v2.1.1

go install github.com/onsi/ginkgo/v2/ginkgo

./edgevpn api &

export TEST_INSTANCE="http://localhost:8080"

ginkgo -v -r --flake-attempts 5 --coverprofile=coverage.txt --covermode=atomic --race ./pkg/... ./api/...