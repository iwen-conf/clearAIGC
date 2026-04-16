# clearAIGC — Project Tracker

## Repository Bootstrap

- [x] Review workspace and remote repository state
- [x] Define `.gitignore` rules for local tools, build artifacts, and document outputs
- [x] Remove legacy nested repository content (`baibaiAIGC/`)
- [ ] Initialize Git, configure GitHub remote, and create the initial commit
- [ ] Push the initial branch and verify upstream tracking

## Architecture & Documentation

- [x] Analyze existing pipeline, state persistence, and resume logic
- [x] Validate Eino workflow and ReAct orchestration capabilities
- [x] Produce rewrite architecture: deterministic workflow + bounded agent reasoning
- [x] Define numbered documentation taxonomy and stable reading order
- [x] Deliver architecture documents (master-plan, vision, modules, workflow, infra, api, observability, security)
- [x] Validate docs tree and repair all cross-references

## Implementation

- [ ] Scaffold Go project structure (`cmd/server`, `internal/`, `pkg/`, `deploy/`)
- [ ] Implement document intake, 4-level chunking, and Manifest persistence
- [ ] Implement Eino Workflow orchestration: round execution, BatchNode, QualityGate, export
- [ ] Add bounded ReAct Agent for quality recovery (retry_strict, split_rewrite, accept)
- [ ] Implement Redis CheckPointStore and Pub/Sub progress streaming
- [ ] Implement REST API layer (Gin + SSE + health check)
- [ ] Migrate React UI to new Go API backend
- [ ] End-to-end validation with `.txt` and `.docx` samples

## Review

- The most reusable assets are: chunking rules, round-state derivation, prompt assets, resume semantics, and export behavior.
- The most replaceable parts are: Python transport layers, ad hoc JSON file storage, direct LLM calls inside round execution, and the legacy desktop bridge.
- Target shape: deterministic Workflow as the main execution path, with ReAct limited to exception handling, tool selection, validation, and adaptive retry.
- The `docs/` tree has a stable numbered reading order; all markdown relative links resolve correctly.
