# Naturalize — API 规范

## 1. REST API

基础路径：`/api/v1`

### 1.1 Session 管理

| 方法 | 路径 | 说明 | 请求体 | 响应 |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/sessions` | 上传文档，创建会话 | `multipart/form-data: file, promptProfile` | `{id, docId, status, promptProfile}` |
| `POST` | `/sessions/batch` | 批量上传多文档 | `multipart/form-data: files[], promptProfile` | `{sessions[{id, docId, status}]}` |
| `GET` | `/sessions/{id}` | 获取会话详情 | — | `{id, docId, status, rounds[], createdAt}` |
| `GET` | `/sessions/{id}/history` | 获取历史详情聚合 | — | `{session, progress, metrics, rounds[], timeline[]}` |
| `DELETE` | `/sessions/{id}` | 删除会话及所有关联数据 | — | `204` |
| `GET` | `/sessions` | 分页查询会话列表 | `?page=1&size=20&status=completed&q=demo&sort=-created_at` | `{items[], total, page, size}` |

**列表响应**：

```json
{
  "items": [
    {
      "session": {
        "id": "uuid",
        "documentName": "demo.txt",
        "promptProfile": "cn",
        "status": "completed",
        "rounds": [
          {
            "id": "uuid",
            "number": 1,
            "providerUsed": "gpt-4.1-mini",
            "totalTokens": 1834,
            "status": "completed"
          }
        ]
      },
      "progress": {
        "sessionId": "uuid",
        "round": 1,
        "phase": "complete",
        "completedChunks": 6,
        "totalChunks": 6,
        "percent": 100,
        "providerUsed": "gpt-4.1-mini",
        "updatedAt": "2026-04-19T10:02:30Z"
      },
      "metrics": {
        "completedRounds": 1,
        "totalRounds": 2,
        "totalTokens": 1834,
        "lastActivityAt": "2026-04-19T10:02:34Z"
      }
    }
  ],
  "total": 42,
  "page": 1,
  "size": 20
}
```

**历史详情响应**：

```json
{
  "session": { "id": "uuid", "documentName": "demo.txt", "rounds": [] },
  "progress": {
    "sessionId": "uuid",
    "round": 1,
    "phase": "complete",
    "completedChunks": 6,
    "totalChunks": 6,
    "percent": 100,
    "providerUsed": "gpt-4.1-mini",
    "updatedAt": "2026-04-19T10:02:30Z"
  },
  "metrics": {
    "completedRounds": 1,
    "totalRounds": 2,
    "totalTokens": 1834,
    "lastActivityAt": "2026-04-19T10:02:34Z"
  },
  "rounds": [
    {
      "round": { "id": "uuid", "number": 1, "status": "completed" },
      "summary": {
        "chunkCount": 6,
        "passedChunks": 5,
        "recoveredChunks": 1,
        "failedChunks": 0,
        "scoreTotal": 68,
        "durationSeconds": 145
      }
    }
  ],
  "timeline": [
    {
      "id": "1042",
      "round": 1,
      "tone": "success",
      "title": "第 1 轮完成",
      "detail": "6/6 片段就绪 · 消耗 1834 tokens",
      "timestamp": 1745049754000
    }
  ]
}
```

### 1.2 Round 控制

| 方法 | 路径 | 说明 | 请求体 | 响应 |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/sessions/{id}/start` | 启动下一轮处理 | `{chunkLimit?: 850}` | `{roundId, roundNumber, status}` |
| `POST` | `/sessions/{id}/pause` | 暂停当前处理 | — | `{status: "paused", checkpointId}` |
| `POST` | `/sessions/{id}/resume` | 从断点恢复 | — | `{roundId, status: "processing"}` |
| `GET` | `/sessions/{id}/rounds/{num}` | 获取轮次详情 | — | Round 详情（见下文） |

**Round 详情响应**：

```json
{
  "id": "uuid",
  "number": 1,
  "status": "completed",
  "scoreTotal": 62,
  "chunkLimit": 850,
  "inputSegments": 45,
  "outputSegments": 45,
  "providerUsed": "openai",
  "totalTokens": 125000,
  "startedAt": "2026-04-16T10:00:00Z",
  "completedAt": "2026-04-16T10:02:30Z",
  "quality": {
    "totalChunks": 45,
    "passedChunks": 42,
    "recoveredChunks": 2,
    "failedChunks": 1,
    "passRate": 0.933,
    "recoveryRate": 0.667
  }
}
```

