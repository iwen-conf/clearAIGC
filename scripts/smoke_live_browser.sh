#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
WEB_DIR="$ROOT_DIR/web"
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COMPOSE_PROJECT="${COMPOSE_PROJECT_NAME:-naturalize-live-browser-smoke}"

if [[ -z "${NATURALIZE_PROVIDERS_REWRITE_API_KEY:-}" ]]; then
  echo "NATURALIZE_PROVIDERS_REWRITE_API_KEY is required." >&2
  exit 1
fi

if [[ -z "${NATURALIZE_PROVIDERS_REWRITE_MODEL:-}" ]]; then
  echo "NATURALIZE_PROVIDERS_REWRITE_MODEL is required." >&2
  exit 1
fi

export NATURALIZE_PROVIDERS_REWRITE_BASE_URL="${NATURALIZE_PROVIDERS_REWRITE_BASE_URL:-https://api.openai.com}"
export NATURALIZE_PROVIDERS_FALLBACK_BASE_URL="${NATURALIZE_PROVIDERS_FALLBACK_BASE_URL:-$NATURALIZE_PROVIDERS_REWRITE_BASE_URL}"
export NATURALIZE_PROVIDERS_AGENT_BASE_URL="${NATURALIZE_PROVIDERS_AGENT_BASE_URL:-$NATURALIZE_PROVIDERS_REWRITE_BASE_URL}"
export NATURALIZE_PROVIDERS_FALLBACK_API_KEY="${NATURALIZE_PROVIDERS_FALLBACK_API_KEY:-$NATURALIZE_PROVIDERS_REWRITE_API_KEY}"
export NATURALIZE_PROVIDERS_AGENT_API_KEY="${NATURALIZE_PROVIDERS_AGENT_API_KEY:-$NATURALIZE_PROVIDERS_REWRITE_API_KEY}"
export NATURALIZE_PROVIDERS_FALLBACK_MODEL="${NATURALIZE_PROVIDERS_FALLBACK_MODEL:-$NATURALIZE_PROVIDERS_REWRITE_MODEL}"
export NATURALIZE_PROVIDERS_AGENT_MODEL="${NATURALIZE_PROVIDERS_AGENT_MODEL:-$NATURALIZE_PROVIDERS_REWRITE_MODEL}"

cleanup() {
  if [[ "${KEEP_STACK:-0}" == "1" ]]; then
    echo "KEEP_STACK=1 is set; leaving the Docker stack running."
    return
  fi
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" down --remove-orphans >/dev/null 2>&1 || true
}

wait_for_app() {
  echo "Waiting for application health..."
  for _ in {1..90}; do
    if curl -fsS "$BASE_URL/api/v1/health/live" >/dev/null 2>&1 && curl -fsS "$BASE_URL/" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "Application did not become ready at $BASE_URL" >&2
  return 1
}

ensure_browser_deps() {
  if [[ ! -d "$WEB_DIR/node_modules" ]]; then
    echo "Installing frontend dependencies..."
    (cd "$WEB_DIR" && npm install)
  fi

  echo "Ensuring Playwright Chromium is installed..."
  (cd "$WEB_DIR" && npm run e2e:install >/dev/null)
}

trap cleanup EXIT

ensure_browser_deps

echo "Starting live-provider smoke stack..."
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" up -d --build

wait_for_app

echo "Running browser smoke test against live provider settings..."
(cd "$WEB_DIR" && PLAYWRIGHT_BASE_URL="$BASE_URL" npm run smoke:browser)

echo "Live browser smoke test passed against $BASE_URL"
