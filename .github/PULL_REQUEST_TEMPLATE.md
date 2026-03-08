<!--
Please fill out this template to help reviewers quickly understand
what the PR changes and how to validate it. Project documentation
is English-first; a short Spanish note is provided below.
-->

## Summary

Provide a short description of the change and the reason for it.

## Related issues

- Fixes: A brief description of the issue, e.g. "Fixes updating the API endpoint to handle new parameters."

## Type of change
- [ ] Bugfix
- [ ] New feature
- [ ] Documentation
- [ ] Refactor
- [ ] Test-only change

## How to test / QA steps

Describe manual steps and automated commands to validate the change, e.g.

1. Install workspace dependencies: `pnpm install`
2. Build backend: `pnpm --filter @openlobster/backend build`
3. Build frontend: `pnpm --filter @openlobster/frontend build`
4. Run unit tests: `pnpm --filter @openlobster/frontend test` and `pnpm --filter @openlobster/backend test`
5. Lint: `pnpm --filter @openlobster/frontend lint` and `pnpm --filter @openlobster/backend lint`
6. Steps to reproduce the issue (if applicable)

## Checklist (required before merge)
- [ ] I have read the repository rules and contribution requirements
- [ ] All unit and integration tests pass locally and in CI
- [ ] The project builds without errors (frontend and backend)
- [ ] Linters and formatters pass (ESLint/Prettier, gofmt/golangci-lint)
- [ ] I updated user-facing documentation when applicable (docs/ or README)
- [ ] I did not add secrets or personal config files
- [ ] Database migrations added (if schema changes) and tested
- [ ] I added or updated tests for relevant changes

## Deployment notes / Rollback plan

Any special deployment steps, feature flags, or rollbacks.

## Notes for the reviewer

- Highlight non-obvious design decisions, API changes, or migration steps.

---

ES (resumen breve):

Incluye una descripción corta en español si quieres; la plantilla principal debe permanecer en inglés para la documentación del proyecto.
