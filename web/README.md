# Naturalize Web

This directory contains the React + Vite frontend for the customer-facing Naturalize workflow.

## Commands

```bash
npm run dev
npm run lint
npm run build
```

For local API development, set `VITE_API_PROXY_TARGET` explicitly if you want a specific backend. If it is unset, Vite auto-detects `http://127.0.0.1:18081` first and then falls back to `http://127.0.0.1:8080`.

## Browser smoke test

Install the browser once per machine:

```bash
npm run e2e:install
```

Run the Playwright smoke test against a running Naturalize server:

```bash
PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 npm run smoke:browser
```

From the repository root, the full Docker-backed browser smoke path is:

```bash
bash scripts/smoke_mock_browser.sh
```
