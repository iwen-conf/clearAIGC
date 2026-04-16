# clearAIGC — 交付路线图

## 总览

```
Phase 1        Phase 2         Phase 3         Phase 4         Phase 5
Foundation     Core Pipeline   Quality+Perf    Advanced        Enterprise
─────────────  ──────────────  ──────────────  ──────────────  ──────────────
项目脚手架      Eino Workflow    增强质量门控     ReAct 高级工具   JWT/RBAC
领域模型        BatchNode 并发   Provider Chain   术语字典         多租户 RLS
PostgreSQL      质量门控         LLM 缓存         批量处理         分区表
Redis           ReAct Agent      SLI/SLO          Webhook         数据加密
REST API        断点续传         成本分析          OpenAPI 文档    审计增强
SSE 基础        SSE 完整                          前端迁移
```

## Phase 1: Foundation（基础搭建）

**目标**：可编译运行的项目骨架 + 核心存储层 + 基础 API

### 交付物

| 任务 | 说明 | 验收标准 |
| :--- | :--- | :--- |
| 项目脚手架 | `cmd/server`, `internal/`, `pkg/`, `deploy/` 目录结构 | `go build ./...` 通过 |
| 领域模型 | `internal/domain/` 全部实体和接口定义 | 零外部依赖，编译通过 |
| PostgreSQL 集成 | 连接池、迁移脚本、Session/Round/Manifest CRUD | `docker-compose up` 后 migrate 成功 |
| Redis 集成 | 连接、CheckPointStore 接口实现 | Get/Put 单测通过 |
| REST API 骨架 | Gin 路由、Session CRUD、健康检查 | `POST /sessions` 创建成功，`GET /health` 返回 200 |
| SSE 骨架 | 基础 SSE 连接、心跳保活 | 浏览器 EventSource 连接成功 |
| 文档解析 | ParseNode: txt/docx → 纯文本 | .txt 和 .docx 文件均可正确提取 |
| 4 级分块 | ChunkNode: 段落 → 句子 → 标点 → 硬截断 | 中英文文档分块结果符合预期，Manifest 正确 |
| 配置管理 | Viper 配置加载（文件 + 环境变量） | 所有配置项可通过环境变量覆盖 |
| Docker Compose | PG + Redis + App 本地开发环境 | `docker-compose up -d` 一键启动 |

### 技术决策

- ORM 使用 GORM（成熟、社区大、Migration 支持好）
- 配置使用 Viper（多源、热加载、结构化绑定）
- 日志使用 slog（标准库、结构化、零分配）

---

## Phase 2: Core Pipeline（核心管线）

**目标**：端到端可运行的改写管线 + 质量门控 + 断点续传

### 交付物

| 任务 | 说明 | 验收标准 |
| :--- | :--- | :--- |
| Eino Workflow | `compose.Workflow` DAG: Parse → Chunk → BatchLLM → QG → Merge → Export | 单个 .txt 文件端到端处理成功 |
| LLM Executor | OpenAI 兼容客户端 + RPM/TPM 限流 | 调用 LLM API 成功，限流生效 |
| BatchNode 并发 | MaxConcurrency 控制的并发 chunk 处理 | 20 并发 chunk 处理无竞态 |
| 质量门控 | 4 项基础检查（空输出、禁止模式、Markdown 注入、异常膨胀） | 9 个禁止模式全部检出 |
| ReAct Agent | `react.NewAgent` + retry_strict + split_rewrite + accept | 禁止模式触发后 Agent 成功修复 |
| 断点续传 | Redis CheckPointStore + Interrupt/Resume | 处理中 kill 进程 → 重启后从断点恢复 |
| SSE 完整流 | progress、quality_alert、recovery、complete、error、paused 事件 | 前端实时显示处理进度和质量告警 |
| Round 控制 | start/pause/resume API | 暂停后 CheckPoint 保存，恢复后继续 |
| 事务原子性 | Round 完成时多表原子写入 | 任意步骤失败后数据一致 |
| Advisory Lock | 同一 Session 并发保护 | 并发 start 第二个返回 409 |
| Prompt 模板 | 迁移 3 个 prompt 文件，实现 PromptBuilder | 中文 2 轮 + 英文 1 轮产出正确 |

### 验收场景

1. 上传中文 .txt → 2 轮改写 → 导出对比 → 评分 > 49
2. 上传英文 .docx → 1 轮改写 → 导出 .docx → 格式完整
3. 处理中 kill 进程 → 重启 → resume → 从断点继续
4. 人为注入禁止模式 → Quality Gate 触发 → ReAct 修复

---

## Phase 3: Quality & Performance（质量增强 + 性能优化）

**目标**：多 Provider 容灾 + 增强质量检查 + LLM 缓存 + 可观测性基线

### 交付物

