#!/usr/bin/env bash
# bootstrap.sh — Install pinned dev tools to ~/.local/bin
# Idempotent: skips tools that are already at the correct version.
set -euo pipefail

# ── Pinned versions ────────────────────────────────────────────────────────────
GO_VERSION="1.26.0"
GOLANGCI_LINT_VERSION="v2.10.1"
STATICCHECK_VERSION="v0.7.0"
GOVULNCHECK_VERSION="v1.1.4"
GITLEAKS_VERSION="8.30.0"
BD_VERSION="v0.54.0"
NODE_WANT="22"

# ── Helpers ────────────────────────────────────────────────────────────────────
INSTALL_DIR="$HOME/.local/bin"
GO_DIR="$HOME/.local/bin/go"

mkdir -p "$INSTALL_DIR"

info()  { echo "[bootstrap] $*"; }
ok()    { echo "[bootstrap] ✓ $*"; }
warn()  { echo "[bootstrap] ⚠ $*" >&2; }

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        warn "Required command not found: $1"
        exit 1
    fi
}

need_cmd curl
need_cmd tar

# ── Go ─────────────────────────────────────────────────────────────────────────
CURRENT_GO=""
if [ -x "$GO_DIR/bin/go" ]; then
    CURRENT_GO=$("$GO_DIR/bin/go" version 2>/dev/null | awk '{print $3}' | sed 's/go//')
fi

if [ "$CURRENT_GO" = "$GO_VERSION" ]; then
    ok "Go $GO_VERSION already installed"
else
    info "Installing Go $GO_VERSION..."
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  GOARCH="amd64" ;;
        aarch64) GOARCH="arm64" ;;
        *)       warn "Unsupported arch: $ARCH"; exit 1 ;;
    esac
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    TARBALL="go${GO_VERSION}.${OS}-${GOARCH}.tar.gz"
    curl -fsSL "https://go.dev/dl/${TARBALL}" | tar -C "$HOME/.local/bin" -xz
    ok "Go $GO_VERSION installed to $GO_DIR"
fi

export PATH="$GO_DIR/bin:$INSTALL_DIR:$PATH"

# ── golangci-lint ──────────────────────────────────────────────────────────────
CURRENT_LINT=""
if [ -x "$INSTALL_DIR/golangci-lint" ]; then
    CURRENT_LINT=$("$INSTALL_DIR/golangci-lint" version 2>/dev/null | grep -oP '\d+\.\d+\.\d+' | head -1 || true)
fi
WANT_LINT="${GOLANGCI_LINT_VERSION#v}"

if [ "$CURRENT_LINT" = "$WANT_LINT" ]; then
    ok "golangci-lint $GOLANGCI_LINT_VERSION already installed"
else
    info "Installing golangci-lint $GOLANGCI_LINT_VERSION..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
        | sh -s -- -b "$INSTALL_DIR" "$GOLANGCI_LINT_VERSION"
    ok "golangci-lint $GOLANGCI_LINT_VERSION installed"
fi

# ── staticcheck ───────────────────────────────────────────────────────────────
GOPATH_DIR=$(go env GOPATH 2>/dev/null || echo "$HOME/go")
SC_BIN="$GOPATH_DIR/bin/staticcheck"
CURRENT_SC=""
if [ -x "$SC_BIN" ]; then
    CURRENT_SC=$("$SC_BIN" -version 2>/dev/null | grep -oP '\d{4}\.\d+\.\d+' | head -1 || true)
fi

if [ -x "$SC_BIN" ] && go version -m "$SC_BIN" 2>/dev/null | grep -q "honnef.co/go/tools.*${STATICCHECK_VERSION}"; then
    ok "staticcheck $STATICCHECK_VERSION already installed"
else
    info "Installing staticcheck $STATICCHECK_VERSION..."
    go install "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}"
    ok "staticcheck $STATICCHECK_VERSION installed to $GOPATH_DIR/bin"
fi

# ── govulncheck ───────────────────────────────────────────────────────────────
GV_BIN="$GOPATH_DIR/bin/govulncheck"
if [ -x "$GV_BIN" ] && go version -m "$GV_BIN" 2>/dev/null | grep -q "golang.org/x/vuln.*${GOVULNCHECK_VERSION}"; then
    ok "govulncheck $GOVULNCHECK_VERSION already installed"
