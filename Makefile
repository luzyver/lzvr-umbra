# Common commands. Run `make help` for the list of targets.
GOOS ?= linux
GOARCH ?= amd64
BIN := deploy/umbra

.PHONY: help web build dev check test vet clean

help: ## show the list of targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-8s %s\n", $$1, $$2}'

web: ## build the frontend into internal/ui/dist
	cd web && npm ci --no-audit --no-fund && npm run build

build: web ## build the Linux binary (GOARCH=arm64 for ARM VPSes)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/umbra

dev: ## run the backend with fake WireGuard & firewall
	UMBRA_DEV=1 UMBRA_FAKE_WG=1 UMBRA_FAKE_FIREWALL=1 go run ./cmd/umbra

check: ## type-check the frontend
	cd web && npm run check

test: ## run all tests
	go test ./...
	cd web && npm test

vet: ## go vet
	go vet ./...

clean: ## remove build output
	rm -f $(BIN)
	find internal/ui/dist -mindepth 1 ! -name .keep -delete
