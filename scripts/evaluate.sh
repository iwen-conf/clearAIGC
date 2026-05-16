#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
DATASET_FILE="${DATASET_FILE:-$ROOT_DIR/testdata/eval/dataset.100.jsonl}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/var/data/reports}"
WAIT_SECONDS="${WAIT_SECONDS:-120}"
START_STACK="${START_STACK:-1}"
KEEP_STACK="${KEEP_STACK:-0}"
PROMPT_PROFILE_OVERRIDE="${PROMPT_PROFILE:-}"
MAX_SAMPLES="${MAX_SAMPLES:-0}"

usage() {
  cat <<USAGE
Usage: scripts/evaluate.sh [options]

Options:
  --dataset <path>         JSONL dataset file (default: testdata/eval/dataset.100.jsonl)
  --base-url <url>         API base URL (default: http://127.0.0.1:8080)
  --out-dir <path>         Report output directory (default: var/data/reports)
  --wait-seconds <n>       Wait timeout per sample (default: 120)
  --profile <name>         Override prompt profile (cn|cn_single|en)
  --max-samples <n>        Evaluate first N samples only (default: all)
  --no-start               Do not start docker compose stack
  --keep-stack             Keep stack running after script exits
  -h, --help               Show this help

Dataset line schema (JSONL):
  {"id":"...","profile":"cn|en|cn_single","source":"ai|human","domain":"...","text":"..."}
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dataset)
      DATASET_FILE="$2"; shift 2 ;;
    --base-url)
      BASE_URL="$2"; shift 2 ;;
    --out-dir)
      OUT_DIR="$2"; shift 2 ;;
    --wait-seconds)
      WAIT_SECONDS="$2"; shift 2 ;;
    --profile)
      PROMPT_PROFILE_OVERRIDE="$2"; shift 2 ;;
    --max-samples)
      MAX_SAMPLES="$2"; shift 2 ;;
    --no-start)
      START_STACK=0; shift ;;
    --keep-stack)
      KEEP_STACK=1; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1 ;;
  esac
done

if [[ ! -f "$DATASET_FILE" ]]; then
  echo "Dataset not found: $DATASET_FILE" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
TMP_DIR="$(mktemp -d)"
RESULTS_JSON="$TMP_DIR/results.json"
SUMMARY_TXT="$TMP_DIR/summary.txt"
RUN_TS="$(date +%Y%m%d-%H%M%S)"
RUN_DATE="$(date +%F)"
REPORT_FILE="$OUT_DIR/baseline-$RUN_DATE.md"
METRICS_FILE="$OUT_DIR/metrics-$RUN_TS.json"

