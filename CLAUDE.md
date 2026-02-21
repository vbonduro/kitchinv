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
```

Go is installed at `~/.local/bin/go/bin/go`. If `make` cannot find it, run commands directly:
```bash
~/.local/bin/go/bin/go test -race -count=1 ./...
```

## Beads sync

Run `bd sync` before every `git push` — the pre-push hook requires it.
