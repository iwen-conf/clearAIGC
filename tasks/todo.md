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

## Findings Alignment 2026-04-19

- [x] Audit the current code against `docs/03-findings` and lock a deliverable subset
- [x] Rename product-facing `AI率` / detector wording to internal heuristic-risk terminology
- [x] Reword the Chinese dual-pass flow so round 2 is clearly optional, not implied mandatory
- [x] Add whole-round heuristic-risk regression fallback on top of the existing chunk-level protection
- [x] Update or add backend/frontend tests for the new behavior
- [x] Run focused verification (`go test ./...`, `npm run build`, targeted assertions) and record review notes

Review notes:
- Scope was intentionally limited to the P0/P1 items that fit the current architecture: terminology alignment, optional second-round UX, safer rollback, and better propagation of round intent into the coordinator worker prompts.
- The backend now applies two layers of heuristic-risk protection: existing chunk-level no-worse fallback plus a new whole-round rollback when the merged output regresses against the round input.
- Coordinator worker prompts no longer receive only raw text; they now also receive the round goal and the current heuristic-risk features so the actual lexical/syntax agents better reflect round-specific intent.
- Frontend copy now consistently frames the metric as an internal heuristic-risk score and frames Chinese round 2 as optional refinement instead of mandatory "final polishing."
- Verification passed with `go test ./...` and `npm run build`; the frontend build still emits the pre-existing Vite large-chunk warning, but there are no new build failures.

## Findings Alignment Follow-up 2026-04-19

- [x] Add deterministic fact-invariant checks to the quality gate for numbers, percentages, currencies, years/dates, and citation-like markers
- [x] Route fact-invariant failures through the existing recovery path with explicit failure reasons
- [x] Add quality-gate tests for factual drift pass/fail cases
- [x] Re-run focused verification and record review notes

Review notes:
- The new quality-gate check is intentionally deterministic and overlap-aware: it extracts high-priority factual markers first (citations, dates, currencies, percentages, years, then plain numbers) and compares them as multisets between input and output.
- This does not replace semantic fidelity checking, but it closes one of the highest-risk gaps from `docs/03-findings`: the system now rejects obvious factual drift even when the rewrite remains fluent.
- Recovery integration stayed minimal by design. Fact-invariant failures reuse the existing strict-retry path, but the supervisor prompt now explicitly treats numeric/date/citation drift as a retry case.
- Focused verification passed with `go test ./...`. No frontend build was needed in this step because only backend quality-gate behavior changed.

## Findings Alignment Follow-up 2 2026-04-19

- [x] Add a deterministic low-burstiness check so the quality gate can reject overly uniform sentence rhythm
- [x] Route low-burstiness failures through the existing strict-retry recovery path
- [x] Add quality-gate tests for burstiness regression and healthy variation
- [x] Re-run focused verification and record review notes

Review notes:
- The new burstiness check is intentionally lightweight and deterministic. It measures sentence-length variance from end-of-sentence punctuation only and flags outputs that become unnaturally uniform either in absolute terms or relative to the source text.
- To avoid noisy failures on short chunks, the absolute branch only triggers on outputs with at least four sentences; shorter cases mainly rely on relative regression against the input rhythm.
- The implementation also fixed an English sentence-boundary blind spot by including `.` in the splitter. Without that, English chunks were incorrectly treated as single-sentence text and no rhythm analysis was possible.
- Verification passed with `go test ./internal/workflow/nodes ./internal/agent` and `go test ./...`.

## Findings Alignment Follow-up 3 2026-04-19

- [x] Add a deterministic terminology-drift check for protected technical terms, acronyms, identifiers, and path-like tokens
- [x] Route terminology-drift failures through the existing strict-retry recovery path
- [x] Add quality-gate tests for terminology drift pass/fail cases
- [x] Re-run focused verification and record review notes

