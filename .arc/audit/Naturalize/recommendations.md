# Recommendations

## P0 - Add an authentication and authorization boundary

Expected value: prevents unauthenticated mutation, export, provider reconfiguration, and secret writes.
Delivery cost: medium.
Migration risk: medium, because frontend and smoke scripts need credentials.

Remediation:
- Add auth middleware before protected `/api/v1` routes.
- Keep `/health/live` public if needed; require auth for sessions, exports, cards, rounds, and agents.
- Restrict CORS to configured trusted origins. Do not default to `*` outside demo mode.
- Store provider API keys encrypted at rest or move them to a secret manager.

Dispatch command:

```bash
arc build --project-path /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize --focus security --scope "protect /api/v1 session/export/card/round/agents routes with auth middleware, restrict CORS defaults, encrypt or externalize agent_settings.api_key, update frontend API client and tests"
```

## P0 - Enforce request body limits before multipart parsing

Expected value: closes upload resource-exhaustion path.
Delivery cost: low.
Migration risk: low.

Quick fix direction:

```go
const maxUploadBytes = 50 << 20

func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
```

Apply this before upload routes, and set `router.MaxMultipartMemory` to a small bounded value. Batch uploads should enforce a total request cap as well as per-file caps.

Dispatch command:

```bash
arc fix --project-path /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize --focus upload-dos --scope "add MaxBytesReader and MaxMultipartMemory guards for single and batch upload, return 413 or INVALID_REQUEST, add handler tests"
```

## P1 - Restore frontend lint gate

Expected value: makes CI-style frontend checks enforceable and removes React 19 hooks warnings.
Delivery cost: low.
Migration risk: low.

Quick fix direction:

```bash
cd /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize/web
npm run lint
```

Then refactor:
- `web/src/hooks/use-agents.ts`: avoid invoking a callback that synchronously sets state from the effect body in the flagged pattern.
- `web/src/hooks/use-session.ts`: move `refreshRef.current = refreshSession` into an effect or remove the ref if it is not needed; avoid synchronous `setBooting(false)` in the effect body.

Dispatch command:

```bash
arc fix --project-path /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize --focus frontend-lint --scope "fix react-hooks lint errors in web/src/hooks/use-agents.ts and web/src/hooks/use-session.ts, keep npm run build green"
```

## P2 - Make `staticcheck` clean

Expected value: keeps Go quality tooling consistent with the currently passing `go test` and `go vet`.
Delivery cost: low.
Migration risk: very low.

Quick fix:

```bash
cd /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize
perl -0pi -e 's/r\\.lastAt\\.Add\\(r\\.interval\\)\\.Sub\\(time\\.Now\\(\\)\\)/time.Until(r.lastAt.Add(r.interval))/g' internal/domain/scorer/requester.go
perl -0pi -e 's/\\nfunc readabilityPenalty\\(text string\\) float64 \\{\\n\\treturn EvaluateReadability\\(text\\)\\.Penalty\\n\\}\\n//s' internal/domain/scorer/signals.go
staticcheck ./...
go test ./...
```

## P2 - Improve readiness and deployment hardening

Expected value: avoids false healthy signals and reduces container/runtime exposure.
Delivery cost: medium.
Migration risk: low to medium.

Remediation:
- Add optional provider readiness probes or report provider state as `unknown`, `configured`, `healthy`, or `unhealthy`.
- Add app healthcheck to Docker Compose.
- Use Docker secrets or environment-specific overrides for real deployments.
- Add non-root user in the final Docker image and set ownership for writable paths.

Dispatch command:

```bash
arc build --project-path /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize --focus operations --scope "add provider-aware health/readiness, app docker healthcheck, non-root Docker runtime user, production-safe compose override guidance"
```

## P2 - Add dependency and license gates to CI

Expected value: closes the remaining vulnerability and compliance evidence gaps.
Delivery cost: low to medium.
Migration risk: low.

Quick fix direction:

```bash
cd /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
cd web
npm audit --audit-level=moderate
```

Then resolve or document the 5 Go modules with unknown licenses from the local heuristic scan:
- `github.com/bmizerany/assert`
- `github.com/bytedance/mockey`
- `github.com/cloudwego/eino`
- `github.com/cloudwego/eino-ext/components/model/openai`
- `github.com/cloudwego/eino-ext/libs/acl/openai`

## P3 - Split the frontend production bundle

Expected value: better first-load performance.
Delivery cost: low to medium.
Migration risk: low.

Quick fix direction:

```bash
cd /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize/web
npm run build
```

Use route-level dynamic imports for history/settings/workspace pages and consider Ant Design chunk splitting in Vite/Rolldown output options.
