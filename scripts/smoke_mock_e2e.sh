#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
SAMPLE_FILE="$ROOT_DIR/testdata/sample.txt"
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"

cleanup() {
  docker compose -f "$COMPOSE_FILE" down --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo "Starting local stack..."
docker compose -f "$COMPOSE_FILE" up -d --build

echo "Waiting for API health..."
for _ in {1..60}; do
  if curl -fsS "$BASE_URL/api/v1/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

curl -fsS "$BASE_URL/api/v1/health/live" >/dev/null

echo "Creating session..."
CREATE_RESPONSE="$(curl -fsS -X POST \
  -F "file=@$SAMPLE_FILE" \
  -F "promptProfile=en" \
  "$BASE_URL/api/v1/sessions")"

SESSION_ID="$(printf '%s' "$CREATE_RESPONSE" | node -e "const data = JSON.parse(require('fs').readFileSync(0, 'utf8')); process.stdout.write(data.id)")"
if [[ -z "$SESSION_ID" ]]; then
  echo "Failed to extract session id" >&2
  exit 1
fi

echo "Starting round for session $SESSION_ID..."
curl -fsS -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/sessions/$SESSION_ID/start" >/dev/null

echo "Waiting for completion..."
FINAL_STATUS=""
for _ in {1..60}; do
  SESSION_JSON="$(curl -fsS "$BASE_URL/api/v1/sessions/$SESSION_ID")"
  FINAL_STATUS="$(printf '%s' "$SESSION_JSON" | node -e "const data = JSON.parse(require('fs').readFileSync(0, 'utf8')); process.stdout.write(data.status)")"
  if [[ "$FINAL_STATUS" == "completed" ]]; then
    break
  fi
  if [[ "$FINAL_STATUS" == "failed" ]]; then
    echo "Session failed" >&2
    exit 1
  fi
  sleep 2
done

if [[ "$FINAL_STATUS" != "completed" ]]; then
  echo "Timed out waiting for session completion, final status: $FINAL_STATUS" >&2
  exit 1
fi

echo "Fetching output..."
OUTPUT_JSON="$(curl -fsS "$BASE_URL/api/v1/sessions/$SESSION_ID/output?round=1")"
OUTPUT_TEXT="$(printf '%s' "$OUTPUT_JSON" | node -e "const data = JSON.parse(require('fs').readFileSync(0, 'utf8')); process.stdout.write(data.text)")"

if [[ -z "$OUTPUT_TEXT" ]]; then
  echo "Output text was empty" >&2
  exit 1
fi

echo "Smoke test passed for session $SESSION_ID"
