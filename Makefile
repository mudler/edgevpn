.PHONY: all build react-ui react-ui-force test clean docs-gen

all: build

# Skip the npm build when dist already exists, so repeated `make build`
# is fast. This intentionally reuses a stale dist — use react-ui-force
# in CI and releases where correctness matters more than speed.
react-ui:
ifneq ($(wildcard api/react-ui/dist),)
	@echo "api/react-ui/dist already exists, skipping build"
else
	cd api/react-ui && npm ci && npm run build
endif

# Always rebuild from source. Used by CI, goreleaser and Docker.
react-ui-force:
	rm -rf api/react-ui/dist
	cd api/react-ui && npm ci && npm run build

api/react-ui/dist: react-ui

build: api/react-ui/dist
	go build -o edgevpn

test: api/react-ui/dist
	go test ./...

# docs-gen regenerates the CLI and environment-variable reference from the real
# cli.App. The output is committed; CI re-runs this and fails on a diff, so the
# docs cannot drift from the binary.
docs-gen:
	go run ./internal/docsgen

clean:
	rm -rf api/react-ui/dist edgevpn
