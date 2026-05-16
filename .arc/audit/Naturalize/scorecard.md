# Scorecard

Scores are bounded judgments from direct local evidence. `N/A` means the required evidence was not available in this audit surface.

| Dimension | Score | Confidence | Evidence Boundary |
|---|---:|---|---|
| Architecture and longevity | 7.0 / 10 | Medium | Layered backend packages, workflow separation, storage layout, and docs are observable. Long-running workflow behavior was not smoke-tested here. |
| Security posture and access control | 3.0 / 10 | High | Public routes, wildcard CORS default, and plaintext secret persistence are directly observable. |
| Code quality and engineering discipline | 6.5 / 10 | High | `go test` and `go vet` pass; `staticcheck` and frontend lint fail. |
| Business value and flow observability | 6.0 / 10 | Medium | README and Playwright smoke cover the core upload-processing-preview-download flow. No production usage or quality outcome telemetry was inspected. |
| Observability and delivery operations | 5.5 / 10 | Medium | Health endpoints and Docker demo exist; LLM readiness, app healthcheck, and container hardening are partial. |
| Team collaboration and knowledge flow | 5.0 / 10 | Low | README and architecture docs exist; no CI workflow files were found in `.github`. PR/release process evidence was not available. |
| Technical debt and dependency risk | 6.0 / 10 | Medium | npm audit is clean and no risky licenses were found, but Go vulnerability scan is missing and 5 Go licenses are unknown. |

## Specialist Indices

### Business Maturity Index

Score: N/A

- missing_reason: The core user flow is observable, but operating artifacts such as usage metrics, support process, SLA/SLOs, user adoption, or production incident records were not present.
- missing_evidence_type: production metrics, user workflow analytics, incident logs, release notes, support records.
- how_to_collect_more_evidence: add telemetry for upload success rate, processing latency, model failure rate, export success rate, manual recovery rate, and user review decisions.

### Dependency Health Score

Score: N/A

- missing_reason: npm audit and local license scanning were available, but Go vulnerability scanning was unavailable because `govulncheck` is not installed.
- missing_evidence_type: Go vulnerability scan output, maintained SCA report, automated license allow-list report.
- how_to_collect_more_evidence: add `govulncheck ./...`, `npm audit --audit-level=moderate`, and a license scanner such as `go-licenses` or `licensee` to CI.