Review notes:
- The new terminology check is deliberately heuristic and local. It protects backticked code spans, acronym-style tokens, mixed-case names like `OpenAI` / `PostgreSQL`, identifier-style tokens such as `bge-m3`, and path-like strings such as `internal/workflow/nodes/quality_gate.go`.
- To keep false positives down, the word-level extractor avoids treating generic sentence-initial English capitalization as protected terminology. This means the check is strongest on technical tokens and proper noun style terms that are least safe to paraphrase.
- Recovery integration stayed minimal: terminology drift now flows through the existing strict-retry path, and the supervisor prompt explicitly classifies changed technical terms, acronyms, identifiers, model names, and file paths as retry cases.
- Verification passed with `go test ./...`.

## Findings Alignment Follow-up 4 2026-04-19

- [x] Add a deterministic named-entity fidelity check for institution, product, and proper-name phrases
- [x] Route named-entity failures through the existing strict-retry recovery path
- [x] Add quality-gate tests for named-entity drift pass/fail cases
- [x] Re-run focused verification and record review notes

Review notes:
- This named-entity gate is intentionally narrower than a full NER model. It focuses on the safest high-value cases for deterministic checking: multi-token English proper-name phrases such as `New York University` / `OpenAI Responses API`, plus Chinese institution-style names with stable suffixes such as `清华大学` and `中国科学院`.
- The extractor normalizes harmless English article variation like `The United Nations Environment Programme` so the gate does not reject purely stylistic article insertion, but it still flags real entity replacement or deletion.
- Recovery integration stayed aligned with the earlier findings increments: named-entity drift now reuses the existing strict-retry path, and the supervisor prompt explicitly treats people, institution, product, and proper-noun drift as retry cases.
- Verification passed with `go test ./internal/workflow/nodes ./internal/agent` and `go test ./...`.

## Findings Full Implementation 2026-04-20

- [x] Add a composite scorer layer with calibrated total score, multi-signal breakdown, and sentence-level scoring
- [x] Upgrade coordinator flow to sentence-level Best-of-N selection with candidate persistence
- [x] Persist chunk score/output score/sentence decisions in manifest JSON without a new SQL migration
- [x] Replace fixed round-count behavior with score-aware early-stop and backend-driven optional second-pass continuation
- [x] Expand diff/state/API/frontend contracts to expose calibrated scores, signals, and candidate decisions
- [x] Re-run `gofmt`, `go test ./...`, and `npm run build`, then record review notes

Review notes:
- This implementation intentionally uses a repository-local scorer package rather than introducing heavyweight model-serving infrastructure in one jump. The new scorer exposes the shape required by `docs/03-findings`: calibrated total score, multi-signal breakdown, sentence-level ranking, and a detector-like reference signal, while still remaining deterministic enough for current CI and local development.
- Coordinator behavior is now materially different from the earlier chunk-only path. It performs sentence-level ranking, generates multiple candidate rewrites through lexical/syntax/hybrid paths, computes a unified loss, and persists candidate/decision evidence back into manifest JSON so the product can explain why a sentence changed or stayed unchanged.
- To avoid a schema migration, new scoring and decision data ride inside the existing manifest JSON. That keeps storage compatibility simple and lets history/diff/state APIs expose richer data immediately, but it also means round-level analytics still aggregate from stored chunk JSON rather than from dedicated relational tables.
- Round continuation is now score-aware in a durable way: the backend recomputes effective round capability from completed outputs, exposes `totalRounds / nextRound / canStartNextRound` in session state, and the frontend now follows that backend truth instead of deriving rounds from `promptProfile`.
- The lightweight scorer had one real issue during this pass: its `ngram` signal was incorrectly built from the full forbidden-phrase catalog instead of the matched feature set. That bug kept baseline scores artificially high and made early-stop hard to trigger. The signal now only reflects actual matched features, which makes the current threshold-driven loop operational.
- Verification passed with `go test ./...` and `npm run build`. The frontend build still emits the existing large-chunk warning; no new build failures were introduced.

## Findings Remaining Gaps 2026-04-20

