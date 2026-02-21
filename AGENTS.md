# Development Workflow for AI Agents

See `CLAUDE.md` for the full development workflow.

Key rules:
- **All work happens on a feature branch**, never directly on `main`
- **Open a PR** and wait for CI + owner approval before merging
- **Use beads** (`bd`) for issue tracking — never TodoWrite or markdown task lists
- **Run `make ci` locally before every `git push`** — this runs lint, vet, staticcheck, govulncheck, and tests
- Run `bd sync` before every `git push`