| 任务 | 说明 | 验收标准 |
| :--- | :--- | :--- |
| Provider Chain | 多 Provider 优先级切换 + 独立熔断器 | Primary 挂掉后自动切 Fallback |
| 增强质量检查 | 长度偏差 + 结构破坏检查器 | 偏离 ±20% 和编号结构变化被检出 |
| LLM 响应缓存 | prompt hash → response 缓存 (24h TTL) | 相同 prompt 第二次命中缓存 |
| 物化视图 | `document_history_view` + 并发刷新 | 列表查询 < 50ms |
| OpenTelemetry | Tracer + Meter + 结构化日志关联 trace-id | Grafana 可追踪单个 chunk 全链路 |
| Prometheus 指标 | 业务 + LLM + 质量 + 基础设施指标 | Dashboard 可视化全部指标 |
| 告警规则 | 熔断器、失败率、延迟、CheckPoint 告警 | AlertManager 触发并通知 |
| SLI/SLO | 可用性、成功率、延迟、质量通过率 | Grafana SLO Dashboard 可查看 |
| 成本分析 | LLM token 消耗统计 + Provider 维度 | 成本 Dashboard 可查看日趋势 |
| 7 维评分 | 基于 checklist.md 的自动评分 | 每轮完成后输出 0-70 分评分 |

### 性能目标

| 指标 | 目标 | 测试方法 |
| :--- | :--- | :--- |
| 单 chunk P95 | ≤ 5s | 100 chunk 基准测试 |
| 并发 Session | ≥ 50 | 压力测试 |
| 历史列表查询 | < 50ms | 物化视图 + 索引 |
| CheckPoint 恢复 | < 3s | kill + resume 计时 |

---

## Phase 4: Advanced Features（高级功能）

**目标**：术语字典 + 批量处理 + Webhook + 前端迁移

### 交付物

| 任务 | 说明 | 验收标准 |
| :--- | :--- | :--- |
| 术语字典 | terminology.go + query_terminology Tool | Agent 查询术语后正确修复术语漂移 |
| 术语漂移检查 | TerminologyDriftChecker | 关键术语被替换时检出 |
| 批量处理 | 多文件上传 + 任务队列 + 优先级 | 一次上传 10 个文件，按序处理 |
| Webhook | 配置管理 + 事件分发 + 签名 + 重试 | round.completed 回调成功投递 |
| OpenAPI 文档 | Swagger/OpenAPI 3.0 规范文件 | Swagger UI 可查看全部端点 |
| Diff API | 输入输出逐 chunk 对比 | 前端展示每个 chunk 的修改差异 |
| 前端迁移 | React UI 对接新 Go API | 上传 → 处理 → 实时进度 → 下载 全流程 |
| 导出增强 | .docx 格式保真导出（段落样式） | 导出 .docx 保留原始样式 |

---

## Phase 5: Enterprise（企业级）

**目标**：认证授权 + 多租户 + 数据加密 + 生产部署

### 交付物

| 任务 | 说明 | 验收标准 |
| :--- | :--- | :--- |
| JWT 认证 | Login + Refresh + Middleware | 无 Token 请求返回 401 |
| API Key | 轻量认证 + 前缀标识 + bcrypt 存储 | CLI/Webhook 可用 API Key 访问 |
| RBAC | viewer/editor/admin/owner 角色 | editor 不能修改 config |
| 多租户 RLS | PostgreSQL Row-Level Security | 租户 A 看不到租户 B 的数据 |
| 分区表 | rounds 按季度分区 | 历史数据查询性能稳定 |
| 字段加密 | API Key KMS 加密存储 | 数据库直查看不到明文 |
| 审计增强 | 全部审计事件 + 审计日志查询 API | 管理员可查看操作历史 |
| K8s 部署 | Deployment + Service + Ingress + HPA | `kubectl apply` 成功，HPA 生效 |
| TLS | Ingress TLS 1.3 + cert-manager | HTTPS 可访问 |
| 数据保留 | Session 90 天自动清理 | 过期数据被定时任务删除 |

---

## 里程碑

| 里程碑 | Phase | 关键交付 |
| :--- | :--- | :--- |
| **M1: 骨架可运行** | Phase 1 | API 启动，文档解析 + 分块可用 |
| **M2: 端到端改写** | Phase 2 | 中文 .txt 2 轮改写完整通过，断点续传可用 |
| **M3: 生产可用** | Phase 3 | 多 Provider 容灾、完整可观测性、SLO 达标 |
| **M4: 功能完备** | Phase 4 | 批量处理、Webhook、前端迁移完成 |
| **M5: 企业就绪** | Phase 5 | 认证授权、多租户、K8s 部署、数据加密 |

## 技术债务管理

| 项 | 引入时机 | 解决时机 | 影响 |
| :--- | :--- | :--- | :--- |
| 无认证 | Phase 1 | Phase 5 | 开发期安全风险（内网可接受） |
| 单 Provider | Phase 2 | Phase 3 | LLM 不可用时无容灾 |
| 基础质量检查 | Phase 2 | Phase 3-4 | 术语漂移等问题无法检出 |
| 文件存储本地 | Phase 1 | Phase 5 | K8s 环境需 PV 或对象存储 |
| 无速率限制 | Phase 1 | Phase 3 | 公网暴露时有滥用风险 |
