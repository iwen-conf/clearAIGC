# Evidence Registry

## Project Intent And Flow

- `README.md:3`: defines Naturalize as a Go + React application for refining AI-written academic and technical drafts.
- `README.md:7-9`: states backend rewrite orchestration, quality checks, recovery flow, SSE progress, TXT/DOCX export, and React UI.
- `README.md:84-100`: documents expected verification commands for backend and frontend.
- `web/e2e/smoke.spec.ts:7-50`: browser smoke covers upload, processing completion, preview, review cards, and TXT download.

## Architecture And Runtime

- `cmd/server/main.go:34-120`: loads config, prepares storage, connects PostgreSQL and Redis, runs migrations, wires repositories, LLM clients, agent registry, workflow, and API router.
- `internal/api/router.go:14-17`: router uses Gin logger, recovery, and CORS middleware.
- `internal/api/router.go:20-47`: public API routes include session create/delete/export, round control, cards, health, and agent settings.
- `internal/service/service.go:31-45`: service owns repositories, pipeline, publisher, checkpoint store, exporter, layout, execution manager, prompt builder, and AI scorer.
- `internal/workflow/pipeline.go`: pipeline executes parse, chunk, LLM, quality gate, recovery, merge, and export behavior.
- `internal/infra/postgres/session_repo.go:77-165`: session list uses parameterized filters and sort allow-list.

## Security And Access Control

- `internal/api/router.go:14-17`: no authentication or authorization middleware is wired into the API group.
- `internal/api/router.go:55-79`: CORS middleware reflects any request origin when configured with `*` or empty allowed origins.
- `pkg/config/config.go:271-275`: default HTTP listen is `:8080`, write timeout is `0`, and allowed origins default to `*`.
- `internal/api/handler/agent_settings.go:12-16`: agent settings request accepts `protocol`, `baseUrl`, `apiKey`, and `model`.
- `internal/api/handler/agent_settings.go:28-44`: public handler updates agent settings from request body.
- `internal/infra/postgres/agent_settings_repo.go:63-73`: `api_key` is persisted directly in the `agent_settings` table.
- `internal/api/handler/handler.go:49-67`: single upload calls `c.FormFile` before service validation.
- `internal/api/handler/handler.go:81-109`: batch upload calls `c.MultipartForm` and opens each file before service validation.
- Exact search result: no `MaxBytesReader`, `MaxMultipartMemory`, or explicit multipart body cap exists under `internal`, `pkg`, or `cmd`.
- `internal/service/service.go:98-100`: service rejects file sizes above 50 MiB after multipart parsing.
- `pkg/storage/storage.go:40-81`: upload filenames are sanitized with `filepath.Base` and separator replacement before storage path use.

## Observability And Delivery

- `internal/api/health.go:36-45`: readiness checks PostgreSQL and Redis.
- `internal/api/health.go:65-68`: health response reports `llm: configured` rather than probing providers.
- `deploy/docker-compose.yml:7-18`: demo app defaults include dummy provider keys and mock OpenAI URLs.
- `deploy/docker-compose.yml:26-41`: service exposes `8080:8080` and uses a hard-coded demo PostgreSQL password.
- `deploy/Dockerfile:21-28`: final image is `alpine:3.22`, copies binary/assets, exposes 8080, and does not set a non-root user.

## Tests And Static Checks

- `go test ./...`: passed across 14 backend packages.
- `go vet ./...`: passed.
- `staticcheck ./...`: failed:
  - `internal/domain/scorer/requester.go:47`: S1024, prefer `time.Until`.
  - `internal/domain/scorer/signals.go:162`: U1000, unused `readabilityPenalty`.
- `web/package.json:6-12`: frontend scripts include `build`, `lint`, and Playwright smoke.
- `cd web && npm run build`: passed; Vite reported one 1,459.29 kB minified JS chunk, 467.20 kB gzip.
- `cd web && npm run lint`: failed:
  - `web/src/hooks/use-agents.ts:33`: `react-hooks/set-state-in-effect`.
  - `web/src/hooks/use-session.ts:121`: `react-hooks/refs`.
  - `web/src/hooks/use-session.ts:126`: `react-hooks/set-state-in-effect`.

## Dependencies And Licenses

- `go.mod:1-13`: Go module and direct dependencies include Eino, Gin, pgx, Redis, and Viper.
- `web/package.json:14-36`: frontend direct dependencies include Ant Design, React, React Router, TypeScript, Vite, ESLint, and Playwright.
- `npm audit --json`: 0 known vulnerabilities across production and dev npm dependencies.
- npm lock license scan: 332 lock packages scanned, 0 high-risk licenses, 0 unknown license fields.
- Go license scan: 124 module records scanned after `go mod download -json all`; 118 permissive, 1 MPL, 0 GPL/AGPL/non-commercial/Commons Clause/BUSL, 5 unknown.
- Go vulnerability scan: `govulncheck` was not installed, so Go vulnerability status remains incomplete.
