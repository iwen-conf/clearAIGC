# Naturalize

Naturalize is a Go + React application for refining AI-written academic and technical drafts into more natural writing while preserving structure, terminology, and meaning.

## What is in the repository

- Go backend with resumable rewrite orchestration, quality checks, recovery flow, SSE progress, and TXT/DOCX export
- React frontend for upload, progress tracking, pause/resume, preview, section-by-section comparison, and download
- Docker-based local demo path that works without real model credentials by using a mock OpenAI-compatible service

## Quick start

### Local demo mode

This path builds the frontend, starts PostgreSQL and Redis, and uses the bundled mock model service.

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

Open `http://127.0.0.1:8080`.

### Local API smoke test

This script boots the local stack, uploads `testdata/sample.txt`, starts processing, waits for completion, and verifies that output is returned.

```bash
bash scripts/smoke_mock_e2e.sh
```

### Local browser smoke test

This path drives the customer-facing UI with Playwright against the Docker demo stack: upload, processing, preview, section review, and TXT download.

```bash
bash scripts/smoke_mock_browser.sh
```

### Run the backend directly

Copy the example environment file and update values if you want to point at real providers.

```bash
cp .env.example .env
go run ./cmd/server
```

### Frontend development

```bash
cd web
npm install
npm run dev
```

The Vite dev server honors `VITE_API_PROXY_TARGET` when set. Otherwise it auto-detects a local backend by checking `http://127.0.0.1:18081` first and then `http://127.0.0.1:8080`.

## Real provider mode

Override the provider environment variables in `.env` or your shell:

- `NATURALIZE_PROVIDERS_REWRITE_BASE_URL`
- `NATURALIZE_PROVIDERS_REWRITE_API_KEY`
- `NATURALIZE_PROVIDERS_REWRITE_MODEL`
- `NATURALIZE_PROVIDERS_FALLBACK_BASE_URL`
- `NATURALIZE_PROVIDERS_FALLBACK_API_KEY`
- `NATURALIZE_PROVIDERS_FALLBACK_MODEL`
- `NATURALIZE_PROVIDERS_AGENT_BASE_URL`
- `NATURALIZE_PROVIDERS_AGENT_API_KEY`
- `NATURALIZE_PROVIDERS_AGENT_MODEL`

If you switch to real providers, the backend will use:

- Responses API for the primary rewrite path
- Chat Completions for fallback rewrite calls
- ChatModel tool-calling for the recovery agent

To exercise the same browser flow against live credentials, export at least `NATURALIZE_PROVIDERS_REWRITE_API_KEY` and `NATURALIZE_PROVIDERS_REWRITE_MODEL`, then run:

```bash
bash scripts/smoke_live_browser.sh
```

## Verification

Backend:

```bash
go test ./...
go build ./...
```

Frontend:

```bash
cd web
npm run lint
npm run build
npm run smoke:browser
```
