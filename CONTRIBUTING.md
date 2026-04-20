# Contributing to OpenLobster

Thank you for contributing! This document explains how to open issues and pull requests, tests and linting expectations, and other workflows specific to this repository.

---

## Before you open an Issue or PR

- Search existing issues and PRs to avoid duplicates.
- Use the issue templates: choose **Bug report** or **Feature request** from the issue chooser (blank issues are disabled).
- For security vulnerabilities, do NOT post a public issue. Use GitHub Security Advisories or contact the maintainers privately.

## Opening a good issue

- Title: short and actionable (e.g. "Crash on startup when DB unavailable").
- For bugs include: environment (OS, backend binary/commit, frontend commit), exact steps to reproduce, expected vs actual behaviour, logs/screenshots and a minimal reproduction when possible.
- For features include: motivation, proposed solution, acceptance criteria, and alternatives considered.

## Pull request guidelines

- Keep PRs focused and small. If the change is large, open a draft PR and describe the migration/upgrade plan.
- Link related issues using keywords (e.g. "Fixes #123").
- Include automated tests for new behaviour and unit tests for bug fixes where applicable.
- Ensure CI passes before requesting review.

### PR checklist (add this to your description)

- [ ] I have read the repository rules and contribution requirements
- [ ] I ran the test suites and they pass locally
- [ ] I ran linters and formatters locally
- [ ] I updated user-facing documentation when applicable (docs/ or README)
- [ ] I did not commit secrets or personal config files
- [ ] Database migrations (if any) are included and tested

## Git pre-commit hook

After cloning, install the hook (copies [.github/hooks/pre-commit](.github/hooks/pre-commit) to `.git/hooks/`):

```bash
pnpm run hooks:install
```

## Local development — useful commands


Frontend (node/pnpm):

```
pnpm install
# Run frontend in dev mode
pnpm --filter @openlobster/frontend dev
# Run frontend tests and e2e
pnpm --filter @openlobster/frontend test
pnpm --filter @openlobster/frontend test:e2e
# Lint and build
pnpm --filter @openlobster/frontend lint
pnpm --filter @openlobster/frontend build
```

Backend (Go, via pnpm workspace scripts):

```
# Build the Go backend (runs the package's build script)
pnpm --filter @openlobster/backend build
# Run backend unit tests
pnpm --filter @openlobster/backend test
# Lint (runs the package lint script which invokes staticcheck)
pnpm --filter @openlobster/backend lint
# Formatting
gofmt -w .
golangci-lint run
```

General (workspace-level):

```
# Install dependencies for the workspace
pnpm install
# Run turbo build across the monorepo
pnpm -w build
# Run workspace lint (where configured)
pnpm -w lint
```

Adjust commands to your local environment; CI may run slightly different commands.

## Code style and linters

- JavaScript/TypeScript: follow the repo ESLint/Prettier configuration. Run `pnpm -w lint`.
- Go: run `gofmt -w .` and `golangci-lint run` before opening a PR.
- Tests should be deterministic and not rely on external services. Mock external AI providers or use the `mocks/` directory for test fixtures.

## Tests

- Add unit tests for new logic and regression tests for bugs.
- For frontend tests use the existing `vitest`/`playwright` setup; place tests under the appropriate `tests/` folder.
- For backend tests use standard `go test` packages and integration tests under `apps/backend/tests/` where appropriate.

## Commit messages

- Prefer Conventional Commits: `type(scope): short description` (examples: `fix(api): handle nil pointer`, `feat(ui): add dark mode`, `chore: update deps`).
- Keep the subject under ~72 characters and add a longer body if needed.

## Branching

- Create branches named `feature/<short-desc>`, `fix/<short-desc>`, or `chore/<short-desc>`.
- Rebase or squash as requested by maintainers before merging.

## CI and merging

- PRs must pass CI and have at least one approving review before merge.
- Use squash-and-merge for small fixes, or follow the repository's merge policy described by maintainers.

## Adding new dependencies

- New dependencies require a justification in the PR and a security review.
- For frontend use `pnpm`; for backend update `go.mod` and run `go mod tidy`.

## Licensing and file headers

- This project uses the LICENSE file at the repository root. Follow the repository rules for license headers when adding new files.

## Reporting security issues

- Use GitHub Security Advisories or contact repository owners directly. Avoid publishing exploit details publicly until a fix is available.

## Where to get help

- Use the repository Discussions (see `.github/ISSUE_TEMPLATE/config.yml`) for general questions and design proposals.

---

