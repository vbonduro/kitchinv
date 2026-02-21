# Development Workflow for AI Agents

See `CLAUDE.md` for the full development workflow.

Key rules:
- **All work happens on a feature branch**, never directly on `main`
- **Open a PR** and wait for CI + owner approval before merging
- **Use beads** (`bd`) for issue tracking — never TodoWrite or markdown task lists
- **Run `make ci` locally before every `git push`** — this runs lint, vet, staticcheck, govulncheck, and tests
- Run `bd sync` before every `git push`

## Environment & secrets

- Environment variables are managed via **direnv** + `.envrc` (gitignored). The `.envrc` is in the repo root and is visible from all worktrees.
- Secrets (e.g. `CLAUDE_API_KEY`) are stored in **`pass`** (GPG-encrypted). The `.envrc` calls `$(pass kitchinv/claude-api-key)` to inject the key at shell load time.
- **`pass` requires the GPG agent to be unlocked.** In a non-interactive shell (e.g. when an AI agent runs commands), the GPG agent may not be running and `pass` will return empty, causing the server to fail with `CLAUDE_API_KEY is required`. Fix: have the user run `pass kitchinv/claude-api-key` in their own terminal first to unlock the GPG agent, then agent-run commands will work for the duration of that session.