### 1.3 导出

| 方法 | 路径 | 说明 | 响应 |
| :--- | :--- | :--- | :--- |
| `GET` | `/sessions/{id}/export?round={num}&format={txt\|docx}` | 下载输出文件 | `application/octet-stream` |
| `GET` | `/sessions/{id}/output?round={num}` | 读取输出文本 | `{text, segmentCount, paragraphCount}` |
| `GET` | `/sessions/{id}/diff?round={num}` | 获取输入输出 diff | Diff 响应（见下文） |

**Diff 响应**：

```json
{
  "round": 1,
  "chunks": [
    {
      "id": "p0_c0",
      "paragraphIndex": 0,
      "input": "原始文本...",
      "output": "改写后文本...",
      "status": "passed",
      "charDelta": -12,
      "checks": [
        {"type": "empty", "passed": true},
        {"type": "disallowed_pattern", "passed": true}
      ]
    }
  ]
}
```

### 1.4 配置

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| `GET` | `/config` | 读取模型配置（Provider 列表 + 当前设置） |
| `PUT` | `/config` | 更新模型配置 |
| `POST` | `/config/test` | 测试 LLM 连通性（指定 Provider） |
| `GET` | `/config/providers` | 获取 Provider 列表及健康状态 |

**Provider 健康状态**：

```json
{
  "providers": [
    {
      "name": "openai",
      "priority": 1,
      "healthy": true,
      "circuitState": "closed",
      "avgLatencyMs": 320,
      "requestsTotal": 15000,
      "errorsTotal": 12
    },
    {
      "name": "deepseek",
      "priority": 2,
      "healthy": true,
      "circuitState": "closed",
      "avgLatencyMs": 450
    }
  ]
}
```

### 1.5 Webhook 管理

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| `POST` | `/webhooks` | 注册新的 Webhook |
| `GET` | `/webhooks` | 列出所有 Webhook 配置 |
| `PUT` | `/webhooks/{id}` | 更新 Webhook 配置 |
| `DELETE` | `/webhooks/{id}` | 删除 Webhook |
| `GET` | `/webhooks/{id}/deliveries` | 查看投递记录 |
| `POST` | `/webhooks/{id}/test` | 发送测试事件 |

**注册请求**：

```json
{
  "url": "https://example.com/hooks/naturalize",
  "secret": "whsec_...",
  "events": ["round.completed", "session.completed", "quality.alert"]
}
```

**Webhook Payload 签名**：

```
X-Naturalize-Signature: sha256=<HMAC-SHA256(secret, payload)>
X-Naturalize-Event: round.completed
X-Naturalize-Delivery: <delivery-uuid>
X-Naturalize-Timestamp: 1713264000
```

### 1.6 系统

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| `GET` | `/health` | 健康检查（含 PG/Redis/LLM 状态） |
| `GET` | `/health/ready` | 就绪检查（K8s readiness probe） |
| `GET` | `/health/live` | 存活检查（K8s liveness probe） |
| `GET` | `/metrics` | Prometheus 指标端点 |

**健康检查响应**：

```json
{
  "status": "healthy",
  "uptime": "2h35m12s",
  "checks": {
    "postgres": "healthy",
    "redis": "healthy",
    "llm": "healthy"
  },
  "version": "1.0.0",
  "buildTime": "2026-04-16T08:00:00Z"
}
```

## 2. SSE 流

**端点**：`GET /api/v1/sessions/{id}/stream`

通过 Redis Pub/Sub 将 Workflow 内部事件实时推送到前端。

### 2.1 事件类型

**`progress`** — chunk 处理进度

```json
{
  "event": "progress",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "round": 1,
    "phase": "chunk-complete",
    "completedChunks": 45,
    "totalChunks": 100,
    "percent": 45.0,
    "chunkId": "p3_c2",
    "paragraphIndex": 3,
    "chunkIndex": 2,
    "elapsedMs": 1230,
    "providerUsed": "openai"
  }
}
```

**`quality_alert`** — chunk 未通过质量门控

```json
{
  "event": "quality_alert",
  "data": {
    "chunkId": "p5_c0",
    "checkType": "disallowed_pattern",
    "reason": "Output contains '修改后：'",
    "action": "invoking_react_recovery",
    "recoveryStep": 1
  }
}
```

**`recovery`** — ReAct Agent 修复结果

