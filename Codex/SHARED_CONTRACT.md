# 前后端共享契约 — 历史记录功能

> 这是 Codex 与 Claude 双方唯一的「单一事实来源」。任何对 JSON 形状、字段语义、端点路径的分歧都必须先在这里改完、双方确认，才能动手。
>
> **同一份文件会同时放在 `Claude/` 与 `Codex/` 下（软链或复制）**。本项目此处为正本。

---

## 0. 后端/前端状态

| 条目                        | 后端 (Codex) | 前端 (Claude) |
| --------------------------- | ------------ | ------------- |
| `SessionListItem` 结构       | ✅           | ✅            |
| `TimelineEntry.round` 字段   | ✅           | ✅            |
| `GET /sessions/:id/history`  | ✅           | ✅            |
| 列表搜索/排序 query         | ✅           | ✅            |

⏳ = 待实现；✅ = 已合入 main；❌ = 有歧义需回到「开放问题」讨论。

注：后端状态已在当前工作区实现并通过验证，尚未生成独立 commit hash。

---

## 1. 端点总览

| 方法 | 路径                                  | 用途                       | 备注                                        |
| ---- | ------------------------------------- | -------------------------- | ------------------------------------------- |
| GET  | `/api/v1/sessions`                    | 历史列表                   | **结构变更**：每项从 `Session` → `SessionListItem` |
| GET  | `/api/v1/sessions/:id/history`        | 历史详情聚合               | **新增**                                    |
| DELETE | `/api/v1/sessions/:id`              | 删除并级联清理             | 不变                                        |
| GET  | `/api/v1/sessions/:id/state`          | 工作台状态恢复             | 不变（timeline 项可能多了 `round` 字段）    |
| GET  | `/api/v1/sessions/:id/diff?round=N`   | 单轮对比                   | 不变                                        |
| GET  | `/api/v1/sessions/:id/output?round=N` | 单轮输出                   | 不变                                        |
| GET  | `/api/v1/sessions/:id/export?round=N&format=txt\|docx` | 下载导出 | 不变                                        |

---

## 2. 查询参数 — `GET /api/v1/sessions`

| 参数     | 类型                                                      | 默认          | 说明                              |
| -------- | --------------------------------------------------------- | ------------- | --------------------------------- |
| `page`   | int ≥ 1                                                   | 1             | 页码                              |
| `size`   | int 1–100                                                 | 20            | 每页数量；超出 100 clamp 到 100   |
| `status` | `pending\|processing\|paused\|completed\|failed\|`（空） | 空 = 全部     | 精确匹配                          |
| `q`      | string（trim 后 ≤ 128 字符）                              | 空            | 在 `document_name` 上 ILIKE       |
| `sort`   | `-created_at` / `created_at` / `-updated_at` / `updated_at` | `-created_at` | 非白名单值 → fallback 默认         |

不在白名单里的 sort 值，后端静默回退；不报错。

---

## 3. 响应结构

### 3.1 列表 — `GET /api/v1/sessions`

