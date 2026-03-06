FROM ubuntu:24.04

# ── Labels ────────────────────────────────────────────────────────────────────
LABEL org.opencontainers.image.source="https://github.com/vbonduro/kitchinv"
LABEL org.opencontainers.image.description="kitchinv dev/CI environment"

ENV DEBIAN_FRONTEND=noninteractive

# ── 1. OS packages ────────────────────────────────────────────────────────────
# Core + beads deps (libicu-dev, libzstd-dev) + Playwright chromium system deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    git curl ca-certificates sudo build-essential locales \
    libicu-dev libzstd-dev \
    # Playwright / Chromium system dependencies
    libasound2t64 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libx11-6 \
    libxcomposite1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxkbcommon0 \
    libxrandr2 \
    libxshmfence1 \
    libxtst6 \
    && locale-gen en_US.UTF-8 \
    && rm -rf /var/lib/apt/lists/*

ENV LANG=en_US.UTF-8

# ── 2. Go 1.26.0 ──────────────────────────────────────────────────────────────
RUN curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz \
    | tar -C /usr/local -xz
RUN printf 'export PATH=/usr/local/go/bin:$PATH\n' > /etc/profile.d/go.sh

ENV PATH=/usr/local/go/bin:$PATH

# ── 3. Node.js 22.x ───────────────────────────────────────────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# ── 4. Go tools (staticcheck, govulncheck, beads/bd) ─────────────────────────
# Install to /usr/local/go-tools; reset GOPATH so runtime builds use ~/go
RUN GOPATH=/usr/local/go-tools go install honnef.co/go/tools/cmd/staticcheck@v0.7.0 \
    && GOPATH=/usr/local/go-tools go install golang.org/x/vuln/cmd/govulncheck@v1.1.4 \
    && GOPATH=/usr/local/go-tools go install github.com/steveyegge/beads/cmd/bd@v0.54.0

# ── 5. golangci-lint v2.10.1 ──────────────────────────────────────────────────
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    | sh -s -- -b /usr/local/bin v2.10.1

# ── 6. gitleaks 8.30.0 ────────────────────────────────────────────────────────
RUN curl -fsSL \
    https://github.com/gitleaks/gitleaks/releases/download/v8.30.0/gitleaks_8.30.0_linux_x64.tar.gz \
    | tar -C /usr/local/bin -xz gitleaks

# ── 7. Playwright chromium ────────────────────────────────────────────────────
ENV PLAYWRIGHT_BROWSERS_PATH=/usr/local/ms-playwright
RUN npm install -g @playwright/test@1.58.2 \
    && npx playwright install --with-deps chromium

# ── 8. Non-root user ──────────────────────────────────────────────────────────
RUN useradd -m -u 1000 -o -s /bin/bash vscode \
    && echo 'vscode ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers.d/vscode \
    && chmod 0440 /etc/sudoers.d/vscode

# Go tools binaries on PATH; GOPATH left unset so runtime uses ~/go
ENV PATH=/usr/local/go/bin:/usr/local/go-tools/bin:/usr/local/bin:$PATH

USER vscode
WORKDIR /workspace