- [x] Persist external-detector calibration state under `var/data/calibration/` and replace the placeholder offline calibrator
- [x] Add real semantic similarity validation via OpenAI-compatible embeddings, with semantic-anchor fallback when embeddings are unavailable
- [x] Add a real readability metric gate instead of only the current lightweight scorer-side penalty
- [x] Expand the review UI from score tags into a fuller multi-signal visualization with calibration metadata, signal breakdown, and candidate details

Review notes:
- The placeholder calibrator has been replaced with a persisted rolling regression state under `var/data/calibration/`, using live detector observations when a detector provider is configured and falling back cleanly when it is not.
- Semantic validation now prefers a real OpenAI-compatible `/v1/embeddings` cosine-similarity check. The older semantic-anchor logic remains as the explicit degradation path rather than the primary validator.
- Readability is now enforced twice on purpose: the coordinator uses the same scorer-side metrics for candidate ranking, and the quality gate independently rejects outputs whose sentence rhythm and density regress past the configured threshold.
- The review UI now exposes detector/calibration metadata, thresholds, signal breakdowns, failure checks, and sentence-level Best-of-N candidate decisions, so the surface matches the richer manifest/API data.
- Verification passed with `go test ./...` and `npm run build`. The frontend build still emits the existing large-chunk warning; no new build failures were introduced.

## Findings Alignment Follow-up 5 2026-04-19

- [x] Inject explicit target heuristic-risk score and forbidden phrase lists into worker guidance
- [x] Keep the forbidden phrase list sourced from the same heuristic rule tables used by `airate`
- [x] Add prompt-level tests proving lexical worker requests carry the new guidance
- [x] Re-run focused verification and record review notes

Review notes:
- Worker guidance now carries two additional control signals that were previously only implied: a derived target heuristic-risk score and an explicit forbidden phrase list. This makes the lexical/syntax workers act more like target-driven rewriters instead of only receiving general round intent.
- The forbidden phrase list is intentionally sourced from the same `aiStrongPatterns`, `aiTransitionPhrases`, and `aiAbstractTerms` tables used by `EstimateAIRate`, so the negative lexicon seen by the workers stays aligned with the heuristic detector instead of drifting into a separately maintained prompt-only list.
- The current target score is still a local heuristic derived from the current risk score, not a calibrated scorer. That is an intentional intermediate step: it sharpens worker behavior now without forcing the larger scorer/calibrator architecture jump from `docs/03-findings/01-claude-ai-rate-solution.md`.
- Verification passed with `go test ./internal/agent ./internal/domain` and `go test ./...`.

## Findings Alignment Follow-up 6 2026-04-19

- [x] Add a lightweight semantic-anchor fidelity gate as a conservative proxy before a future SimCSE layer
- [x] Route semantic-anchor drift through the existing strict-retry recovery path
- [x] Add quality-gate tests for semantic-anchor pass/fail cases, including Chinese fallback coverage
- [x] Re-run focused verification and record review notes

Review notes:
- This new gate is intentionally framed as a proxy, not as “semantic similarity solved.” For English text it uses conservative content-word anchor overlap; for mixed/Chinese text it falls back to normalized character-trigram containment so the repository gets a low-cost catastrophic-drift detector without introducing embedding models yet.
- The thresholds are deliberately set low and the anchor gate only activates when the input contains enough content anchors. That keeps it focused on obvious topic drift rather than punishing normal paraphrase, while still catching cases where the rewritten output keeps facts intact but wanders onto a different subject.
- Recovery integration again stayed minimal: semantic-anchor drift now reuses the strict-retry path, and the supervisor prompt explicitly classifies collapsed topic-anchor overlap as a retry case.
- Verification passed with `go test ./internal/workflow/nodes ./internal/agent` and `go test ./...`.

## Findings Alignment Follow-up 7 2026-04-20

- [x] Add a deterministic structure-preservation gate for numbering, bullet lists, and explicit heading markers
- [x] Route structure-break failures through the existing strict-retry recovery path
- [x] Add quality-gate tests for structure drift pass/fail cases
- [x] Re-run focused verification and record review notes

