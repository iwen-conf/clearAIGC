# Codex 任务计划 — 历史记录功能（后端 + 数据库）

> 角色：Codex 负责后端逻辑与数据库。前端由 Claude 负责。两者共享同一套 HTTP/JSON 契约（见 `SHARED_CONTRACT.md`）。
>
> 语言约束：代码、注释、commit、日志使用英文；与用户沟通的 PR 描述使用中文。

---

## 0. 背景与现状

项目 `Naturalize` 当前：

- 每次上传文档都会在 `sessions` 表里落一行，`rounds` 表记录每一轮润色。
- 新增的迁移 `000007_create_session_progress.up.sql` 建了 `session_progress` 表，用于持久化每个 session 的实时进度快照（round_number, phase, completed/total_chunks, percent, chunk_id 等）。
- `internal/service/tracking_publisher.go` 的 `TrackingPublisher` 已经把 Redis Pub/Sub 进度事件镜像写入 `session_progress` + 以 `audit_log.action = 'timeline.entry'` 的 JSONB 形式落库。
- `internal/domain/state.go` 定义了 `SessionProgressSnapshot / SessionTimelineEntry / SessionState`。
- `internal/infra/postgres/state_repo.go` 提供了 `UpsertProgress / GetProgress / AppendTimeline / ListTimeline / DeleteForSession`。
- 现有 HTTP 层已经有：
  - `GET /api/v1/sessions?page=&size=&status=` — 分页，返回 `domain.Session`（嵌套 rounds）
  - `GET /api/v1/sessions/:id/state` — 返回 session + preview + comparison + 当前 progress + 最近 100 条 timeline
  - `DELETE /api/v1/sessions/:id` — 已经会级联删除 state/progress/timeline

**缺口：**
1. 列表页需要的 **文档级元数据**（最新进度、已完成轮数、累计 tokens）没有在 list 接口一次返回，前端会被迫 N+1。
2. 时间线条目没有 `round` 字段，详情页无法按轮次分组。
3. 详情页需要的「全量时间线 + 全部 rounds + 每轮 manifest 摘要」没有一个聚合端点，前端要拼多个接口。
4. 列表缺少按文档名模糊搜索、按创建时间排序的能力。

---

## 1. 任务总览

| ID     | 名称                                | 依赖     | 阻塞前端任务 |
| ------ | ----------------------------------- | -------- | ------------ |
| T-B1   | 增强 `GET /sessions` 列表返回结构   | —        | C-F1, C-F4   |
| T-B2   | 为 timeline 条目增加 `round` 字段   | —        | C-F5         |
| T-B3   | 新增 `GET /sessions/:id/history`    | T-B2     | C-F5         |
| T-B4   | 列表筛选：`q`（文档名模糊）+ `sort` | T-B1     | C-F4         |
| T-B5   | 删除时清理（回归验证）              | —        | —            |
| T-B6   | 单元测试 + 集成测试                 | T-B1..B4 | —            |
| T-B7   | 文档与迁移说明更新                  | T-B1..B4 | —            |

**工作目录根**：`/Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize`

**统一验证**（每完成一个任务都要跑）：
```bash
gofmt -w $(git ls-files '*.go')
go vet ./...
go test ./... -count=1
```

---

## 2. T-B1 · 增强 `GET /sessions` 列表返回

### 目标

让前端一次拿到历史列表所需的全部元数据，避免 N+1。

### 契约

**请求**
```
GET /api/v1/sessions?page=1&size=20&status=&q=&sort=-created_at
```

**响应**（新增字段见 `SHARED_CONTRACT.md` §3.1）
```json
{
  "items": [
    {
      "session": { /* 原 domain.Session,含 rounds */ },
      "progress": { /* *domain.SessionProgressSnapshot, 可为 null */ },
      "metrics": {
        "completedRounds": 1,
        "totalRounds": 2,
        "totalTokens": 1834,
        "lastActivityAt": "2026-04-19T12:34:56Z"
      }
    }
  ],
  "total": 42,
  "page": 1,
  "size": 20
}
```

### 实现要点

- **新建**领域类型 `internal/domain/session_list.go`：
  ```go
  type SessionListItem struct {
      Session  *Session                 `json:"session"`
      Progress *SessionProgressSnapshot `json:"progress,omitempty"`
      Metrics  SessionListMetrics       `json:"metrics"`
  }
  type SessionListMetrics struct {
      CompletedRounds int       `json:"completedRounds"`
      TotalRounds     int       `json:"totalRounds"`
      TotalTokens     int64     `json:"totalTokens"`
      LastActivityAt  time.Time `json:"lastActivityAt"`
  }
  ```
