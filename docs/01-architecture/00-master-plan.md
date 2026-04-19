# Naturalize — 总体设计

## 项目定位

Naturalize 是一个面向中英文学术论文、技术文档和课程作业的 **AI 痕迹消除平台**。它通过多轮结构化改写，将模板化、机械化的 AI 生成文本转化为自然的人类写作风格，同时严格保持原文的事实、术语、逻辑和结构完整性。

系统采用 **确定性工作流 + 受限 ReAct Agent** 的混合架构，在保证输出可预测性的同时，利用 AI Agent 能力实现质量自愈。

## 核心能力

### 多轮结构化改写

| 模式 | 轮次 | 策略 | 度量 |
| :--- | :--- | :--- | :--- |
| 中文 | 双轮/单轮 | 包含回退机制的 R1: 词汇降维 + 句式扩展 → R2: AI 套话清除 + 节奏重构 | 字符 |
| 英文 | 单轮 | 模板打破 + 自然粗糙度注入 + 学术语域保持 | 单词 |

### 智能分块引擎

4 级渐进分块策略，确保语义完整性：

```
Level 1: 段落边界 ──── 自然段落作为最小处理单元
Level 2: 句子边界 ──── 超长段落按句末标点（。！？.!?）切分
Level 3: 标点边界 ──── 超长句子按句中标点（，；、,;）切分
Level 4: 硬截断   ──── 仍超长则硬截断（保护术语/编号不被截断）
```

每块上限可配置（默认 850 字符/单词），支持运行时动态调整。

### 质量门控 + 自愈

- **确定性规则检查**：空输出、禁止模式（9 个正则）、Markdown 注入、异常膨胀
- **增强检查**：长度偏差、结构破坏、术语漂移
- **ReAct Agent 自动修复**：80%+ 的门控失败由 Agent 自动修复，减少人工干预
- **7 维质量评分**：直接性、节奏、信任度、真实性、精炼度、语义保真、纯净输出（满分 70）

### 断点续传

基于 Eino CheckPoint + Redis 实现任意时刻暂停/恢复：

- 用户主动暂停 / 服务重启 / LLM 中断 → 均可无损恢复
- 跨服务器迁移不丢失进度
- 每个 chunk 完成后自动保存快照

### 实时进度流

SSE + Redis Pub/Sub 推送每个 chunk 的处理状态到前端：

- `progress` — chunk 级进度和百分比
- `quality_alert` — 质量门控失败告警
- `recovery` — ReAct Agent 修复结果
- `complete` — 轮次完成统计
- `error` — 可恢复/不可恢复错误

### 多 LLM Provider 容灾

```
Primary Provider ──→ [Success] ──→ Continue
       │
       └──→ [Fail/Timeout] ──→ Circuit Breaker ──→ Fallback Provider
                                      │
                                      └──→ [All Down] ──→ Checkpoint + Pause
```

- 主 Provider + N 个 Fallback，优先级链式切换
- 每个 Provider 独立的速率限制（RPM + TPM Token Bucket）
- 熔断器（连续 5 次失败触发，60s 冷却，3 次探测恢复）
- Provider 响应缓存，断点续传恢复时避免重复调用

### 批量文件处理

- 单次上传多个文件，自动创建独立 Session
- 批量任务队列，按优先级和资源使用率调度
- 批量进度汇总推送

### Webhook 通知

```json
{
  "event": "round.completed",
  "session_id": "550e8400-...",
  "round": 1,
  "score_total": 62,
  "download_url": "/api/v1/sessions/550e.../export?round=1"
}
```

- Session 完成、轮次完成、质量告警等事件的外部回调
- 支持自定义 URL + Secret 签名验证
- 重试策略：指数退避，最多 3 次

## 核心架构决策

### 确定性外层 Workflow + 局部内层 ReAct

主管线是严格的 DAG（Eino `compose.Workflow`），每个步骤的行为完全可预测。LLM 仅在两个位置介入：

1. **标准调用**：执行 chunk 改写（无 Agent 行为，纯 prompt in → text out）
2. **ReAct Agent**：修复未通过质量门控的 chunk（消防员角色，MaxStep=10）

这个决策拒绝了让 LLM 自由导航整个处理流程的方案——那会导致不可预测的输出、高昂的 token 成本和系统脆弱性。

### 无状态 API + 有状态存储分离

