# Perintah umum. Jalankan `make help` untuk daftar target.
GOOS ?= linux
GOARCH ?= amd64
BIN := deploy/umbra

.PHONY: help web build dev check test vet clean

help: ## tampilkan daftar target
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-8s %s\n", $$1, $$2}'

web: ## build frontend ke internal/ui/dist
	cd web && npm ci --no-audit --no-fund && npm run build

build: web ## build binary Linux (GOARCH=arm64 untuk VPS ARM)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/umbra

dev: ## jalankan backend dengan WireGuard & firewall tiruan
	UMBRA_DEV=1 UMBRA_FAKE_WG=1 UMBRA_FAKE_FIREWALL=1 go run ./cmd/umbra

check: ## cek tipe frontend
	cd web && npm run check

test: ## jalankan semua test
	go test ./...
	cd web && npm test

vet: ## go vet
	go vet ./...

clean: ## hapus hasil build
	rm -f $(BIN)
	find internal/ui/dist -mindepth 1 ! -name .keep -delete
