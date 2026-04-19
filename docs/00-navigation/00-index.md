# Naturalize — 文档中心

**Naturalize** 是一个高性能的中英文学术文档 AI 痕迹消除平台，基于 Go + Eino 框架构建。

核心架构：**确定性工作流**（DAG 管线，保证输出可预测）+ **受限 ReAct Agent**（质量自愈，80%+ 自动修复率）。

## 快速索引

### 架构设计

| 序号 | 文档 | 内容 |
| :--- | :--- | :--- |
| 1 | [总体设计](../01-architecture/00-master-plan.md) | 项目定位、核心能力、技术栈、行为契约 |
| 2 | [架构愿景](../01-architecture/01-vision.md) | 系统全景、数据流、Provider 容灾链、非功能性需求 |
| 3 | [模块设计](../01-architecture/02-modules.md) | 包结构、领域模型、接口定义、依赖规则 |
| 4 | [Workflow 与 ReAct](../01-architecture/03-workflow-react.md) | DAG 管线编排、可插拔质量门控、Agent 恢复策略、断点续传 |
| 5 | [并发、存储与限流](../01-architecture/04-infra.md) | PostgreSQL Schema、Redis 多角色、BatchNode 并发、Provider Chain |
| 6 | [API 规范](../01-architecture/05-api.md) | REST 端点、SSE 事件流、Webhook 回调、错误码 |
| 7 | [可观测性与运维](../01-architecture/06-observability.md) | 指标体系、SLI/SLO、告警规则、Grafana Dashboard、部署架构 |
| 8 | [安全与多租户](../01-architecture/07-security.md) | JWT/API Key 认证、RBAC、RLS 数据隔离、审计日志 |

### 交付计划

| 序号 | 文档 | 内容 |
| :--- | :--- | :--- |
| 9 | [交付路线图](../02-roadmap/00-phases.md) | 5 阶段计划、验收标准、里程碑、技术债务管理 |

## 目录结构

```
docs/
├── 00-navigation/        # 入口与索引
│   └── 00-index.md       # ← 你在这里
├── 01-architecture/      # 系统架构设计（8 份文档）
│   ├── 00-master-plan.md
│   ├── 01-vision.md
│   ├── 02-modules.md
│   ├── 03-workflow-react.md
│   ├── 04-infra.md
│   ├── 05-api.md
│   ├── 06-observability.md
│   └── 07-security.md
└── 02-roadmap/           # 交付计划
    └── 00-phases.md
```

## 核心特性一览

| 特性 | 说明 |
| :--- | :--- |
| 多轮改写 | 中文 2 轮（词汇降维 + AI 套话清除）/ 英文 1 轮 |
| 智能分块 | 4 级渐进策略（段落 → 句子 → 标点 → 硬截断） |
| 质量自愈 | 可插拔质量门控 + ReAct Agent 自动修复 |
| 断点续传 | Eino CheckPoint + Redis，服务重启不丢进度 |
| 实时进度 | SSE + Redis Pub/Sub，chunk 级进度推送 |
| Provider 容灾 | 多 LLM Provider 链式切换 + 独立熔断器 |
| 批量处理 | 多文件上传 + 任务队列 + 优先级调度 |
| Webhook | Session/Round 完成事件外部回调 + HMAC 签名 |
| 可观测性 | OpenTelemetry 追踪 + Prometheus 指标 + Grafana |
| 企业安全 | JWT + RBAC + RLS 多租户 + 审计日志 |
