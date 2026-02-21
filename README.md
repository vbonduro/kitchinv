# kitchinv — Home Kitchen Inventory

> Never wonder what's in the freezer again.

[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen.svg?style=flat)](https://goreportcard.com/report/github.com/vbonduro/kitchinv)
[![codecov](https://codecov.io/gh/vbonduro/kitchinv/branch/main/graph/badge.svg)](https://codecov.io/gh/vbonduro/kitchinv)

kitchinv lets you photograph every storage area in your home (fridge, freezer, pantry, …), automatically extracts a food-item inventory from each photo using a vision model, and gives you a searchable list across all areas — all from your phone browser.

---

## Contents

- [How it works](#how-it-works)
- [Quick start (Docker)](#quick-start-docker)
- [Switching to Claude](#switching-to-claude)
- [Local development](#local-development)
- [Configuration](#configuration)

---

## How it works

1. **Create an area** — give each physical storage location a name ("Upstairs Fridge", "Garage Freezer", "Pantry").
2. **Upload a photo** — tap the camera button from your phone; the rear camera opens directly.
3. **Vision analysis** — the photo is sent to a vision model (Ollama by default; Claude as an alternative). The model returns a line-per-item list in `name | quantity | notes` format.
4. **Browse & search** — the extracted inventory is stored in SQLite. Search across every area instantly.
5. **Re-upload anytime** — uploading a new photo for an area replaces the existing inventory for that area.

---

## Quick start (Docker)

**Requirements:** Docker and Docker Compose.

```bash
# 1. Clone
git clone https://github.com/vbonduro/kitchinv
cd kitchinv

# 2. Start the stack (app + Ollama sidecar)
make docker-up

# 3. Pull the vision model (one-time, ~1 GB)
make docker-pull-model

# 4. Open in browser
open http://localhost:8080
```

The first `docker-up` builds the image from source. Subsequent starts reuse the cached layers.

Data is persisted in two named Docker volumes:

| Volume | Contents |
|--------|----------|
| `kitchinv_kitchinv_data` | SQLite database + uploaded photos |
| `kitchinv_ollama_data` | Downloaded Ollama model weights |

> Docker prefixes volume names with the project name (`kitchinv_`), so the names above reflect what you'll see in `docker volume ls`.

To wipe everything: `docker compose down -v`.

---

## Switching to Claude

Remove the `ollama` service from the stack and set two environment variables:

```bash
# docker-compose.override.yml  (or export before running)
VISION_BACKEND=claude
CLAUDE_API_KEY=sk-ant-...
```

Or run without Compose:

```bash
docker run \
  -e VISION_BACKEND=claude \
  -e CLAUDE_API_KEY=sk-ant-... \
  -e DB_PATH=/data/kitchinv.db \
  -e PHOTO_LOCAL_PATH=/data/photos \
  -v kitchinv_data:/data \
  -p 8080:8080 \
  kitchinv:latest
```

The Claude model defaults to `claude-opus-4-6`. Override with `CLAUDE_MODEL=claude-haiku-4-5-20251001` for a cheaper option.

---

## Local development

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev/dl](https://go.dev/dl/) — extract to `~/.local/bin/go` or anywhere on `$PATH` |
| golangci-lint | v2.10.1 | See below |
| staticcheck | latest | `go install honnef.co/go/tools/cmd/staticcheck@latest` |
| govulncheck | latest | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| direnv | latest | `curl -sfL https://direnv.net/install.sh \| bin_path=~/.local/bin bash` |
| gitleaks | latest | `curl -sfL https://github.com/gitleaks/gitleaks/releases/latest/download/gitleaks_$(uname -s \| tr '[:upper:]' '[:lower:]')_x64.tar.gz \| tar -xz -C ~/.local/bin gitleaks` |

Install golangci-lint (the official way — do **not** `go install` it):
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b ~/.local/bin v2.10.1
```

If Go is not on your `$PATH`, the Makefile will fall back to `~/.local/bin/go/bin/go` automatically.

### Environment setup

The project uses **direnv** to manage environment variables automatically and **gitleaks** to prevent accidentally committing secrets.

**1. Hook direnv into your shell** (once, in `~/.zshrc` or `~/.bashrc`):
```bash
eval "$(~/.local/bin/direnv hook zsh)"   # or: hook bash
```
Then restart your shell or `source ~/.zshrc`.

**2. Create your `.envrc`** from the provided example:
```bash
cp .envrc.example .envrc
direnv allow
```
`.envrc` is gitignored — edit it with your local values. It is loaded automatically whenever you `cd` into the repo or any worktree.

**3. Store secrets in the system keyring** (recommended over plaintext):
```bash
# Store your Anthropic API key once:
secret-tool store --label="kitchinv Claude API key" service kitchinv key CLAUDE_API_KEY
```
The `.envrc.example` shows how to reference it via `$(secret-tool lookup ...)`.

**4. Secret scanning pre-commit hook** is installed at `.git/hooks/pre-commit` and runs gitleaks on every `git commit`. If a secret is detected the commit is blocked. To install it in a fresh clone:
```bash
cp .git/hooks/pre-commit .git/hooks/pre-commit   # already present after clone
chmod +x .git/hooks/pre-commit
```

### Common commands

```bash
# Build binary
make build

# Run tests (with race detector)
make test

# Coverage report (opens coverage.html)
make test-cover

# Run all CI checks locally (lint, vet, staticcheck, govulncheck, test)
make ci

# Vet + lint + test + build
make all
```

For a live Ollama during development, you have two options:

**Option A — local Ollama install (recommended):**
```bash
# Install Ollama from https://ollama.com, then pull the model once:
ollama pull moondream
# Run the app — it connects to localhost:11434 by default:
DB_PATH=./dev.db PHOTO_LOCAL_PATH=./dev-photos ./kitchinv
```

**Option B — Docker sidecar with published port:**
```bash
# The compose file does not publish Ollama's port to the host.
# Run the container manually to expose it:
docker run -d --rm \
  -v kitchinv_ollama_data:/root/.ollama \
  -p 11434:11434 \
  ollama/ollama:latest
# Pull the model into it (one-time):
docker exec <container-id> ollama pull moondream
# Then run the app:
OLLAMA_HOST=http://localhost:11434 DB_PATH=./dev.db PHOTO_LOCAL_PATH=./dev-photos ./kitchinv
```

The server defaults to `DB_PATH=/data/kitchinv.db` and `PHOTO_LOCAL_PATH=/data/photos`. With direnv configured, your `.envrc` sets these automatically and you can just run:

```bash
go run ./cmd/kitchinv
```

---

## Configuration

All configuration is via environment variables. Every variable has a sensible default.

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `DB_PATH` | `/data/kitchinv.db` | SQLite database file path |
| `VISION_BACKEND` | `ollama` | Vision provider: `ollama` or `claude` |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama API base URL |
| `OLLAMA_MODEL` | `moondream` | Ollama vision model name |
| `CLAUDE_API_KEY` | *(required if backend=claude)* | Anthropic API key |
| `CLAUDE_MODEL` | `claude-opus-4-6` | Claude model ID |
| `PHOTO_BACKEND` | `local` | Photo storage backend (only `local` supported) |
| `PHOTO_LOCAL_PATH` | `/data/photos` | Directory for uploaded photo files |

---

For project layout, HTTP routes, and architecture details see [docs/architecture.md](docs/architecture.md).
