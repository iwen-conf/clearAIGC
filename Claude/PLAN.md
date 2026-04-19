# Claude 任务计划 — 历史记录功能（前端 + 联调）

> 角色：Claude 负责前端开发，并在前后端交汇点做联调、契约校对、回归验证。后端改动由 Codex 执行（见 `../Codex/PLAN.md`）。
>
> 语言约束：代码、组件注释、commit 使用英文；用户可见的 UI 文案和 PR 描述使用简体中文。

---

## 0. 背景与现状

项目 `Naturalize` 前端（Vite + React 18 + Ant Design 5 + Pro Layout + React Router）目前只有两个主菜单：

- `/` → `WorkspacePage.tsx` — 单活动 session 工作台
- `/settings` → `SettingsPage.tsx`

`useSession` hook 把唯一「活动会话 ID」存在 `localStorage.naturalize.active-document`，上传新文档就把旧会话从 UI 踢掉，**没有任何查看历史的入口**。

同时后端已经：
- `GET /api/v1/sessions` 支持分页
- `GET /api/v1/sessions/:id/state` 返回 preview/comparison/progress/timeline
- `GET /api/v1/sessions/:id/output?round=N`、`GET /api/v1/sessions/:id/diff?round=N`、`GET /api/v1/sessions/:id/export?round=N&format=txt|docx` 都已存在

**缺口：**
1. 没有「历史文档列表」页面。
2. 没有「某文档的处理历史」详情页面，无法看到过往轮次的 timeline / diff / 下载。
3. `useSession` 没有「加载指定历史 session 继续处理」的入口。

本计划覆盖：新增 `/history` 列表 + `/history/:sessionId` 详情 + 与 Codex 后端新契约对齐 + 回归验证。

---

## 1. 任务总览

| ID     | 名称                                      | 依赖后端         |
| ------ | ----------------------------------------- | ---------------- |
| C-F0   | 契约对齐：跟 Codex 过一遍 `SHARED_CONTRACT.md` | —                |
| C-F1   | 更新 `types.ts`（列表、详情、timeline.round） | T-B1/B2/B3       |
| C-F2   | 更新 `api.ts`（新增 list 参数、历史详情）     | T-B1/B3/B4       |
| C-F3   | 注册 `/history` 菜单与路由                  | —                |
| C-F4   | 实现 `HistoryPage.tsx`（列表 + 搜索 + 删除） | T-B1/B4          |
| C-F5   | 实现 `HistoryDetailPage.tsx`（详情）        | T-B2/B3          |
| C-F6   | `useSession` 增加 `adopt(sessionId)` 能力   | —                |
| C-F7   | 工作台上补「返回历史」入口                  | —                |
| C-F8   | 通用 Hooks：`useHistoryList`、`useHistoryDetail` | T-B1/B3      |
| C-F9   | 前端回归验证（build / lint / 手测）         | C-F1..F7         |
| C-F10  | 文档（`tasks/todo.md`）与 PR 描述             | —                |

**工作目录**：`/Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize/web`

**统一验证**（每完成关键任务跑一次）：
```bash
cd web
pnpm install   # 或 npm install,项目用 package-lock.json,以现仓为准
npm run build
npm run lint   # 存量错误已记录,不要让历史错误计数增加
```

---

## 2. C-F0 · 契约对齐

在动手前做一件事：打开 `../Codex/PLAN.md` 与 `./SHARED_CONTRACT.md`，确认以下字段一致：

- `SessionListItem.metrics.completedRounds / totalRounds / totalTokens / lastActivityAt`
- `SessionHistoryResponse.rounds[].summary.*`
- `TimelineEntry.round?` 为可选数字

如果契约里有歧义，在 `SHARED_CONTRACT.md` 顶部的「开放问题」区域记录，等 Codex 确认后再进 F1。

---

## 3. C-F1 · 扩展 `web/src/types.ts`

**新增 / 修改：**