cleanup() {
  if [[ "$START_STACK" == "1" && "$KEEP_STACK" != "1" ]]; then
    docker compose -f "$COMPOSE_FILE" down --remove-orphans >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if [[ "$START_STACK" == "1" ]]; then
  echo "[evaluate] starting local stack"
  docker compose -f "$COMPOSE_FILE" up -d --build
fi

echo "[evaluate] waiting for API health: $BASE_URL"
for _ in {1..90}; do
  if curl -fsS "$BASE_URL/api/v1/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
curl -fsS "$BASE_URL/api/v1/health/live" >/dev/null

echo "[evaluate] running dataset: $DATASET_FILE"
node - "$BASE_URL" "$DATASET_FILE" "$RESULTS_JSON" "$WAIT_SECONDS" "$PROMPT_PROFILE_OVERRIDE" "$MAX_SAMPLES" <<'NODE'
const fs = require('fs')

const baseUrl = process.argv[2]
const datasetFile = process.argv[3]
const resultsFile = process.argv[4]
const waitSeconds = Number(process.argv[5] || '120')
const profileOverride = (process.argv[6] || '').trim()
const maxSamples = Number(process.argv[7] || '0')

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function request(url, options = {}) {
  const res = await fetch(url, options)
  const text = await res.text()
  let json = null
  if (text) {
    try { json = JSON.parse(text) } catch {}
  }
  if (!res.ok) {
    const body = text ? text.slice(0, 400) : ''
    throw new Error(`HTTP ${res.status} ${url} ${body}`)
  }
  return json
}

function normalizeProfile(v) {
  const p = String(v || '').trim()
  if (p === 'cn' || p === 'cn_single' || p === 'en') return p
  return 'cn'
}

function extractRoundScores(history) {
  const rounds = Array.isArray(history?.rounds) ? history.rounds : []
  if (rounds.length === 0) return { scoreBefore: null, scoreAfter: null, detector: '', roundsUsed: 0 }
  const last = rounds[rounds.length - 1]
  const q = last?.summary?.qualityVerdict || {}
  return {
    scoreBefore: typeof q.scoreBefore === 'number' ? q.scoreBefore : null,
    scoreAfter: typeof q.scoreAfter === 'number' ? q.scoreAfter : null,
    detector: String(q.detector || '').trim(),
    roundsUsed: rounds.length,
  }
}

function mkFormData(fields) {
  const boundary = `----naturalize-eval-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  const chunks = []
  for (const f of fields) {
    chunks.push(`--${boundary}\r\n`)
    if (f.fileName) {
      chunks.push(`Content-Disposition: form-data; name="${f.name}"; filename="${f.fileName}"\r\n`)
      chunks.push(`Content-Type: ${f.contentType || 'text/plain'}\r\n\r\n`)
      chunks.push(f.value)
      chunks.push(`\r\n`)
    } else {
      chunks.push(`Content-Disposition: form-data; name="${f.name}"\r\n\r\n`)
      chunks.push(String(f.value))
      chunks.push(`\r\n`)
    }
  }
  chunks.push(`--${boundary}--\r\n`)
  const body = chunks.join('')
  return { body, contentType: `multipart/form-data; boundary=${boundary}` }
}

async function startNextRoundIfPossible(sessionId) {
  try {
    await request(`${baseUrl}/api/v1/sessions/${sessionId}/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    })
    return true
  } catch {
    return false
  }
}