Review notes:
- This new gate is deliberately narrower than a full document-layout diff. It only activates when the source chunk already contains explicit structural signals worth preserving: numbered items, bullet items, or standalone heading-style lines.
- The comparison is marker-based rather than text-based. It checks the ordered sequence of structure signatures, which keeps it strict on broken numbering and list collapse while still allowing ordinary sentence-level paraphrase inside each item.
- Recovery integration remained minimal and architecture-compatible: structure-break failures now reuse the existing strict-retry tool, and both the recovery agent instructions and the shared output contract explicitly remind the model to preserve numbering and list structure.
- Verification passed with `go test ./internal/workflow/nodes ./internal/agent` and `go test ./...`.

## Findings Docs Closure 2026-04-20

- [x] Add external detector result caching so findings docs no longer over-promise controlled detector usage
- [x] Add a four-signal radar view to the review panel so the UI matches the findings analysis surface
- [x] Rewrite `docs/03-findings/00-ai-rate-two-pass-issues.md` as a current-state audit instead of a stale problem list
- [x] Rewrite `docs/03-findings/01-claude-ai-rate-solution.md` as implemented architecture plus explicit remaining gaps
- [x] Mark `docs/03-findings/02-statistical-methods.md` as a method library rather than the literal file map
- [x] Re-run `gofmt`, `go test ./...`, and `npm run build`, then record review notes

Review notes:
- The remaining code-side mismatch in `docs/03-findings` was concentrated in two places: detector caching and the review-panel visualization. Both are now real code paths rather than aspirational bullets in the docs.
- External detector calls now go through a 24-hour in-memory cache keyed by normalized text hash. This keeps the reference-detector path practical for repeated review and calibration flows without changing the fallback behavior when no detector is configured.
- The review panel now includes a lightweight inline SVG radar for the four scoring signals, while retaining the existing progress bars and candidate detail panels. This closes the biggest UI gap between the findings write-up and the actual product surface without adding a chart dependency.
- `docs/03-findings/00` and `01` were rewritten to describe current truth rather than historical intent, and `02` now explicitly states that its file-level code blocks are illustrative method sketches, not the literal repo file map.
- Verification passed with `gofmt -w`, `go test ./...`, `npm run build` in `web/`, and `git diff --check`.

## Business Completion Retry 2026-04-28

- [x] Restore clean backend/frontend verification gates after accumulated local changes
- [x] Fix React 19 lint blockers in `use-agents` and `use-session`
- [x] Make the Docker demo host port configurable so local services already using `8080` do not block browser smoke runs
- [x] Make unconfigured coordinator agents fall back to the primary rewrite provider instead of completing locally with unchanged text
- [x] Render preview paragraphs as semantic `<p>` elements so the browser path can assert visible document content
- [x] Update the repository mock OpenAI service so `testdata/sample.txt` produces deterministic, quality-gate-safe rewrites
- [x] Re-run API-level and browser-level business smoke tests against the Docker mock stack

Review notes:
- Root cause 1: browser smoke originally failed before app startup because `8080` was already occupied by another local container. `deploy/docker-compose.yml` now supports `NATURALIZE_HTTP_HOST_PORT`, while preserving `8080` as the default.
- Root cause 2: the new coordinator path returned `agent-coordinator-local` when no remote coordinator model was configured, which bypassed the mock rewrite provider and allowed unchanged output to look completed. It now returns `ErrCoordinatorNotConfigured`, allowing the existing provider chain to own the rewrite.
- Root cause 3: the mock provider did not rewrite the repository sample text, so the review tab had no changed cards. The mock now rewrites the sample while preserving facts, terms, and structure, and the API smoke confirmed `changed=2`, `provider=gpt-4.1-mini`, and `tokens=152`.
- Verification passed with `go test ./... -count=1`, `go vet ./...`, `cd web && npm run lint`, `cd web && npm run build`, API smoke against `http://127.0.0.1:19080`, and `PLAYWRIGHT_BASE_URL=http://127.0.0.1:19080 npm run smoke:browser`.