```ts
export interface SessionListMetrics {
  completedRounds: number
  totalRounds: number
  totalTokens: number
  lastActivityAt: string // ISO
}

export interface SessionListItem {
  session: Session
  progress: ProgressPayload | null
  metrics: SessionListMetrics
}

export interface SessionListResponse {
  items: SessionListItem[]
  total: number
  page: number
  size: number
}

export interface RoundSummary {
  chunkCount: number
  passedChunks: number
  recoveredChunks: number
  failedChunks: number
  scoreTotal: number | null
  durationSeconds: number
}

export interface RoundHistoryEntry {
  round: Round
  summary: RoundSummary
}

export interface SessionHistoryResponse {
  session: Session
  progress: ProgressPayload | null
  metrics: SessionListMetrics
  rounds: RoundHistoryEntry[]
  timeline: TimelineEntry[]
}

export interface TimelineEntry {
  id: string
  round?: number        // ← 新增
  tone: 'neutral' | 'warning' | 'success' | 'error'
  title: string
  detail: string
  timestamp: number
}
```

- `Round` / `Session` / `ProgressPayload` 保持原定义。
- `TimelineEntry.round` 标为可选（旧数据没有该字段）。

**验收：** `tsc` 通过；不留 `any`。

---

## 4. C-F2 · 扩展 `web/src/api.ts`

**新增 / 修改：**

```ts
export interface ListSessionsParams {
  page?: number
  size?: number
  status?: SessionStatus | ''
  q?: string
  sort?: '-created_at' | 'created_at' | '-updated_at' | 'updated_at'
}

export async function listSessions(params: ListSessionsParams = {}): Promise<SessionListResponse> {
  const search = new URLSearchParams()
  if (params.page) search.set('page', String(params.page))
  if (params.size) search.set('size', String(params.size))
  if (params.status) search.set('status', params.status)
  if (params.q) search.set('q', params.q)
  if (params.sort) search.set('sort', params.sort)
  const qs = search.toString()
  return request<SessionListResponse>(`/api/v1/sessions${qs ? `?${qs}` : ''}`)
}

export async function getSessionHistory(sessionId: string): Promise<SessionHistoryResponse> {
  return request<SessionHistoryResponse>(`/api/v1/sessions/${sessionId}/history`)
}
```

- 保留现有 `getSession / getSessionState / deleteSession / exportUrl` 不动。
- 不破坏 `createSession` 返回结构（仍是 `SessionSummary`）。

**验收：** 所有 import 路径改动的文件（`use-session.ts` 等）编译通过。

---

## 5. C-F3 · 路由注册

**文件：** `web/src/App.tsx`

- 在 `menuRoute.routes` 里插入新项：
  ```tsx
  { path: '/history', name: '历史记录', icon: <HistoryOutlined /> }
  ```
  放在「工作台」和「智能体配置」之间。
- 在 `<Routes>` 里增加：
  ```tsx
  <Route path="/history" element={<HistoryPage />} />
  <Route path="/history/:sessionId" element={<HistoryDetailPage />} />
  ```
- 从 `@ant-design/icons` import `HistoryOutlined`。

---

## 6. C-F4 · 历史列表页

**文件：** `web/src/pages/HistoryPage.tsx`（新建）

### 布局

- 顶部 `PageContainer`，`title = "历史记录"`，`subTitle = "查看所有上传过的文档与处理进度"`。
- 工具栏：
  - 左：`<Input.Search placeholder="搜索文档名" allowClear />`
  - 中：`<Select>` 状态筛选（全部 / 处理中 / 已完成 / 已暂停 / 失败）
  - 右：排序下拉（最新上传 / 最早上传 / 最近活动）
- 主体：Antd `Table`（不需要 ProTable，避免额外复杂度）：
  | 列             | 字段来源                                          | 渲染                                 |
  | -------------- | ------------------------------------------------- | ------------------------------------ |
  | 文档           | `session.documentName`                            | 文本 + 文件格式徽标（`fileFormat`）  |
  | 大小           | `session.fileSizeBytes`                           | `formatBytes`（新建 util）           |
  | 模式           | `session.promptProfile`                           | `cn → 中文润色` / `en → 英文润色`    |
  | 状态           | `session.status`                                  | 复用 `components/workspace/status-pill.tsx` |
  | 轮次           | `metrics.completedRounds / metrics.totalRounds`   | `1 / 2`                              |
  | 进度           | `progress.percent`（若非空）                      | `<Progress percent>` 小条；无则 `—`  |
  | 最近活动       | `metrics.lastActivityAt`                          | 相对时间（`dayjs.fromNow`）          |
  | 操作           | —                                                 | 「查看详情」「继续处理」「删除」     |

### 交互

