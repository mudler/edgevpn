.PHONY: all build react-ui react-ui-force test clean

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

clean:
	rm -rf api/react-ui/dist edgevpn
