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

## Session State Persistence Incident 2026-04-19

- [x] Record the persistence scope and inspect the current code path for session restore
- [x] Re-run `gofmt`, `go test ./...`, and `npm run build`
- [x] Restart the managed backend and confirm migration `000007` applies cleanly
- [x] Verify `GET /api/v1/sessions/:id/state` returns session, preview, comparison, progress, and timeline
- [x] Run a live document flow and confirm refresh/re-entry restores backend state instead of losing progress
- [x] Record review notes and add the lesson for recoverable state

Review notes:
- Root cause: the existing product only persisted durable session and round artifacts; live progress and the activity timeline were effectively frontend-memory plus Redis SSE, so a refresh could lose the visible execution state even though the backend already had the core document outputs.
- Fix: add a dedicated backend session-state repository backed by PostgreSQL, persist progress snapshots into `session_progress`, reuse `audit_log` for timeline entries, wrap the live publisher with `TrackingPublisher`, and expose a single `GET /api/v1/sessions/:id/state` endpoint for frontend restore.
- Frontend decision: do not add browser SQLite for this path. The Vite app now restores from backend state and only keeps the active `sessionId` in `localStorage` as a pointer to the recoverable server-side session.
- Verification: `go test ./...` and `npm run build` passed after `gofmt`; the managed backend was restarted on `127.0.0.1:18081`; a live API run on `testdata/needs_polish_demo.txt` returned early `/state` data with `progress.phase=queued` and `timeline=["文档已提交"]`, then final `/state` data with preview, diff comparison, `progress.phase=complete`, and eight timeline entries.
- Browser proof: in headless Chromium against `http://127.0.0.1:18081`, the page was refreshed mid-processing and still restored `润色进行中`; after completion it showed `待开始下一轮`, the preview meta `6 段·已处理 6 个片段`, the persisted timeline, and a visible TXT download button.

## History Feature 2026-04-19

- [x] Extend `GET /sessions` to return `SessionListItem{session, progress, metrics}`
- [x] Add `round` to persisted timeline entries without a new SQL migration
- [x] Add `GET /sessions/:id/history` aggregated history endpoint
- [x] Add history-list query support for `q` and `sort`
- [x] Re-verify delete path cleans persisted history state
- [x] Add backend tests for metrics, history payloads, timeline round propagation, and list filters
- [x] Run `gofmt`, `go vet ./...`, and `go test ./... -count=1`

Review notes:
- The backend now exposes history-list metadata in one response instead of forcing the frontend to fan out on every row.
- Timeline grouping reused the existing `audit_log.detail` JSON payload by adding a `round` key, so no new migration was needed for history grouping.
- `GET /sessions/:id/history` aggregates the current session, persisted progress, derived metrics, per-round summaries, and an ascending full timeline in one payload.
- Search and sort stay server-side with a strict whitelist for `sort` and parameter binding for `q`, which keeps the API predictable and avoids unsafe SQL construction.
- Verification passed with `go vet ./...` and `go test ./... -count=1`; targeted handler and service tests cover the new list shape, history route, timeline round propagation, and delete cleanup behavior.

## SSE Timeout Incident 2026-04-19

- [x] Capture the stuck-looking round evidence from live `/state`, `/history`, and backend logs
- [x] Identify the root cause in HTTP server timeout configuration rather than workflow execution
- [x] Disable the default HTTP `write_timeout` that was truncating SSE after 30s
- [x] Update the example config so new environments inherit the streaming-safe default
- [x] Add a config regression test for the SSE-safe timeout default
- [x] Restart the managed backend and verify a single `/stream` connection survives past 30s and receives round 2 completion

Review notes:
- Root cause: the global HTTP `WriteTimeout` default was `30s`, and Go's server applies that limit to the whole SSE response lifetime. That caused `/api/v1/sessions/:id/stream` to be cut off at the 30-second mark even while the workflow kept running in the background.
- Failure evidence: the user-visible workspace froze at `第 2 / 2 轮 · 33% · 2/6` while the backend `/state` for the same session had already advanced to `round=2, phase=complete, percent=100`, and the backend access log showed the matching `/stream` request ending at exactly `30s`.
- Fix: change the default `http.write_timeout` to `0s` in `pkg/config/config.go`, mirror that in `deploy/config.example.yaml`, and add `pkg/config/config_test.go` to lock the default behavior.
- Verification: after restarting `naturalize-backend`, an idle SSE client stayed connected for `35s` until the client aborted it; then an end-to-end script kept one `/stream` connection open across both rounds and received the round 2 `complete` event at `36223ms`, which directly covers the previously broken time window.

## History Feature — Frontend 2026-04-19

- [x] C-F0 契约对齐（`Claude/SHARED_CONTRACT.md`、`Codex/SHARED_CONTRACT.md`、后端 `domain.SessionListItem` / `SessionHistory` / `SessionTimelineEntry.Round` 已对齐）
- [x] C-F1 `web/src/types.ts` 新增 `SessionListItem` / `SessionListResponse` / `RoundSummary` / `RoundHistoryEntry` / `SessionHistoryResponse`；`TimelineEntry.round` 改为可选
- [x] C-F2 `web/src/api.ts` 新增 `ListSessionsParams` / `listSessions(params)` / `getSessionHistory(id)`
- [x] C-F3 `/history`、`/history/:sessionId` 路由与菜单注册（`HistoryOutlined`）
- [x] C-F4 `HistoryPage.tsx`:搜索 debounce 300ms、状态筛选、排序、分页、删除确认、空态分「无记录」「无匹配」
- [x] C-F5 `HistoryDetailPage.tsx`:`SessionSummaryCard` + `RoundHistoryList` + `GroupedActivityTimeline`,Round Collapse 带下载/对比抽屉(复用只读 `CardReviewPanel`)
- [x] C-F6 `useSession.adopt(sessionId)` 设置 `localStorage` 并拉取 `getSessionState`,失败清 key
- [x] C-F7 工作台 `PageContainer.extra` 加「历史记录」按钮,空态提示从历史继续
- [x] C-F8 `useHistoryList` / `useHistoryDetail` hooks,`AbortController` 竞态处理
- [x] C-F9 `npm run build` 通过;`npm run lint` 维持 3 条存量报错(`use-agents.ts`、`use-session.ts`),新代码零新增报错
- [x] C-F10 更新 `tasks/todo.md` 与 PR 描述

PR 要点:
- 新路由 `/history`、`/history/:sessionId`;工作台补入口链接。
- 前端 API 新增 `listSessions(params)` / `getSessionHistory(id)` 并扩展类型。
- **破坏性变更**:`GET /api/v1/sessions` 响应从 `domain.Session[]` 改为 `SessionListItem[]`(包装 `session/progress/metrics`),旧调用方需更新。
- `CardReviewPanel` 增加 `readOnly` prop 供历史详情页复用,不破坏工作台现有行为。
- 已知风险:详情页复用 `useSession` 仅为调用 `adopt`,会在页面挂载时短暂开一个 EventSource(与工作台一致),后续可通过抽出 session context 优化。
- 手测清单(待手动过一遍):上传 → 列表出现 → 搜索收敛 → 状态筛选/排序 → 详情 Round Collapse 下载/对比 → 删除后刷新仍消失 → 暂停项继续处理跳回工作台。