- 点击「查看详情」→ `navigate(\`/history/\${id}\`)`。
- 「继续处理」仅在 `status ∈ {pending, paused, processing, failed}` 时显示：点击后调 `useSession.adopt(id)` 并跳转 `/`。
- 「删除」调 `deleteSession` 并 `Modal.confirm`，成功后刷新列表。
- 搜索框使用 `useDebouncedValue`（300ms）避免频繁请求。
- 分页默认 `size = 20`，可切换 10 / 20 / 50。

### 状态管理

- 使用自建 hook `useHistoryList`（见 C-F8）返回 `{ data, loading, error, refetch, params, setParams }`。

### 空状态

- `total === 0` 且无搜索条件：
  - 占位图 + 文案「还没有上传过文档」+ 按钮「去工作台上传」→ 跳转 `/`。
- 有搜索条件但无结果：Antd `Empty` + 文案「没有匹配的记录」。

---

## 7. C-F5 · 历史详情页

**文件：** `web/src/pages/HistoryDetailPage.tsx`（新建）

### 布局

```
PageContainer (header: 返回箭头 + 文档名)
├─ 概要卡片 SessionSummaryCard
│   (文档名、格式、大小、模式、状态、创建时间、更新时间、累计 tokens、已完成/总轮数)
├─ 进度条(仅当 status !== completed)
├─ Tabs
│   ├─ "处理轮次" RoundHistoryList
│   └─ "活动时间线" GroupedActivityTimeline
└─ 底部操作栏:「继续处理」/「回到列表」
```

### 子组件

1. **`SessionSummaryCard`**（新建于 `web/src/components/history/session-summary-card.tsx`）
   - 展示元数据；右上角展示状态徽标。
2. **`RoundHistoryList`**（新建于 `web/src/components/history/round-history-list.tsx`）
   - 每个 round 一个 `Collapse.Panel`：
     - Panel header：第 N 轮 · 状态 · 持续时间 · tokens
     - Panel body：
       - summary 数字块（`chunkCount / passedChunks / recoveredChunks / failedChunks / scoreTotal`）
       - 按钮：`下载 TXT` / `下载 DOCX`（`exportUrl(id, n, 'txt')` / `'docx'`）
       - 按钮：`查看对比`（点击懒加载 `getDiff(id, n)` 进 `CardReviewPanel` 只读模式，或打开抽屉）
     - 如果 `round.status === 'failed'`，header 红色标记；body 显示 `recoveryJustification` 如果有。
3. **`GroupedActivityTimeline`**（新建于 `web/src/components/history/grouped-activity-timeline.tsx`）
   - 按 `entry.round` 分组（0 或 undefined 归「其他」）；组内按时间升序。
   - 内部仍复用 Antd `Timeline`，每条条目的 `color` 根据 `tone` 映射。
4. **只读对比**：扩展 `components/workspace/card-review-panel.tsx` 接受一个 `readOnly?: boolean` prop，当为 true 时隐藏「接受 / 拒绝 / 全部应用」按钮。不要新开一个组件。

### 状态管理

- 使用自建 hook `useHistoryDetail(sessionId)`（见 C-F8）。

### 错误与空态

- 404（会话不存在）：显示 `Result status="404"` + 「返回历史列表」按钮。
- timeline 为空：显示 `Empty` 占位。
- 还没跑过任何 round：`rounds` 空数组时，在「处理轮次」Tab 显示 `Empty`，文案「这份文档还没有任何处理记录」。

---

## 8. C-F6 · `useSession` 增加 `adopt`

**文件：** `web/src/hooks/use-session.ts`

- 新增方法：
  ```ts
  adopt: (sessionId: string) => Promise<void>
  ```
  行为：
  1. 设置 `localStorage.setItem(STORAGE_KEY, sessionId)`
  2. 调 `getSessionState(sessionId)` 拉取最新状态
  3. 把 `session / preview / comparison / progress / timeline` 写入各自 state
  4. 失败时 `setError` 并清掉 localStorage
- 在 `UseSessionResult` 接口里暴露 `adopt`。
- 在 `HistoryPage` 的「继续处理」按钮里调用。

---

## 9. C-F7 · 工作台增加「历史记录」入口

**文件：** `web/src/pages/WorkspacePage.tsx`

- 在 `PageContainer` 的 `extra` 区域增加一个按钮：
  ```tsx
  <Button icon={<HistoryOutlined />} onClick={() => navigate('/history')}>历史记录</Button>
  ```
