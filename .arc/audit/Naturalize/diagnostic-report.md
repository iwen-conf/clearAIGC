# Diagnostic Report

## Observed Facts

Naturalize has a coherent layered shape: a Go backend, React frontend, PostgreSQL and Redis infrastructure, OpenAI-compatible provider clients, storage layout, workflow nodes, and API handlers are separated by package. The README documents local demo, live provider, backend, frontend, and smoke verification paths.

Automated backend verification is healthy at the unit level. `go test ./...` and `go vet ./...` passed. The frontend production build also passed. The project has a browser smoke specification for the main upload-to-download flow.

The main security boundary is weak. The API group is registered with only logger, recovery, and CORS middleware. Public routes include session creation, deletion, export, card mutation, round control, and agent settings mutation. Agent settings accept and persist API keys. Default CORS allows all origins by reflecting any request origin when `*` is configured.

Upload validation exists in the service layer, but HTTP body limits are not enforced before multipart parsing. The handlers call `c.FormFile` and `c.MultipartForm`; the service rejects files over 50 MiB only after the multipart parser has already accepted the request body.

Operational readiness is partial. `/ready` checks PostgreSQL and Redis, but `/health` reports LLM as `configured` without a provider probe. Docker Compose is fit for local demo but uses exposed port 8080, dummy provider defaults, and a hard-coded demo PostgreSQL password. The final image does not set a non-root user.

Engineering checks are not fully clean. `staticcheck` reports one unused function and one simple time helper issue. `npm run lint` fails on three React Hooks rules. These do not block the production build but should block CI before release.

Dependency evidence is mixed. npm audit reports 0 known vulnerabilities and the npm lockfile license scan found no high-risk or unknown license fields. Go license scanning found no GPL, AGPL, non-commercial, Commons Clause, or BUSL licenses, but 5 Go modules could not be classified by the local heuristic. `govulncheck` was unavailable, so Go vulnerability status is not complete.

## Findings

### F1 - Public API exposes destructive and secret-bearing operations

Severity: High
Confidence: High

Evidence:
- `internal/api/router.go:14-17` wires only logger, recovery, and CORS middleware.
- `internal/api/router.go:20-47` exposes create, delete, export, card mutation, round control, and agent settings routes.
- `pkg/config/config.go:271-275` defaults allowed origins to `*`.
- `internal/api/handler/agent_settings.go:12-16` accepts `apiKey`.
- `internal/infra/postgres/agent_settings_repo.go:63-73` persists `api_key`.

Impact:
Any caller that can reach the service can mutate sessions, download outputs, change model provider settings, and write or replace API keys. If exposed outside a trusted local network, this is a direct data and credential risk.

### F2 - Upload request body can be parsed before size rejection

Severity: High
Confidence: High

Evidence:
- `internal/api/handler/handler.go:49-67` uses `c.FormFile`.
- `internal/api/handler/handler.go:81-109` uses `c.MultipartForm`.
- Exact search found no `MaxBytesReader`, `MaxMultipartMemory`, or equivalent cap.
- `internal/service/service.go:98-100` applies the 50 MiB limit after handler multipart parsing.

Impact:
Large or repeated multipart requests can consume memory, temporary disk, and handler time before service validation rejects the file.

### F3 - Frontend lint currently fails

Severity: Medium
Confidence: High

Evidence:
- `npm run lint` exits with 3 errors.
- `web/src/hooks/use-agents.ts:33` triggers `react-hooks/set-state-in-effect`.
- `web/src/hooks/use-session.ts:121` triggers `react-hooks/refs`.
- `web/src/hooks/use-session.ts:126` triggers `react-hooks/set-state-in-effect`.

Impact:
The frontend build passes, but the repository cannot enforce a clean lint gate. The flagged hooks also indicate render/effect patterns that React 19 tooling considers risky.

### F4 - Staticcheck fails on dead code and a time helper issue

Severity: Low
Confidence: High

Evidence:
- `internal/domain/scorer/requester.go:47` uses `r.lastAt.Add(r.interval).Sub(time.Now())`.
- `internal/domain/scorer/signals.go:162` defines unused `readabilityPenalty`.
- `staticcheck ./...` exits non-zero.

Impact:
Small but real engineering-discipline gap. This is simple to fix and should not remain in a project that otherwise has good unit test coverage.

### F5 - Health and deployment signals are demo-grade, not production-grade

Severity: Medium
Confidence: Medium

Evidence:
- `internal/api/health.go:36-45` checks only PostgreSQL and Redis readiness.
- `internal/api/health.go:65-68` reports `llm: configured`.
- `deploy/docker-compose.yml:7-18` includes dummy provider key defaults.
- `deploy/docker-compose.yml:36-41` uses a hard-coded PostgreSQL password.
- `deploy/Dockerfile:21-28` does not set a non-root runtime user.

Impact:
The service can appear healthy while the provider path is broken. The Docker artifacts are useful for local demo, but they should not be treated as production deployment hardening.

### F6 - Dependency compliance and Go vulnerability status are incomplete

Severity: Medium
Confidence: Medium

Evidence:
- `go.mod:1-13` and `web/package.json:14-36` define the dependency manifests.
- npm audit found 0 known vulnerabilities.
- Go license scan found 0 high-risk licenses, 1 MPL dependency, and 5 unknown Go module licenses.
- `govulncheck` was not installed.

Impact:
No immediate high-risk license was observed, but release compliance is not complete until the 5 unknown Go modules are manually resolved and a Go vulnerability scanner runs in CI.