- **Service 层** `internal/service/service.go` 的 `ListSessions`：
  1. 调现有 `sessions.List(ctx, filter)` 拿到 `[]Session`。
  2. 如果 `s.states != nil`，再做一次 `state_repo.ListProgressBySessionIDs(ctx, ids) -> map[uuid.UUID]*SessionProgressSnapshot`（见下一步）。
  3. 计算 `Metrics`：从 `Session.Rounds` 汇总 `CompletedRounds / TotalRounds / TotalTokens`；`LastActivityAt = max(session.updated_at, progress.updated_at, max(round.completed_at))`。
  4. 返回 `[]SessionListItem, total`。
- **Repo 扩展** `internal/infra/postgres/state_repo.go`：
  - 新增 `ListProgressBySessionIDs(ctx, ids []uuid.UUID) (map[uuid.UUID]*domain.SessionProgressSnapshot, error)`，一次性 `SELECT ... WHERE session_id = ANY($1)`。
  - 同步更新 `domain.SessionStateRepository` 接口定义在 `internal/domain/interfaces.go`。
- **Handler** `internal/api/handler/handler.go::ListSessions`：
  - 把 `[]SessionListItem` 作为 `items` 返回。
- **兼容性**：外部 API 的顶层结构 `{items, total, page, size}` 不变，只是每个 `item` 从 `Session` 变成 `SessionListItem`。前端会同步改类型。

### 验收
- `curl '/api/v1/sessions?page=1&size=5'` 返回的 `items[0]` 带 `session/progress/metrics` 三级字段。
- 空库时 `items = []`，不是 `null`。
- `size > 100` 需要被 clamp 到 100。

---

## 3. T-B2 · 时间线增加 `round` 字段

### 目标

让前端详情页按轮次分组时间线。

### 做法

**不新增迁移**，`round` 作为 `audit_log.detail` JSONB 的一个键位写入/读出。

- **domain** `internal/domain/state.go`：
  ```go
  type SessionTimelineEntry struct {
      ID        string       `json:"id"`
      SessionID uuid.UUID    `json:"-"`
      Round     int          `json:"round,omitempty"`
      Tone      TimelineTone `json:"tone"`
      Title     string       `json:"title"`
      Detail    string       `json:"detail"`
      Timestamp int64        `json:"timestamp"`
  }
  ```
- **state_repo** 的 `AppendTimeline`：在 `payload` map 里新增 `"round": entry.Round`；`ListTimeline` 的匿名 struct 添加 `Round int \`json:"round"\``。
- **tracking_publisher** `internal/service/tracking_publisher.go`：对每个 case 在构造 `SessionTimelineEntry` 时填入 `Round`：
  - `progress / complete / paused`：直接从 payload 取 `Round`。
  - `quality_alert / recovery`：payload 本身不带 round，读一次 `state.GetProgress(ctx, sessionID)`，取 `snapshot.Round`（已经读过的话复用）。
  - `error`：取当前 progress 的 round，如果没有则为 0。
- **历史数据**：旧条目缺少 `round` 字段时反序列化为 0；前端按 0 归到「未分组」。无需回填。

### 验收
- 新跑一次完整流程，`SELECT detail FROM audit_log WHERE action='timeline.entry' ORDER BY created_at DESC LIMIT 5;` 的 JSONB 里包含 `"round": N`。
- `ListTimeline` 返回的每个 entry `Round > 0`（对新会话）。

---

## 4. T-B3 · 新增 `GET /sessions/:id/history`

### 目标

为详情页提供一次性聚合数据，避免前端拼多个接口。

### 契约

**请求**
```
GET /api/v1/sessions/:id/history
```

**响应**
```json
{
  "session": { /* domain.Session 含 rounds */ },
  "progress": { /* 可为 null */ },
  "metrics": { /* 同 SessionListMetrics */ },
  "rounds": [
    {
      "round": { /* domain.Round */ },
      "summary": {
        "chunkCount": 6,
        "passedChunks": 5,
        "recoveredChunks": 1,
        "failedChunks": 0,
        "scoreTotal": 68,
        "durationSeconds": 42
      }
    }
  ],
  "timeline": [ /* 全部 SessionTimelineEntry,按 created_at ASC 排序 */ ]
}
```

### 实现要点

- **Service** 新增 `Service.ReadHistory(ctx, sessionID)`，位置在 `internal/service/service.go` 的 `ReadState` 下方：
  1. `GetSession` 拿 session（含 rounds）。
  2. `state.GetProgress` 拿当前 progress；`state.ListTimeline(ctx, id, 0)` 拿全量 timeline——`limit = 0` 的语义需要在 `state_repo` 里改成「不限制」。
  3. 对 `session.Rounds`：从 `manifests.GetByRoundID` 拿 manifest，统计 chunkCount / passed / recovered / failed；duration = `CompletedAt - StartedAt`，都空则 0。
  4. 排序：rounds 按 `Number ASC`；timeline 按 `created_at ASC`。