- 在「未选择文档的上传界面」（`!state.session`）也加一条辅助链接：「或从历史记录中继续之前的文档 →」。

---

## 10. C-F8 · 自建 hooks

**文件：** `web/src/hooks/use-history-list.ts`、`web/src/hooks/use-history-detail.ts`（新建）

### `useHistoryList`

```ts
export interface HistoryListParams {
  page: number
  size: number
  status?: SessionStatus | ''
  q?: string
  sort?: ListSessionsParams['sort']
}

export function useHistoryList(initial?: Partial<HistoryListParams>): {
  data: SessionListResponse | null
  loading: boolean
  error: string | null
  params: HistoryListParams
  setParams: (patch: Partial<HistoryListParams>) => void
  refetch: () => Promise<void>
}
```

- 内部用 `useEffect` 在 `params` 变化时调 `listSessions`，用 `AbortController` 处理竞态。
- 提供 `refetch()` 给删除后强刷。

### `useHistoryDetail`

```ts
export function useHistoryDetail(sessionId: string | undefined): {
  data: SessionHistoryResponse | null
  loading: boolean
  error: string | null
  reload: () => Promise<void>
}
```

- `sessionId` 为空时不触发请求。
- 失败时保留旧 `data`，给 UI 容错。

---

## 11. C-F9 · 前端回归验证

完成 F1..F7 后：

1. **类型检查**：`npm run build`（Vite 会跑 `tsc`）。
2. **Lint**：`npm run lint`。存量错误（`use-agents.ts` / `use-session.ts`）不要把计数变多。
3. **手测步骤**（在后端已经接入 Codex 新契约的前提下）：
   - 上传两份不同文档 → 完成 round 1。
   - 进入 `/history`：
     - 看到两条，文档名、大小、进度都对。
     - 搜「第一个文件名的前缀」→ 只剩一条。
     - 切状态筛选为「已完成」→ 只剩已完成的。
     - 切排序为「最早上传」→ 顺序颠倒。
   - 点任一行的「查看详情」：
     - Round 1 Panel 能展开，下载按钮能触发 TXT 下载。
     - 「查看对比」抽屉能展示片段对比。
     - Timeline 按时间升序，带「第 1 轮」分组。
   - 回 `/history` 点「删除」→ 确认后行消失；刷新仍然没了。
   - 找一个「暂停」状态的 session 点「继续处理」→ 跳到 `/` 且状态恢复。
4. **刷新测试**：在详情页直接粘贴 URL 刷新，页面不应崩，数据重新加载。
5. **移动端窄屏**：Antd `Table` 需要横向滚动或隐藏次要列（可用 `responsive` 属性）。

---

## 12. C-F10 · 文档收尾

- 更新 `tasks/todo.md`：新增「历史记录功能（前端）」分区，勾选完成项。
- PR 描述要点：
  - 新路由 `/history` 与 `/history/:sessionId`
  - 新 API 客户端方法：`listSessions(params) / getSessionHistory(id)`
  - 破坏性变更：`listSessions` 返回结构由 `domain.Session[]` → `SessionListItem[]`（老代码路径已全部替换，不留兼容分支）
  - 截图：列表页、详情页、删除确认、空状态 × 2

---

## 13. 交付顺序建议

1. 等 Codex T-B1 合入 → 做 C-F1 / C-F2 / C-F3 / C-F4 / C-F8 列表 hook。
2. 等 Codex T-B2 + T-B3 合入 → 做 C-F5 / C-F8 详情 hook。
3. 最后做 C-F6 / C-F7 / C-F9 / C-F10。

如果 Codex 的后端改动先落，则每个前端任务的依赖已解；如果还没落，前端可以先写 mock handler（在 `api.ts` 下的同名函数里返回写死 JSON），但 merge 前必须切回真接口。

---

## 14. 不做的事

- **不做** SSE 重构：历史详情页只读，拿 snapshot 即可。
- **不做** 浏览器端 IndexedDB 缓存：数据都从后端 `/history` 实时拉（参见 `tasks/todo.md` 里 2026-04-19 的决策）。
- **不做** Round 级别的「重跑」「暂停单轮」按钮：超出本次范围，老接口继续覆盖。
- **不做** 移动端专属样式重设计：Antd `Table responsive` + 现有 ProLayout 的 sider 折叠够用。
