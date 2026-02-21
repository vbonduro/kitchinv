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

## Commands

```bash
make build       # compile binary
make test        # run tests with race detector
make all         # vet + lint + test + build
make ci          # run full CI workflow locally via act (see below)
```

Go is installed at `~/.local/bin/go/bin/go`. The Makefile auto-detects it, so
`make test` / `make build` work without any PATH changes.

## Running CI locally

`make ci` uses [act](https://github.com/nektos/act) to replay
`.github/workflows/ci.yml` inside Docker containers that mirror the
`ubuntu-latest` GitHub Actions environment.

**First run:** `make ci` installs `act` into `.tools/` automatically (no sudo
needed). Docker must be running.

```bash
make ci          # runs all CI jobs: lint, vet, staticcheck, govulncheck, test
```

On the very first invocation act will ask which Docker image to use for
`ubuntu-latest`. Choose **Medium** (`catthehacker/ubuntu:act-latest`) for a
good balance of size (~500 MB) and tool availability. This choice is saved to
`~/.actrc` so you won't be asked again.

## Beads sync

Run `bd sync` before every `git push` — the pre-push hook requires it.