else
    info "Installing govulncheck $GOVULNCHECK_VERSION..."
    go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
    ok "govulncheck $GOVULNCHECK_VERSION installed to $GOPATH_DIR/bin"
fi

# ── gitleaks ──────────────────────────────────────────────────────────────────
CURRENT_GL=""
if [ -x "$INSTALL_DIR/gitleaks" ]; then
    CURRENT_GL=$("$INSTALL_DIR/gitleaks" version 2>/dev/null || true)
fi

if [ "$CURRENT_GL" = "$GITLEAKS_VERSION" ]; then
    ok "gitleaks $GITLEAKS_VERSION already installed"
else
    info "Installing gitleaks $GITLEAKS_VERSION..."
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  GLARCH="x64" ;;
        aarch64) GLARCH="arm64" ;;
        *)       warn "Unsupported arch for gitleaks: $ARCH"; exit 1 ;;
    esac
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    curl -fsSL \
        "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_${OS}_${GLARCH}.tar.gz" \
        | tar -C "$INSTALL_DIR" -xz gitleaks
    ok "gitleaks $GITLEAKS_VERSION installed"
fi

# ── beads (bd) ────────────────────────────────────────────────────────────────
# Requires libicu-dev and libzstd-dev on Ubuntu/Debian for CGO deps.
BD_BIN="$GOPATH_DIR/bin/bd"
if [ -x "$BD_BIN" ] && go version -m "$BD_BIN" 2>/dev/null | grep -q "beads.*${BD_VERSION}"; then
    ok "beads (bd) $BD_VERSION already installed"
else
    # Warn about system deps on Debian/Ubuntu
    if command -v dpkg >/dev/null 2>&1; then
        if ! dpkg -l libicu-dev libzstd-dev >/dev/null 2>&1; then
            warn "beads requires libicu-dev and libzstd-dev. Install with:"
            warn "  sudo apt-get install -y libicu-dev libzstd-dev"
        fi
    fi
    info "Installing beads (bd) $BD_VERSION..."
    go install "github.com/steveyegge/beads/cmd/bd@${BD_VERSION}"
    ok "beads (bd) $BD_VERSION installed to $GOPATH_DIR/bin"
fi

# ── direnv ────────────────────────────────────────────────────────────────────
if command -v direnv >/dev/null 2>&1; then
    ok "direnv already installed ($(direnv version 2>/dev/null || echo 'unknown version'))"
else
    info "Installing direnv..."
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  DARCH="amd64" ;;
        aarch64) DARCH="arm64" ;;
        *)       warn "Unsupported arch for direnv: $ARCH"; exit 1 ;;
    esac
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    DIRENV_VERSION=$(curl -fsSL https://api.github.com/repos/direnv/direnv/releases/latest \
        | grep '"tag_name"' | grep -oP 'v[\d.]+')
    curl -fsSL \
        "https://github.com/direnv/direnv/releases/download/${DIRENV_VERSION}/direnv.${OS}-${DARCH}" \
        -o "$INSTALL_DIR/direnv"
    chmod +x "$INSTALL_DIR/direnv"
    ok "direnv $DIRENV_VERSION installed"
fi

# ── Node.js (advisory only) ───────────────────────────────────────────────────
NODE_OK=false
if command -v node >/dev/null 2>&1; then
    NODE_MAJOR=$(node --version | grep -oP '^\d+' || echo "0")
    if [ "$NODE_MAJOR" -ge "$NODE_WANT" ]; then
        ok "Node.js $(node --version) already installed"
        NODE_OK=true
    fi
fi

if [ "$NODE_OK" = "false" ]; then
    warn "Node.js $NODE_WANT+ not found. Please install it via nvm, fnm, or your OS package manager."
    warn "  nvm: https://github.com/nvm-sh/nvm"
    warn "  fnm: https://github.com/Schniz/fnm"
fi

# ── PATH reminder ─────────────────────────────────────────────────────────────
echo ""
echo "─────────────────────────────────────────────────────────────────────────"
echo "Bootstrap complete. Add the following to your shell profile if needed:"
echo ""
echo "  export PATH=\"$GO_DIR/bin:$INSTALL_DIR:\$PATH\""
echo "  export PATH=\"\$(go env GOPATH)/bin:\$PATH\""
echo ""
echo "Then reload your shell: source ~/.bashrc  (or ~/.zshrc)"
echo "─────────────────────────────────────────────────────────────────────────"
