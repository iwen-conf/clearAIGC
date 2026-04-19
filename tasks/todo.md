# Naturalize — Project Tracker

## Repository Bootstrap

- [x] Review workspace and remote repository state
- [x] Define `.gitignore` rules for local tools, build artifacts, and document outputs
- [x] Remove legacy nested repository content (`baibaiAIGC/`)
- [x] Initialize Git, configure GitHub remote, and create the initial commit
- [x] Push the initial branch and verify upstream tracking

## Architecture & Documentation

- [x] Analyze existing pipeline, state persistence, and resume logic
- [x] Validate Eino workflow and ReAct orchestration capabilities
- [x] Produce rewrite architecture: deterministic workflow + bounded agent reasoning
- [x] Define numbered documentation taxonomy and stable reading order
- [x] Deliver architecture documents (master-plan, vision, modules, workflow, infra, api, observability, security)
- [x] Validate docs tree and repair all cross-references

## Implementation

- [x] Scaffold Go project structure (`cmd/server`, `internal/`, `pkg/`, `deploy/`)
- [x] Implement document intake, 4-level chunking, and Manifest persistence
- [x] Implement Eino Workflow orchestration: deterministic graph execution, resumable chunk processing, quality gate, export
- [x] Add bounded ReAct Agent for quality recovery (retry_strict, split_rewrite, accept)
- [x] Implement Redis CheckPointStore and Pub/Sub progress streaming
- [x] Implement REST API layer (Gin + SSE + health check)
- [x] Build customer-facing React UI against the new Go API backend
- [x] Validate core backend behavior with `.txt` / `.docx` unit and transport tests
- [x] Run a full local end-to-end smoke test with Docker, frontend build, API, Redis, PostgreSQL, and a mock OpenAI-compatible provider
- [x] Persist section-level comparison data and expose the documented `/sessions/{id}/diff` API
- [x] Add the documented `/sessions/batch` upload API and verify multi-file creation in the local stack
- [x] Extend the customer-facing review experience with a collapsed section-by-section comparison panel
- [x] Add a reusable Playwright browser smoke test for the customer-facing flow
- [ ] Run the browser smoke harness against live OpenAI credentials and infrastructure

## Review

- The most reusable assets are: chunking rules, round-state derivation, prompt assets, resume semantics, and export behavior.
- The most replaceable parts are: Python transport layers, ad hoc JSON file storage, direct LLM calls inside round execution, and the legacy desktop bridge.
- Target shape: deterministic Workflow as the main execution path, with ReAct limited to exception handling, tool selection, validation, and adaptive retry.
- The `docs/` tree has a stable numbered reading order; all markdown relative links resolve correctly.
- Current implementation scope now includes the customer-facing frontend: upload, round execution, pause/resume, quality recovery, output export, SSE progress, and the browser workflow are all present in-repo.
- The orchestration layer is implemented as a deterministic Eino graph rather than a field-mapped workflow because the current Eino API is substantially cleaner for a single mutable pipeline state while preserving the same DAG behavior.
- The repository now includes a Vite + React frontend under `web/`, serves the production build from the Go server when `web/dist` exists, and keeps the frontend in a nested module boundary so `go test ./...` does not walk `web/node_modules`.
- The repository now has a local demo mode that needs no external model credentials: Docker builds the frontend and backend together, the mock provider speaks enough of the OpenAI surface for the happy path, and `scripts/smoke_mock_e2e.sh` verifies upload → processing → output.
- The persistence model now stores finalized section outputs and review results inside manifest JSON, which allows the product to serve a section-by-section comparison view without adding a schema migration.
- The customer-facing frontend now keeps detailed comparison behind a collapsed review panel so the primary flow stays focused while still allowing closer review when needed.
- Local verification now covers the new `/sessions/{id}/diff` endpoint and `/sessions/batch` endpoint in addition to the existing single-document smoke path.
- The repository now includes a Playwright browser smoke harness that exercises upload, live progress, preview, section comparison, and TXT download locally; the remaining unchecked item is running the same path with live provider credentials.

## Dev Proxy Incident 2026-04-18

- [x] Capture failing evidence for `vite` proxy ECONNREFUSED on `/api/v1/agents`
- [x] Identify root cause in `web/vite.config.ts` default proxy target vs active backend listen port
- [x] Implement safe dev-proxy target resolution for local backend ports
- [x] Verify with build and runtime reachability checks
- [x] Record review notes and regression guidance

Review notes:
- Root cause: the active local backend was listening on `127.0.0.1:18081`, while Vite defaulted to `http://localhost:8080`, so proxied API requests were sent to an unopened port.
- Fix: keep `VITE_API_PROXY_TARGET` as the override, otherwise probe `127.0.0.1:18081` before `127.0.0.1:8080` and log the selected target at Vite startup.
- Verification: `npm run build` now reports `[vite] proxying /api to http://127.0.0.1:18081`, and `curl http://127.0.0.1:5176/api/v1/agents` returns the backend JSON through the Vite proxy.
- Residual risk: `npm run lint` still fails on pre-existing React hook issues in `web/src/hooks/use-agents.ts` and `web/src/hooks/use-session.ts`; this incident fix does not change those files.

## Mock Rewrite Incident 2026-04-19

- [x] Reproduce the no-op rewrite with `testdata/needs_polish_demo.txt` and confirm round 1 diff shows `changedCount=0`
- [x] Identify the concrete root cause in `deploy/mock-openai/server.mjs`
- [x] Replace the Chinese mock rewrite path with deterministic round-aware transformations for round 1 and round 2
- [x] Expose local debug evidence from the mock rewrite path
- [x] Route the project backend to the repository-local mock service instead of the unrelated `clearAIGC` mock container
- [x] Restart managed local services and verify health
- [x] Re-run round 1 + round 2 against the live API and confirm both rounds produce changed chunks and non-trivial AI rates
- [x] Re-run `go test ./...` and `go vet ./...`

Review notes:
- Root cause: the project backend was effectively consuming a mock provider that returned Chinese input unchanged, so the workflow completed, spent tokens, and emitted chunk-complete events without producing any meaningful rewrite.
- Secondary environment issue: the local backend had been pointed at an unrelated `clearAIGC` mock on `127.0.0.1:18787`; fixing only this repository's Docker mock was insufficient until the backend was repointed to a repository-local mock process.
- Fix: `deploy/mock-openai/server.mjs` now parses `[ROUND n]`, applies deterministic Chinese rewrite heuristics per round, and logs concise input/output debug evidence for each request.
- Runtime contract: the repository-local mock is now managed in tmux on `127.0.0.1:18788`, and the repository-local backend is managed in tmux on `127.0.0.1:18081` with provider URLs pointing to that mock.
- Verification: before the fix, round 1 on `needs_polish_demo.txt` returned `changedCount=0`; after the fix, round 1 returned `changedCount=6` with visible rewrites, round 2 returned `changedCount=6` with further refinement, and diff payloads now carry non-trivial `aiRate` values.