```
┌─────────────────────────────────────────────────────────┐
│  Stateless API Layer (Gin)                               │
│  ┌──────┐ ┌──────┐ ┌──────┐                            │
│  │ Pod1 │ │ Pod2 │ │ PodN │  ← HPA auto-scale          │
│  └──┬───┘ └──┬───┘ └──┬───┘                            │
│     └────────┴────────┘                                  │
│              │                                           │
├──────────────┼───────────────────────────────────────────┤
│  Stateful    │                                           │
│  ┌───────────┴───────────┐  ┌──────────────────────┐    │
│  │  PostgreSQL 16+       │  │  Redis 7+            │    │
│  │  (Source of Truth)    │  │  (Cache + CheckPoint │    │
│  │  Sessions, Rounds,    │  │   + Pub/Sub)         │    │
│  │  Manifests, Quality   │  │                      │    │
│  └───────────────────────┘  └──────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

API 层完全无状态，支持 K8s HPA 水平扩缩。所有持久化状态在 PostgreSQL，加速层在 Redis。Redis 丢失不影响正确性。

### 可插拔质量检查

质量检查以 `Checker` 接口注册，支持运行时扩展：

```go
type Checker interface {
    Type() CheckType
    Check(ctx context.Context, input, output string) (passed bool, reason string)
}
```

内置 4 项基础检查 + 3 项增强检查，可通过配置启用/禁用，或注册自定义检查器。

## 技术栈

| 层 | 选型 | 理由 |
| :--- | :--- | :--- |
| 语言 | Go 1.22+ | 高并发、强类型、编译期安全、原生协程模型 |
| 工作流引擎 | Eino `compose.Workflow` | DAG 编排、字段级映射、原生 CheckPoint、Callback |
| Agent 框架 | Eino `flow/agent/react` | ReAct 循环、Tool Calling、MaxStep 控制 |
| 并发批处理 | Eino `BatchNode` | MaxConcurrency 信号量、Interrupt 收集 |
| 断点续传 | Eino `Interrupt & CheckPoint` | 自动序列化/恢复 Graph 状态 |
| 持久化 | PostgreSQL 16+ | JSONB、Advisory Lock、物化视图、RLS、分区表 |
| 缓存/消息 | Redis 7+ | CheckPointStore、Session 缓存、Pub/Sub、LLM 缓存 |
| HTTP 框架 | Gin | REST + SSE + WebSocket + Middleware 链 |
| 可观测性 | OpenTelemetry + Prometheus + Grafana | 分布式追踪、指标、结构化日志、告警 |
| 部署 | Docker + Kubernetes | 水平扩展、滚动更新、HPA 自动扩缩 |
| 数据库迁移 | golang-migrate | 版本化 SQL 迁移、回滚支持 |
| 配置管理 | Viper | 多源配置（文件、环境变量、远程）、热加载 |

## Eino 框架映射

| Naturalize 概念 | Eino API | 说明 |
| :--- | :--- | :--- |
| 主处理管线 | `compose.Workflow` | DAG，`NodeTriggerMode=AllPredecessor`，不支持循环 |
| ReAct 修复 Agent | `react.NewAgent` | 基于 `compose.Graph` 的循环，`MaxStep=10` |
| chunk 并发处理 | `BatchNode` | `MaxConcurrency` 控制，原生 Interrupt/Resume |
| 断点续传 | `compose.CheckPointStore` | Redis 实现，KV 接口 |
| Agent 嵌入管线 | `compose.AnyLambda` | 将 Agent 包装为 Lambda 节点 |
| 进度回调 | `callbacks.OnStart/OnEnd/OnError` | 跨切面事件注入 |
| 多 Agent 协作 | ADK (`SequentialAgent`, `ParallelAgent`) | 未来扩展：多维度质量评估 |

## 行为契约

以下是系统必须严格遵守的不可变规则：

| 规则 | 说明 |
| :--- | :--- |
| Prompt Profile 映射 | cn: 2 轮 (char 度量), cn_single: 1 轮 (char 度量), en: 1 轮 (word 度量) |
| 轮次顺序不可逆 | Round 1 → Round 2，不可跳过、合并或反转 |
| 输出契约注入 | 每个 chunk prompt 必须附加 `[OUTPUT CONTRACT]` 文本块 |
| Prompt 格式 | `[ROUND N]\n[CHUNK id]\n\n{prompt}\n\n{contract}\n\n[INPUT TEXT]\n{text}` |
| 4 级分块 | 段落 → 句子边界 → 标点边界 → 硬截断，每块上限可配置（默认 850） |
| Quality Gate 基础检查 | 空输出、禁止模式（9 个）、Markdown 注入、异常膨胀、高启发式风险分（AI 率）拦截 |

> **进阶架构说明**：当前系统通过启发式规则和动态回退机制（Quality Gate 拦截 + Recovery Agent 定向靶向纠错）控制 AI 生成痕迹。为进一步提升降重能力并对标外部真实 AI 检测器，长期架构将向 **多维统计信号融合 (Perplexity, Burstiness, 功能词 KL 散度等)**、**句级 Best-of-N 采样** 以及 **用户 Style Anchor (个性化风格对齐)** 演进。详细的理论推导与工程设计草案参见 [Claude 解决 AI 率优化问题的思路](../03-findings/01-claude-ai-rate-solution.md) 及 [统计学与跨学科方法](../03-findings/02-statistical-methods.md)。
| 轮次幂等 | 同一 session + round_number 的重复写入替换而非追加 |
| 结构保留 | 改写后必须保持原段落顺序、编号结构、术语不变 |
| 零添加 | 不添加新事实、数据、引用或结论 |
| 输出纯净 | 仅输出改写文本，禁止 meta-commentary、建议、替代方案 |
