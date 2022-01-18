#!/bin/bash

set -ex

GO111MODULE=off go get github.com/onsi/ginkgo/ginkgo
GO111MODULE=off go get github.com/onsi/gomega/...

go test -race ./pkg/...

./edgevpn api &

TEST_INSTANCE="http://localhost:8080" go test ./api/client