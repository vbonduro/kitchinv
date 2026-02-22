.PHONY: build test test-cover lint vet staticcheck govulncheck all ci docker-build docker-up docker-pull-model

# ---------------------------------------------------------------------------
# Tool discovery
# Prefer tools on $PATH; fall back to well-known local install paths.
# ---------------------------------------------------------------------------
GO             ?= $(shell which go 2>/dev/null || echo $(HOME)/.local/bin/go/bin/go)
GOLANGCI_LINT  ?= $(shell which golangci-lint 2>/dev/null || echo $(HOME)/.local/bin/golangci-lint)
STATICCHECK    ?= $(shell which staticcheck 2>/dev/null || echo $(shell $(GO) env GOPATH)/bin/staticcheck)
GOVULNCHECK    ?= $(shell which govulncheck 2>/dev/null || echo $(shell $(GO) env GOPATH)/bin/govulncheck)

# ---------------------------------------------------------------------------
# Build & test
# ---------------------------------------------------------------------------
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o kitchinv ./cmd/kitchinv

test:
	$(GO) test -race -count=1 ./...

test-unit:
	$(GO) test -race -count=1 -short ./...

test-integration:
	$(GO) test -race -count=1 ./...

test-cover:
	$(GO) test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# ---------------------------------------------------------------------------
# Static analysis
# ---------------------------------------------------------------------------
lint:
	$(GOLANGCI_LINT) run ./...

vet:
	$(GO) vet ./...

staticcheck:
	$(STATICCHECK) ./...

govulncheck:
	$(GOVULNCHECK) ./...

# ---------------------------------------------------------------------------
# Aggregate targets
# ---------------------------------------------------------------------------
## all: lint + vet + test + build
all: lint vet test build

## ci: run every check that GitHub Actions runs (lint, vet, staticcheck, govulncheck, test)
ci: lint vet staticcheck govulncheck test

# ---------------------------------------------------------------------------
# Docker helpers
# ---------------------------------------------------------------------------
docker-build:
	docker build -t kitchinv:latest .

docker-up:
	docker compose up --build

docker-pull-model:
	docker compose exec ollama ollama pull moondream