```jsonc
{
  "items": [
    {
      "session": {
        "id": "uuid",
        "documentId": "uuid",
        "documentName": "合同_2026.docx",
        "docId": "doc_abc123",
        "originPath": "/data/sessions/<id>/input/源文件.docx",
        "fileFormat": "docx",
        "fileSizeBytes": 123456,
        "promptProfile": "cn",
        "status": "completed",
        "createdAt": "2026-04-19T10:00:00Z",
        "updatedAt": "2026-04-19T10:02:34Z",
        "rounds": [
          {
            "id": "uuid",
            "sessionId": "uuid",
            "number": 1,
            "status": "completed",
            "providerUsed": "gpt-4.1-mini",
            "totalTokens": 1834,
            "startedAt": "2026-04-19T10:00:05Z",
            "completedAt": "2026-04-19T10:02:30Z",
            "createdAt": "2026-04-19T10:00:05Z"
            /* 其它 Round 字段完整保留 */
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
        "chunkId": "",
        "paragraphIndex": 0,
        "chunkIndex": 0,
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

**语义：**

- `progress` 可能为 `null`（老 session 还没跑进度，或已被 purge）。
- `metrics.lastActivityAt = max(session.updatedAt, progress.updatedAt, max(round.completedAt))`，空值不计入。
- `metrics.totalRounds` 取自 `session.promptProfile`：cn → 2，en → 1。Codex 使用 `domain.TotalPassesFor(profile)` 统一计算。
- `metrics.totalTokens` = `Σ round.totalTokens`。

### 3.2 历史详情 — `GET /api/v1/sessions/:id/history`

```jsonc
{
  "session": { /* 同列表中的 session */ },
  "progress": { /* 同列表中的 progress,可为 null */ },
  "metrics": { /* 同列表中的 metrics */ },
  "rounds": [
    {
      "round": { /* 完整 Round */ },
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

**语义：**

- `rounds` 按 `round.number ASC`。
- `timeline` 按 `timestamp ASC`（升序）——详情页阅读顺序。注意这与 `/state` 的降序不同。
- `timeline[*].round` 为整数；旧数据为 `0` 或缺省，前端按「未分组」处理。
- `summary.durationSeconds = round.completedAt - round.startedAt`，任一为空则 `0`。
- `summary.scoreTotal` 为 `null` 当 round 未完成。

### 3.3 `TimelineEntry`（全局）

```jsonc
{
  "id": "string",
  "round": 0,                       // 可缺省; 0 表示未分组
  "tone": "neutral|warning|success|error",
  "title": "string",
  "detail": "string",
  "timestamp": 1745049754000        // epoch ms
}
```

- `/state` 中 `timeline` 沿用降序 + limit 100。
- `/history` 中 `timeline` 升序 + 无 limit。
- 后端在 `audit_log.detail` JSONB 中追加 `round` 键；迁移不必要。

---

## 4. 错误约定

- 400：`q` 超过 128 字符 / `size` 为负数 / `page` < 1。
- 404：`GET /sessions/:id/history`、`DELETE /sessions/:id` 找不到 session。
- 其他：沿用现有错误包 `{"error":{"code":"","message":"","details":{}}}`。

---

## 5. 前端调用约定

| 函数                           | 签名                                                                                  |
| ------------------------------ | ------------------------------------------------------------------------------------- |
| `listSessions(params)`         | `(params?: ListSessionsParams) => Promise<SessionListResponse>`                       |
| `getSessionHistory(sessionId)` | `(sessionId: string) => Promise<SessionHistoryResponse>`                              |
| `deleteSession(sessionId)`     | 不变                                                                                  |
| `exportUrl(id, round, fmt)`    | 不变                                                                                  |

- 请求失败时 throw `Error(message)`；前端在 hook 里捕获并写 `error`。
- 搜索框 debounce 300ms；翻页立即触发。

---

## 6. 开放问题（Open Questions）

> 如果 Codex 或 Claude 发现下列疑点，**先在这里写清楚，不要开始编码**。

- [ ] `GET /sessions` 响应是否还要保留一份旧版的 flat `Session` 列表以向下兼容移动端？**当前决定**：不保留，此接口只有 web 内部调用方。
- [ ] `timeline.round` 的字段名是用 `round` 还是 `roundNumber`？**当前决定**：`round`（与 `SessionProgressSnapshot.round` 对齐）。
- [ ] 详情页「查看对比」按钮是否需要新端点？**当前决定**：复用 `GET /sessions/:id/diff?round=N`。

---

## 7. 变更记录

| 日期       | 修改人 | 说明                     |
| ---------- | ------ | ------------------------ |
| 2026-04-19 | PM     | 初稿                     |
| 2026-04-19 | Claude | 前端全部落地：列表/详情页、历史 hooks、`useSession.adopt`、`CardReviewPanel.readOnly`；`npm run build` 通过，`npm run lint` 维持 3 条存量报错。 |
