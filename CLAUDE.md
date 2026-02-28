# Development Workflow

## Branch & PR Process

Every change — no matter how small — must follow this process:

1. **Create a feature branch** from `main` before writing any code.
   ```bash
   git checkout main && git pull
   git checkout -b <short-descriptive-name>
   ```

2. **Do the work** on that branch. Commit as needed.

3. **Push the branch and open a PR.**
   ```bash
   git push -u origin <branch-name>
   gh pr create --title "..." --body "..."
   ```

4. **Wait for CI to pass.** Do not merge until all checks are green.

5. **Wait for owner approval.** Do not merge until the PR is approved by @vbonduro.

6. **Merge only after approval + green CI.** Use the GitHub UI or:
   ```bash
   gh pr merge <number> --squash
   ```

**Never commit directly to `main`.** Never push to `main`. Never merge your own PR.

## Issue Tracking

Use beads for all task tracking. Before starting work, mark the relevant issue `in_progress`:
```bash
bd update <id> --status=in_progress
```

Close issues when the PR merges, not before:
```bash
bd close <id>
```

## TDD Workflow

Follow this cycle for all bug fixes and new features:

1. **Write a failing test first** that reproduces the issue or specifies the behaviour.
2. **Run the test and confirm it fails** for the right reason.
3. **Write the minimum code** to make the test pass.
4. **Run the test and confirm it passes.**
5. **Run the full suite** (`make test`) to ensure nothing is broken.
6. **Spin up the service and verify manually** where applicable.

For server-side logic, prefer unit/integration tests in `internal/service/` or `internal/web/`.
For UI behaviour that can't be tested server-side, cover it with E2E tests in `e2e/`.

## Commands

```bash
make build       # compile binary
make test        # run tests with race detector
make all         # vet + lint + test + build
make ci          # run all CI checks locally (lint, vet, staticcheck, govulncheck, test, e2e)
make e2e         # run E2E tests only
```

Go is installed at `~/.local/bin/go/bin/go`. The Makefile auto-detects it, so
`make test` / `make build` work without any PATH changes.

## Running CI locally

`make ci` runs all checks that GitHub Actions runs, directly against your
locally installed tools — no Docker required. This includes E2E tests via
`make e2e`. See the README for tool installation instructions.

Run E2E tests directly (faster, no npm install):
```bash
cd e2e && APP_PORT=9090 OLLAMA_PORT=19434 npx playwright test
```

## Beads sync

Run `bd sync` before every `git push` — the pre-push hook requires it.

## Environment & secrets

- Environment variables are managed via **direnv** + `.envrc` (gitignored). The `.envrc` is in the repo root and is visible from all worktrees.
- Secrets (e.g. `CLAUDE_API_KEY`) are stored in **`pass`** (GPG-encrypted). The `.envrc` calls `$(pass kitchinv/claude-api-key)` to inject the key at shell load time.
- **`pass` requires the GPG agent to be unlocked.** In a non-interactive shell the GPG agent may not be running and `pass` will return empty, causing the server to fail with `CLAUDE_API_KEY is required`. Fix: run `pass kitchinv/claude-api-key` in your own terminal first to unlock the GPG agent, then commands in that session will work.
