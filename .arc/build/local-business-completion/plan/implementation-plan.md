# Implementation Plan: Local Business Completion

## Goals And Scope

- Add decision-complete local quality verdicts to session history.
- Add local run metrics to history responses and UI.
- Add data retention/privacy configuration and cleanup service behavior.
- Improve upload body guard and preserve personal-local deployment constraints.

Out of scope: auth, users, tenants, billing, online price sync, separate analytics dashboard.

## Change Steps

1. Extend domain/config contracts for quality verdicts, local metrics, retention, and privacy.
2. Compute verdicts and metrics from existing rounds/manifests in `ReadHistory`.
3. Implement retention cleanup and wire it from startup without cleaning active processing sessions.
4. Add HTTP upload body limiting before multipart parsing.
5. Update frontend types and history detail UI to show verdicts and local metrics.
6. Add focused backend tests and update docs/config examples.

## Risks And Fallback

- Risk: existing in-flight working tree contains related uncommitted changes. Mitigation: small scoped patches and no reverts.
- Risk: database schema churn. Mitigation: avoid new persistent tables unless necessary; derive metrics from existing data.
- Risk: cleanup deletes data unexpectedly. Mitigation: default only completed/failed/paused older than retention cutoff, never processing.
- Rollback: revert changed hunks from this task; no irreversible migrations are planned.

## Verification Plan

- `go test ./...`
- `go vet ./...`
- `staticcheck ./...` if available; record any pre-existing failures separately.
- `cd web && npm run build`
- `cd web && npm run lint`; record any pre-existing lint failures separately if not fully addressed.
