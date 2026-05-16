# Naturalize Project Snapshot

Audit date: 2026-04-24
Project path: `/Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize`
Depth: standard

## Responsibility Model

- Owner: repository maintainers.
- Executor: Codex audit pass.
- Reviewer perspective: security, engineering quality, delivery operations, dependency compliance.

## Repository Surface

- Backend: Go 1.26.1 module `github.com/iwen-conf/Naturalize`.
- Frontend: React 19, Vite 8, Ant Design 6.
- Runtime dependencies: PostgreSQL, Redis, OpenAI-compatible providers.
- Main documented flow: upload `.txt` or `.docx`, run rewrite workflow, publish SSE progress, export TXT/DOCX.

## Verification Commands

- `go test ./...`: passed.
- `go vet ./...`: passed.
- `staticcheck ./...`: failed with 2 findings.
- `cd web && npm run build`: passed with one large chunk warning.
- `cd web && npm run lint`: failed with 3 React Hooks errors.
- `cd web && npm audit --json`: 0 known npm vulnerabilities.
- `cd web && npm audit --omit=dev --json`: 0 known production npm vulnerabilities.
- `govulncheck ./...`: not run because `govulncheck` is not installed.
- Go license scan: local `go mod download -json all` plus LICENSE-file heuristic.

## Scope Boundaries

- Source code was not changed.
- Audit artifacts were written only under `.arc/audit/Naturalize/`.
- Browser smoke and Docker smoke scripts were not executed in this pass.
- Go vulnerability status is incomplete until `govulncheck` or another SCA tool runs in CI.
