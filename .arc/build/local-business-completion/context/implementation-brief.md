# Implementation Brief: Local Business Completion

Date: 2026-04-24
Project: Naturalize

## Goal

Implement the agreed personal-local deployment business requirements:

- external-detector-first quality acceptance, with local fallback marked as unverified;
- broader error scenario behavior surfaced through history;
- local metrics on history detail;
- 30-day configurable local data retention and privacy defaults.

No user accounts, tenants, commercial billing, or SaaS operations are in scope.

## Roles

- Owner: product maintainer defining personal-local behavior.
- Executor: Codex implementation pass.
- Reviewer: verification-oriented pass using tests, build, and lint/static checks where feasible.

## Inputs

- User plan finalized in chat.
- Existing audit artifacts under `.arc/audit/Naturalize/`.
- Current code paths: domain history/scoring types, service history/delete/runRound, config/main, frontend history detail components.

## Recovery Boundary

The repository already has many uncommitted changes. This implementation will only add scoped changes and will not revert existing files. Rollback is via reverting this task's changed hunks/files from the working tree.
