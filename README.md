# kitchinv — Home Kitchen Inventory

> Never wonder what's in the freezer again.

[![CI](https://github.com/vbonduro/kitchinv/actions/workflows/ci.yml/badge.svg)](https://github.com/vbonduro/kitchinv/actions/workflows/ci.yml)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen.svg?style=flat)](https://goreportcard.com/report/github.com/vbonduro/kitchinv)
[![codecov](https://codecov.io/gh/vbonduro/kitchinv/branch/main/graph/badge.svg)](https://codecov.io/gh/vbonduro/kitchinv)

kitchinv lets you photograph every storage area in your home (fridge, freezer, pantry, …), automatically extracts a food-item inventory from each photo using a vision model, and gives you a searchable list across all areas — all from your phone browser.

---

## Contents

- [How it works](#how-it-works)
- [Quick start (Docker)](#quick-start-docker)
- [Switching to Claude](#switching-to-claude)
- [Switching to Gemini](#switching-to-gemini)
- [Deploying on Unraid](#deploying-on-unraid)
- [Local development](#local-development)
- [Configuration](#configuration)

---

## How it works

1. **Create an area** — give each physical storage location a name ("Upstairs Fridge", "Garage Freezer", "Pantry").
2. **Upload a photo** — tap the camera button from your phone; the rear camera opens directly.
3. **Vision analysis** — the photo is sent to a vision model (Ollama by default; Claude or Gemini as alternatives). The model identifies each food item and returns structured JSON.
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

## Switching to Gemini

Set two environment variables (no Ollama sidecar needed):

```bash
VISION_BACKEND=gemini
GEMINI_API_KEY=<your-google-ai-key>
```

Or run without Compose:

```bash
docker run \
  -e VISION_BACKEND=gemini \
  -e GEMINI_API_KEY=<your-google-ai-key> \
  -e DB_PATH=/data/kitchinv.db \
  -e PHOTO_LOCAL_PATH=/data/photos \
  -v kitchinv_data:/data \
  -p 8080:8080 \
  kitchinv:latest
```

The Gemini model defaults to `gemini-2.5-flash`. Override with `GEMINI_MODEL=gemini-3-flash-preview` or any model listed by the [Gemini API](https://ai.google.dev/gemini-api/docs/models).

Get an API key at [aistudio.google.com](https://aistudio.google.com/).

> **Note:** The vision prompt is tuned specifically for `gemini-2.5-flash`. Using a different model may reduce item detection accuracy. See [docs/vision-model-benchmark.md](docs/vision-model-benchmark.md) for the full benchmark results and model comparison.

---

## Deploying on Unraid

kitchinv runs well as a Docker container on Unraid. The recommended setup keeps the app off the public internet (access via Tailscale only) and stores API keys in files rather than environment variables (so they don't appear in `docker inspect` or process listings).

### Prerequisites

**1. Tailscale** (strongly recommended)

Install the Tailscale plugin from Unraid Community Applications. Once installed, your Unraid server joins your tailnet and kitchinv is only reachable from your own devices — not the public internet. Do **not** expose port 8080 via your router's port forwarding.

**2. A Gemini API key**

Get one free at [aistudio.google.com](https://aistudio.google.com/). The vision prompt is tuned for `gemini-2.5-flash` — see [docs/vision-model-benchmark.md](docs/vision-model-benchmark.md) for model comparison.

### Step 1: Create the appdata directories

In Unraid's terminal (or via SSH):

```bash
mkdir -p /mnt/user/appdata/kitchinv/data
mkdir -p /mnt/user/appdata/kitchinv/secrets
```

### Step 2: Store your API key securely

Write your Gemini API key to a file with tight permissions:

```bash
echo -n "YOUR_GEMINI_API_KEY" > /mnt/user/appdata/kitchinv/secrets/gemini_key
chown 100:100 /mnt/user/appdata/kitchinv/secrets/gemini_key
chmod 600 /mnt/user/appdata/kitchinv/secrets/gemini_key
```

The container runs as UID 100 (`kitchinv` user). Setting ownership to `100:100` ensures the container process can read the file. This keeps the key out of `docker inspect`, `ps` output, and the Unraid template XML.

### Step 3: Install the template

Download the template file to Unraid's user templates directory:

```bash
wget -P /boot/config/plugins/dockerMan/templates-user/ \
  https://raw.githubusercontent.com/vbonduro/kitchinv/main/deploy/unraid/kitchinv.xml
```

Then in the Unraid **Docker** tab, click **Add Container**, select the **kitchinv** template from the template dropdown, verify the settings, and click **Apply**.

### Step 4: Access kitchinv

Once the container is running, open it from any device on your Tailscale network:

```
http://<unraid-tailscale-ip>:8080
```

Or set a Tailscale MagicDNS hostname (e.g. `http://unraid:8080`).

### Updating

When a new version is released (tagged on GitHub), update by pulling the new image in the Unraid Docker tab:

1. Click the kitchinv container row
2. Click **Check for Updates** → **Update**
3. The container restarts with the new image; your data in `/mnt/user/appdata/kitchinv/data` is preserved

Alternatively, enable **Watchtower** (available in Community Apps) to auto-pull and restart containers when new image versions are published to GHCR.

---

## Local development

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev/dl](https://go.dev/dl/) — extract to `~/.local/bin/go` or anywhere on `$PATH` |
| Node.js | 22+ | [nodejs.org](https://nodejs.org/) — required for E2E tests only |
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

**3. Store secrets with `pass`** (recommended — GPG-encrypted, works in any terminal):
```bash
# Install pass and create a GPG key (one-time):
sudo apt-get install pass
gpg --gen-key
pass init <your-gpg-email>

# Store your API keys:
pass insert kitchinv/claude-api-key
pass insert kitchinv/gemini-api-key
```
The `.envrc.example` shows how to reference them via `$(pass kitchinv/claude-api-key)`.

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

# Run E2E browser tests (headless, requires Node 22+)
make e2e

# Run E2E tests with a visible browser (useful for debugging)
make e2e-headed
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
| `VISION_BACKEND` | `ollama` | Vision provider: `ollama`, `claude`, or `gemini` |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama API base URL |
| `OLLAMA_MODEL` | `moondream` | Ollama vision model name |
| `CLAUDE_API_KEY` | *(required if backend=claude)* | Anthropic API key |
| `CLAUDE_API_KEY_FILE` | *(optional)* | Path to file containing Anthropic API key (takes precedence over `CLAUDE_API_KEY`) |
| `CLAUDE_MODEL` | `claude-opus-4-6` | Claude model ID |
| `GEMINI_API_KEY` | *(required if backend=gemini)* | Google AI API key |
| `GEMINI_API_KEY_FILE` | *(optional)* | Path to file containing Google AI API key (takes precedence over `GEMINI_API_KEY`) |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Gemini model ID |
| `PHOTO_BACKEND` | `local` | Photo storage backend (only `local` supported) |
| `PHOTO_LOCAL_PATH` | `/data/photos` | Directory for uploaded photo files |

---

For project layout, HTTP routes, and architecture details see [docs/architecture.md](docs/architecture.md).