- **state_repo** `ListTimeline`：当 `limit <= 0` 时改为不加 `LIMIT` 子句，排序改为 `created_at ASC, id ASC`（历史阅读习惯）。保持原 `GET /state` 的调用继续传 `100` 且期望 DESC——让调用者决定是否 DESC；可以新增一个方法 `ListTimelineAscending` 或加一个参数 `direction`。简洁方案：新增布尔参数 `ascending` 到接口。
  - **接口变更**：`ListTimeline(ctx, sessionID uuid.UUID, limit int, ascending bool) ([]SessionTimelineEntry, error)`
  - 同步改 `ReadState` 传 `ListTimeline(ctx, id, 100, false)`。
- **Handler** `handler.go` 新增 `GetSessionHistory`；**Router** `router.go` 新增 `v1.GET("/sessions/:id/history", h.GetSessionHistory)`。
- **错误处理**：session 不存在 → 404；其他错误 → 500。

### 验收
- 启动后端，创建一个 session 并跑完一轮，调用 `/api/v1/sessions/<id>/history`：
  - `rounds.length >= 1`
  - `timeline` 按时间升序
  - 每个 round 的 `summary.chunkCount` 等于 manifest 中 chunk 数

---

## 5. T-B4 · 列表支持搜索与排序

### 目标

列表页「按文档名搜索」「按时间倒序」。

### 做法

- **`SessionListFilter`** `internal/domain/interfaces.go` 扩展：
  ```go
  type SessionListFilter struct {
      Page   int
      Size   int
      Status SessionStatus
      Query  string // 文档名模糊匹配(ILIKE %q%)
      Sort   string // 允许值: "-created_at"(default), "created_at", "-updated_at", "updated_at"
  }
  ```
- **Postgres `SessionRepository.List`**（`internal/infra/postgres/` 下找对应实现，约在 `session_repo.go`）：
  - `Query` 非空时拼 `AND document_name ILIKE '%' || $N || '%'`（使用参数绑定，严禁字符串拼接）。
  - `Sort` 白名单映射到 `ORDER BY ... ASC/DESC`，不在白名单回退到 `created_at DESC`。
- **Handler** 读 `c.Query("q")` 与 `c.Query("sort")`，Trim 空白后塞到 filter。
- **size** clamp：默认 20，最大 100。

### 验收
- `GET /api/v1/sessions?q=demo` 只返回文档名含 `demo` 的条目（大小写不敏感）。
- `GET /api/v1/sessions?sort=created_at` 升序；默认降序。

---

## 6. T-B5 · 删除路径回归

### 目标

确认 `DELETE /sessions/:id` 和文件/数据/进度一起被清理。

- 现有 `Service.DeleteSession` 已调用 `state.DeleteForSession`。需要确认 `SessionRepository.Delete` 通过 `ON DELETE CASCADE` 连带删除 `rounds / manifests / quality_reports / audit_log / session_progress`。
- 写一个集成测试 `TestDeleteSessionRemovesHistory`：创建 session → 追加 timeline & progress → delete → `ListTimeline` 与 `GetProgress` 返回 not found。

---

## 7. T-B6 · 测试策略

**单测位置与命名：**

| 目标                         | 文件                                              | 测试名                               |
| ---------------------------- | ------------------------------------------------- | ------------------------------------ |
| List metrics 汇总正确         | `internal/service/service_test.go`                | `TestListSessionsMetrics`            |
| Timeline 带 round            | `internal/service/tracking_publisher_test.go`（新增） | `TestRecordEventSetsRound`           |
| History 端点 happy path      | `internal/api/handler/handler_test.go`            | `TestGetSessionHistory`              |
| Search & sort                | `internal/api/handler/handler_test.go`            | `TestListSessionsSearchAndSort`      |
| Delete 级联                  | 同上                                              | `TestDeleteSessionRemovesHistory`    |

**全量回归：**
```bash
go test ./... -count=1 -race
```

---

## 8. T-B7 · 文档与移交

- 更新 `tasks/todo.md`：在现有清单末尾追加「历史记录功能」分区，列出 T-B1..B6 的勾选状态。
- 如果 `docs/` 内存在 API 定义文档（如 `docs/03-api.md`），把 `/sessions` 新结构与 `/sessions/:id/history` 新端点补进去。
- 在 PR 描述里贴出：
  - 迁移清单：无新增 SQL 迁移（JSONB 扩展字段），需要强调。
  - 兼容性：`/api/v1/sessions` 返回的每个 item 结构变化，前端需同步升级。
- 完成后把 `SHARED_CONTRACT.md` 里的「后端状态」字段改为 ✅ 并注明 commit hash。

---

## 9. 不做的事

- **不做** 新增迁移：本次所有持久化扩展都能复用现有表。
- **不做** 前端代码：由 Claude 执行。
- **不做** 性能优化（索引、物化视图）：当前量级（人手上传）没有必要，Postgres 顺序扫 + 现有索引足矣。
- **不做** SSE 事件结构变更：`/stream` 的 `data` 形状保持不动；只有持久化的 timeline 追加 `round`。
