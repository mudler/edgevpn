#!/bin/bash

set -ex

./edgevpn api &

GO111MODULE=off go get github.com/onsi/ginkgo/ginkgo
GO111MODULE=off go get github.com/onsi/gomega/...

TEST_INSTANCE="http://localhost:8080" go test ./api/client