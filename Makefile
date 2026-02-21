.PHONY: build test test-cover lint vet staticcheck all ci tools docker-build docker-up docker-pull-model

# ---------------------------------------------------------------------------
# Tool discovery
# ---------------------------------------------------------------------------
# Prefer the system go; fall back to the well-known local install path.
GO ?= $(shell which go 2>/dev/null || echo $(HOME)/.local/bin/go/bin/go)

# Per-project tool cache so we never pollute the system PATH.
TOOLS_DIR := .tools
ACT := $(TOOLS_DIR)/act

# ---------------------------------------------------------------------------
# Tool installation
# ---------------------------------------------------------------------------
$(TOOLS_DIR):
	mkdir -p $(TOOLS_DIR)

$(ACT): | $(TOOLS_DIR)
	@echo "Installing act → $(ACT)"
	curl -sSfL https://raw.githubusercontent.com/nektos/act/master/install.sh | bash -s -- -b $(TOOLS_DIR)

## tools: install act into .tools/
tools: $(ACT)

# ---------------------------------------------------------------------------
# Build & test
# ---------------------------------------------------------------------------
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o kitchinv ./cmd/kitchinv

test:
	$(GO) test -race -count=1 ./...

test-cover:
	$(GO) test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# ---------------------------------------------------------------------------
# Static analysis (local, using system tools if available)
# ---------------------------------------------------------------------------
lint:
	golangci-lint run ./...

vet:
	$(GO) vet ./...

staticcheck:
	staticcheck ./...

govulncheck:
	govulncheck ./...

# ---------------------------------------------------------------------------
# Aggregate targets
# ---------------------------------------------------------------------------
## all: lint + vet + test + build
all: lint vet test build

## ci: run the full GitHub Actions CI workflow locally via act
ci: $(ACT)
	$(ACT) --workflows .github/workflows/ci.yml \
		-P ubuntu-latest=catthehacker/ubuntu:act-latest

# ---------------------------------------------------------------------------
# Docker helpers
# ---------------------------------------------------------------------------
docker-build:
	docker build -t kitchinv:latest .

docker-up:
	docker compose up --build

docker-pull-model:
	docker compose exec ollama ollama pull moondream
