#!/bin/bash

set -ex

GO111MODULE=off go get github.com/onsi/ginkgo/ginkgo
GO111MODULE=off go get github.com/onsi/gomega/...


./edgevpn api &

export TEST_INSTANCE="http://localhost:8080"

go test -coverprofile=coverage.txt -covermode=atomic -race ./pkg/... ./api/client