```json
{
  "event": "recovery",
  "data": {
    "chunkId": "p5_c0",
    "success": true,
    "method": "retry_with_strict_prompt",
    "steps": 2,
    "tokenCost": 450
  }
}
```

**`complete`** — 轮次完成

```json
{
  "event": "complete",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "round": 1,
    "scoreTotal": 58,
    "chunkCount": 100,
    "passedChunks": 95,
    "recoveredChunks": 4,
    "failedChunks": 1,
    "totalTokens": 125000,
    "elapsedMs": 45000,
    "providerUsed": "openai",
    "downloadUrl": "/api/v1/sessions/550e.../export?round=1"
  }
}
```

**`error`** — 处理错误

```json
{
  "event": "error",
  "data": {
    "chunkId": "p5_c0",
    "message": "LLM request timeout after 30s",
    "recoverable": true,
    "code": "LLM_TIMEOUT"
  }
}
```

**`paused`** — 处理暂停

```json
{
  "event": "paused",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "round": 1,
    "completedChunks": 45,
    "totalChunks": 100,
    "checkpointId": "session-550e...-round-1",
    "reason": "user_requested"
  }
}
```

### 2.2 SSE 实现

```go
func (h *SSEHandler) Stream(c *gin.Context) {
    sessionID := c.Param("id")
    
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")  // Nginx SSE 支持
    
    events, cancel, err := h.publisher.Subscribe(c.Request.Context(), sessionID)
    if err != nil {
        c.AbortWithError(500, err)
        return
    }
    defer cancel()
    
    // 心跳保活
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    
    flusher := c.Writer.(http.Flusher)
    for {
        select {
        case event, ok := <-events:
            if !ok {
                return
            }
            data, _ := json.Marshal(event.Data)
            fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
            flusher.Flush()
        case <-ticker.C:
            fmt.Fprintf(c.Writer, ": heartbeat\n\n")
            flusher.Flush()
        case <-c.Request.Context().Done():
            return
        }
    }
}
```

## 3. 错误响应格式

所有 API 错误使用统一格式：

```json
{
  "error": {
    "code": "SESSION_NOT_FOUND",
    "message": "Session 550e8400... not found",
    "details": {},
    "traceId": "abc123def456"
  }
}
```

| HTTP 状态码 | 错误码 | 说明 |
| :--- | :--- | :--- |
| 400 | `INVALID_REQUEST` | 请求参数校验失败 |
| 400 | `INVALID_FILE_FORMAT` | 不支持的文件格式 |
| 404 | `SESSION_NOT_FOUND` | 会话不存在 |
| 404 | `ROUND_NOT_FOUND` | 轮次不存在 |
| 409 | `SESSION_LOCKED` | 会话正在处理中 |
| 409 | `ROUND_ALREADY_COMPLETED` | 轮次已完成 |
| 413 | `FILE_TOO_LARGE` | 文件超过 50MB 限制 |
| 422 | `ALL_ROUNDS_COMPLETED` | 所有轮次已完成 |
| 429 | `RATE_LIMITED` | API 限流 |
| 502 | `LLM_UNAVAILABLE` | LLM Provider 不可用 |
| 503 | `SERVICE_DEGRADED` | 熔断器打开 |

## 4. Webhook 回调规范

### 4.1 事件类型

| 事件 | 触发时机 | Payload 字段 |
| :--- | :--- | :--- |
| `round.completed` | 单轮处理完成 | sessionId, round, scoreTotal, downloadUrl |
| `session.completed` | 所有轮次完成 | sessionId, rounds[], totalTokens |
| `quality.alert` | chunk 质量检查失败率 > 20% | sessionId, round, failRate, failedChecks[] |
| `session.failed` | Session 不可恢复失败 | sessionId, round, error |

### 4.2 投递策略

- 异步投递，不阻塞主流程
- 签名验证：`HMAC-SHA256(secret, timestamp + "." + payload)`
- 重试：指数退避（1s, 4s, 16s），最多 3 次
- 超时：单次 10s
- 投递记录保留 30 天

## 5. 认证（Phase 5）

JWT Bearer Token 认证：

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

| 端点 | 方法 | 说明 |
| :--- | :--- | :--- |
| `POST /auth/login` | — | 获取 JWT |
| `POST /auth/refresh` | — | 刷新 JWT |

Phase 1-4 可跳过认证中间件。
