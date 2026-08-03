.PHONY: build test docs-gen

build:
	go build -o edgevpn ./

test:
	go test ./...

# docs-gen regenerates the CLI and environment-variable reference from the real
# cli.App. The output is committed; CI re-runs this and fails on a diff, so the
# docs cannot drift from the binary.
docs-gen:
	go run ./internal/docsgen
