#!/bin/bash

set -ex

go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo

./edgevpn api &

export TEST_INSTANCE="http://localhost:8080"

ginkgo -v -r --flake-attempts 5 --coverprofile=coverage.txt --covermode=atomic --race ./pkg/... ./api/...