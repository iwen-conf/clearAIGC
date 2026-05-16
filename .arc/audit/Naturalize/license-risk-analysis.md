# License Risk Analysis

This is a local heuristic dependency-license scan, not legal advice. It inspected dependency manifests and local module/package metadata available on 2026-04-24.

## Inputs Inspected

- `go.mod`
- `go.sum`
- `web/package.json`
- `web/package-lock.json`
- Go module cache after `go mod download -json all`

## NPM Result

- Direct npm dependencies: 6 production, 13 development.
- Lockfile packages scanned: 332.
- High-risk licenses detected: 0.
- Unknown license fields: 0.
- `npm audit --json`: 0 vulnerabilities.
- `npm audit --omit=dev --json`: 0 production vulnerabilities.

No npm package in the lockfile matched the high-risk patterns checked: AGPL, GPL, LGPL, Commons Clause, CC-BY-NC, NonCommercial, or BUSL.

## Go Result

- Go module records scanned: 124.
- Permissive licenses detected: 118.
- Weak copyleft detected: 1.
- High-risk licenses detected: 0.
- Unknown licenses: 5.

Weak copyleft:

- `github.com/certifi/gocertifi@v0.0.0-20190105021004-abcd57078448`: MPL.

Unknown license classification:

- `github.com/bmizerany/assert@v0.0.0-20160611221934-b7ed37b82869`
- `github.com/bytedance/mockey@v1.3.0`
- `github.com/cloudwego/eino@v0.8.8`
- `github.com/cloudwego/eino-ext/components/model/openai@v0.1.13`
- `github.com/cloudwego/eino-ext/libs/acl/openai@v0.1.17`

No Go module matched the high-risk patterns checked: AGPL, GPL, LGPL, Commons Clause, CC-BY-NC, NonCommercial, or BUSL.

## Risk Assessment

| Item | Risk | Reason | Mitigation |
|---|---|---|---|
| npm dependencies | Low | Lockfile has explicit licenses and no high-risk patterns. | Keep `npm audit` and license allow-list in CI. |
| Go permissive dependencies | Low | Majority classified as MIT/BSD/Apache/ISC-style by local LICENSE files. | Add a formal Go license report in CI. |
| `github.com/certifi/gocertifi` MPL | Medium-low | MPL is weak copyleft and usually file-level, but still needs compliance notice review. | Preserve notices and include MPL in third-party notices. |
| 5 unknown Go modules | Medium | Local heuristic could not classify them; some are core framework dependencies. | Resolve from upstream metadata or a formal scanner before commercial release. |
| Go vulnerabilities | N/A | `govulncheck` was not installed, so no Go vulnerability report was produced. | Run `govulncheck ./...` in CI and before release. |

## Recommended Gate

Add CI jobs for:

```bash
go test ./...
go vet ./...
govulncheck ./...
cd web && npm audit --audit-level=moderate
```

Add a dependency notice generation step using a formal scanner and fail the gate on AGPL, GPL, non-commercial, Commons Clause, BUSL, or unknown licenses unless explicitly approved.