async function runSample(sample, index) {
  const profile = normalizeProfile(profileOverride || sample.profile)
  const text = String(sample.text || '').trim()
  if (!text) throw new Error('empty text')

  const fileName = `${sample.id || `sample-${index + 1}`}.txt`
  const fd = mkFormData([
    { name: 'promptProfile', value: profile },
    { name: 'file', fileName, value: text, contentType: 'text/plain; charset=utf-8' },
  ])

  const created = await request(`${baseUrl}/api/v1/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': fd.contentType },
    body: fd.body,
  })
  const sessionId = created?.id
  if (!sessionId) throw new Error('missing session id')

  await request(`${baseUrl}/api/v1/sessions/${sessionId}/start`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  })

  const startedAt = Date.now()
  const deadlineMs = waitSeconds * 1000
  let startAttempts = 0

  for (;;) {
    await sleep(1500)
    const session = await request(`${baseUrl}/api/v1/sessions/${sessionId}`)
    const status = String(session?.status || '')

    if (status === 'completed') {
      const history = await request(`${baseUrl}/api/v1/sessions/${sessionId}/history`)
      const scores = extractRoundScores(history)
      return {
        id: sample.id || `sample-${index + 1}`,
        source: sample.source || 'unknown',
        domain: sample.domain || 'unknown',
        profile,
        sessionId,
        status,
        scoreBefore: scores.scoreBefore,
        scoreAfter: scores.scoreAfter,
        delta: (typeof scores.scoreBefore === 'number' && typeof scores.scoreAfter === 'number')
          ? (scores.scoreBefore - scores.scoreAfter)
          : null,
        detector: scores.detector,
        roundsUsed: scores.roundsUsed,
        elapsedMs: Date.now() - startedAt,
      }
    }

    if (status === 'pending') {
      const nextRound = Number(session?.nextRound || 0)
      const canStart = Boolean(session?.canStartNextRound)
      if (canStart && nextRound > 0 && startAttempts < 3) {
        const started = await startNextRoundIfPossible(sessionId)
        if (started) {
          startAttempts += 1
          continue
        }
      }
    }

    if (status === 'failed') {
      return {
        id: sample.id || `sample-${index + 1}`,
        source: sample.source || 'unknown',
        domain: sample.domain || 'unknown',
        profile,
        sessionId,
        status,
        scoreBefore: null,
        scoreAfter: null,
        delta: null,
        detector: '',
        roundsUsed: 0,
        elapsedMs: Date.now() - startedAt,
        error: 'session failed',
      }
    }

    if (Date.now() - startedAt > deadlineMs) {
      return {
        id: sample.id || `sample-${index + 1}`,
        source: sample.source || 'unknown',
        domain: sample.domain || 'unknown',
        profile,
        sessionId,
        status: 'timeout',
        scoreBefore: null,
        scoreAfter: null,
        delta: null,
        detector: '',
        roundsUsed: 0,
        elapsedMs: Date.now() - startedAt,
        error: `timeout after ${waitSeconds}s`,
      }
    }
  }
}

function parseDataset(file) {
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/).map((line) => line.trim()).filter(Boolean)
  const rows = []
  for (const line of lines) {
    rows.push(JSON.parse(line))
  }
  if (rows.length === 0) throw new Error('dataset empty')
  return rows
}

function summarize(results) {
  const done = results.filter((r) => r.status === 'completed' && typeof r.scoreBefore === 'number' && typeof r.scoreAfter === 'number')
  const failed = results.length - done.length
  const avg = (arr, key) => arr.length ? arr.reduce((s, x) => s + x[key], 0) / arr.length : null
  const before = avg(done, 'scoreBefore')
  const after = avg(done, 'scoreAfter')
  const delta = (before != null && after != null) ? (before - after) : null

  const grouped = {}
  for (const row of done) {
    const k = row.source || 'unknown'
    grouped[k] ||= []
    grouped[k].push(row)
  }

  const bySource = {}
  for (const [k, arr] of Object.entries(grouped)) {
    const b = avg(arr, 'scoreBefore')
    const a = avg(arr, 'scoreAfter')
    bySource[k] = {
      count: arr.length,
      scoreBefore: b,
      scoreAfter: a,
      delta: (b != null && a != null) ? (b - a) : null,
    }
  }

  const detectorSet = new Set(done.map((r) => r.detector).filter(Boolean))

  return {
    total: results.length,
    completed: done.length,
    failed,
    scoreBefore: before,
    scoreAfter: after,
    delta,
    bySource,
    detectors: Array.from(detectorSet),
  }
}

async function main() {
  const dataset = parseDataset(datasetFile)
  const rows = (maxSamples > 0) ? dataset.slice(0, maxSamples) : dataset
  const results = []

  for (let i = 0; i < rows.length; i++) {
    const row = rows[i]
    const prefix = `[${i + 1}/${rows.length}] ${row.id || 'sample'}`
    try {
      const r = await runSample(row, i)
      results.push(r)
      if (r.status === 'completed') {
        const b = r.scoreBefore == null ? 'n/a' : r.scoreBefore.toFixed(3)
        const a = r.scoreAfter == null ? 'n/a' : r.scoreAfter.toFixed(3)
        const d = r.delta == null ? 'n/a' : r.delta.toFixed(3)
        console.log(`${prefix} ok rounds=${r.roundsUsed} before=${b} after=${a} delta=${d}`)
      } else {
        console.log(`${prefix} ${r.status} ${r.error || ''}`.trim())
      }
    } catch (err) {
      const msg = err && err.message ? err.message : String(err)
      results.push({
        id: row.id || `sample-${i + 1}`,
        source: row.source || 'unknown',
        domain: row.domain || 'unknown',
        profile: normalizeProfile(profileOverride || row.profile),
        status: 'error',
        error: msg,
        scoreBefore: null,
        scoreAfter: null,
        delta: null,
        detector: '',
        roundsUsed: 0,
      })
      console.log(`${prefix} error ${msg}`)
    }
  }

  const summary = summarize(results)
  fs.writeFileSync(resultsFile, JSON.stringify({ summary, results }, null, 2))

  const before = summary.scoreBefore == null ? 'n/a' : summary.scoreBefore.toFixed(6)
  const after = summary.scoreAfter == null ? 'n/a' : summary.scoreAfter.toFixed(6)
  const delta = summary.delta == null ? 'n/a' : summary.delta.toFixed(6)
  console.log(`SUMMARY before=${before} after=${after} delta=${delta} completed=${summary.completed}/${summary.total}`)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
NODE

node - "$RESULTS_JSON" "$SUMMARY_TXT" <<'NODE'
const fs = require('fs')
const file = process.argv[2]
const out = process.argv[3]
const data = JSON.parse(fs.readFileSync(file, 'utf8'))
const s = data.summary || {}

function fmt(v) {
  return typeof v === 'number' ? v.toFixed(6) : 'n/a'
}

const lines = []
lines.push(`score_before_avg=${fmt(s.scoreBefore)}`)
lines.push(`score_after_avg=${fmt(s.scoreAfter)}`)
lines.push(`score_delta_avg=${fmt(s.delta)}`)
lines.push(`completed=${s.completed || 0}`)
lines.push(`total=${s.total || 0}`)
lines.push(`failed=${s.failed || 0}`)
fs.writeFileSync(out, lines.join('\n') + '\n')
console.log(lines.join('\n'))
NODE

SCORE_BEFORE="$(awk -F= '/^score_before_avg=/{print $2}' "$SUMMARY_TXT")"
SCORE_AFTER="$(awk -F= '/^score_after_avg=/{print $2}' "$SUMMARY_TXT")"
SCORE_DELTA="$(awk -F= '/^score_delta_avg=/{print $2}' "$SUMMARY_TXT")"
COMPLETED="$(awk -F= '/^completed=/{print $2}' "$SUMMARY_TXT")"
TOTAL="$(awk -F= '/^total=/{print $2}' "$SUMMARY_TXT")"
FAILED="$(awk -F= '/^failed=/{print $2}' "$SUMMARY_TXT")"

cp "$RESULTS_JSON" "$METRICS_FILE"

LAST_REPORT="$(ls -1t "$OUT_DIR"/baseline-*.md 2>/dev/null | head -n 1 || true)"
PREV_LINE=""
if [[ -n "$LAST_REPORT" && "$LAST_REPORT" != "$REPORT_FILE" ]]; then
  PREV_LINE="- Previous report: $(basename "$LAST_REPORT")"
fi

{
  echo "# Baseline Evaluation Report ($RUN_DATE)"
  echo
  echo "- Dataset: $DATASET_FILE"
  echo "- Base URL: $BASE_URL"
  echo "- Completed samples: $COMPLETED / $TOTAL"
  echo "- Failed samples: $FAILED"
  echo "- Avg score before: $SCORE_BEFORE"
  echo "- Avg score after: $SCORE_AFTER"
  echo "- Avg delta (before-after): $SCORE_DELTA"
  echo "- Raw metrics: $(basename "$METRICS_FILE")"
  if [[ -n "$PREV_LINE" ]]; then
    echo "$PREV_LINE"
  fi
  echo
  echo "## Notes"
  echo
  echo "- This report is generated by scripts/evaluate.sh."
  echo "- A positive delta means the average risk score decreased after rewrite."
  echo "- If detector is not configured, scores still follow the current internal+fallback scorer path."
} > "$REPORT_FILE"

echo "[evaluate] report: $REPORT_FILE"
echo "[evaluate] metrics: $METRICS_FILE"
echo "[evaluate] done"
