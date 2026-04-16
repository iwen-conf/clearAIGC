# clearAIGC — 可观测性与运维

## 1. 三大支柱

### 1.1 分布式追踪 (Tracing)

基于 OpenTelemetry，每个 Workflow 节点自动生成 Span：

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

// Eino Callback 自动注入 Span
handler := &callbacks.HandlerBuilder{
    OnStartFn: func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        tracer := otel.Tracer("clearaigc")
        ctx, span := tracer.Start(ctx, "node."+info.Name)
        span.SetAttributes(
            attribute.String("node.name", info.Name),
            attribute.String("session.id", getSessionID(ctx)),
        )
        return ctx
    },
    OnEndFn: func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
        span := trace.SpanFromContext(ctx)
        span.End()
        return ctx
    },
}
```

Trace 结构：

```
[API Request] POST /sessions/{id}/start
  └── [Workflow] clearaigc-pipeline
      ├── [Node] parse (12ms)
      ├── [Node] chunk (8ms)
      ├── [Node] batch_llm (4500ms)
      │   ├── [Chunk] p0_c0 → LLM call (320ms) [provider=openai]
      │   ├── [Chunk] p0_c1 → LLM call (280ms) [provider=openai]
      │   └── [Chunk] p1_c0 → LLM call (350ms) [provider=openai]
      ├── [Node] quality_gate (5ms)
      │   └── [Recovery] p1_c0 → ReAct Agent (1200ms)
      │       ├── [Tool] retry_with_strict_prompt (800ms)
      │       └── [Tool] retry_with_strict_prompt (400ms)
      ├── [Node] merge (3ms)
      └── [Node] export (15ms)
```

### 1.2 指标 (Metrics)

Prometheus 指标通过 `/api/v1/metrics` 暴露：

**业务指标**：

| 指标名 | 类型 | 标签 | 说明 |
| :--- | :--- | :--- | :--- |
| `clearaigc_sessions_total` | Counter | `status`, `profile` | 会话总数 |
| `clearaigc_sessions_active` | Gauge | `profile` | 当前活跃会话数 |
| `clearaigc_rounds_total` | Counter | `status`, `round_number` | 轮次总数 |
| `clearaigc_chunks_processed_total` | Counter | `status` | chunk 处理总数 |
| `clearaigc_chunk_duration_seconds` | Histogram | `node` | 每个 chunk 的处理耗时 |
| `clearaigc_document_size_bytes` | Histogram | `format` | 上传文档大小分布 |

**LLM 指标**：

| 指标名 | 类型 | 标签 | 说明 |
| :--- | :--- | :--- | :--- |
| `clearaigc_llm_requests_total` | Counter | `provider`, `status` | LLM 调用总数 |
| `clearaigc_llm_tokens_total` | Counter | `provider`, `type` | Token 消耗 (prompt/completion) |
| `clearaigc_llm_latency_seconds` | Histogram | `provider` | LLM 调用延迟 |
| `clearaigc_llm_cost_usd` | Counter | `provider`, `model` | LLM 调用费用估算 |
| `clearaigc_provider_failover_total` | Counter | `from`, `to` | Provider 切换次数 |

**质量指标**：

| 指标名 | 类型 | 标签 | 说明 |
| :--- | :--- | :--- | :--- |
| `clearaigc_quality_checks_total` | Counter | `check_type`, `passed` | 质量检查结果 |
| `clearaigc_quality_score` | Histogram | `round_number`, `profile` | 质量评分分布 |
| `clearaigc_react_recoveries_total` | Counter | `tool`, `success` | ReAct 恢复次数 |
| `clearaigc_react_steps_total` | Histogram | `success` | ReAct Agent 步数分布 |

**基础设施指标**：

| 指标名 | 类型 | 标签 | 说明 |
| :--- | :--- | :--- | :--- |
| `clearaigc_checkpoint_operations_total` | Counter | `operation` | CheckPoint 读写次数 |
| `clearaigc_circuit_breaker_state` | Gauge | `provider` | 熔断器状态 (0=closed, 1=open, 2=half-open) |
| `clearaigc_webhook_deliveries_total` | Counter | `event`, `status` | Webhook 投递状态 |
| `clearaigc_cache_hit_total` | Counter | `cache_type` | 缓存命中次数 |
| `clearaigc_cache_miss_total` | Counter | `cache_type` | 缓存未命中次数 |

### 1.3 结构化日志 (Logging)

使用 Go 1.21+ 的 `slog`，关联 trace-id：

```go
slog.Info("chunk processed",
    "trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
    "session_id", sessionID,
    "round", roundNumber,
    "chunk_id", chunkID,
    "duration_ms", elapsed.Milliseconds(),
    "input_chars", len(input),
    "output_chars", len(output),
    "quality_passed", report.AllPassed,
    "provider", providerName,
    "tokens_used", tokensUsed,
)
```

日志级别策略：

| 级别 | 用途 | 示例 |
| :--- | :--- | :--- |
| `ERROR` | 不可恢复的失败 | LLM 所有 Provider 不可用、数据库连接断开 |
| `WARN` | 可恢复的异常 | Quality Gate 失败（触发 ReAct）、限流触发、熔断器状态变更、Provider 切换 |
| `INFO` | 关键生命周期事件 | Session/Round 创建/完成、chunk 完成、CheckPoint 操作 |
| `DEBUG` | 调试细节 | Prompt 构建内容、LLM 请求/响应、分块详情、缓存命中 |

## 2. SLI/SLO 定义

### 2.1 Service Level Indicators

| SLI | 计算方式 | 采集间隔 |
| :--- | :--- | :--- |
| 可用性 | `1 - (5xx responses / total responses)` | 1m |
| 处理成功率 | `completed sessions / (completed + failed sessions)` | 5m |
| chunk 处理延迟 | `histogram_quantile(0.95, clearaigc_chunk_duration_seconds)` | 1m |
| LLM 调用延迟 | `histogram_quantile(0.95, clearaigc_llm_latency_seconds)` | 1m |
| 质量通过率 | `passed checks / total checks` | 5m |

### 2.2 Service Level Objectives

| SLO | 目标 | 告警阈值 | 窗口 |
| :--- | :--- | :--- | :--- |
| API 可用性 | ≥ 99.5% | < 99% 告警 | 30d rolling |
| 处理成功率 | ≥ 95% | < 90% 告警 | 7d rolling |
| chunk P95 延迟 | ≤ 5s | > 10s 告警 | 5m |
| LLM P95 延迟 | ≤ 3s | > 5s 告警 | 5m |
| 质量通过率 | ≥ 80% | < 70% 告警 | 1h |

## 3. 告警规则

```yaml
groups:
  - name: clearaigc-critical
    rules:
      - alert: LLMCircuitBreakerOpen
        expr: clearaigc_circuit_breaker_state > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "LLM 熔断器打开 ({{ $labels.provider }})"
          
      - alert: CheckPointStoreUnavailable
        expr: rate(clearaigc_checkpoint_operations_total{operation="error"}[5m]) > 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Redis CheckPointStore 不可用"

      - alert: AllProvidersDown
        expr: count(clearaigc_circuit_breaker_state == 0) == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "所有 LLM Provider 不可用"
          
  - name: clearaigc-warning
    rules:
      - alert: HighChunkFailureRate
        expr: >
          rate(clearaigc_quality_checks_total{passed="false"}[5m])
          / rate(clearaigc_quality_checks_total[5m]) > 0.2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Quality Gate 失败率超过 20%"
          
      - alert: HighLLMLatency
        expr: histogram_quantile(0.95, clearaigc_llm_latency_seconds) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "LLM P95 延迟超过 10s"
          
      - alert: HighReActSteps
        expr: histogram_quantile(0.95, clearaigc_react_steps_total) > 8
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "ReAct Agent P95 步数超过 8 步"
          
      - alert: ProviderFailoverFrequent
        expr: rate(clearaigc_provider_failover_total[5m]) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Provider 频繁切换"

      - alert: WebhookDeliveryFailure
        expr: >
          rate(clearaigc_webhook_deliveries_total{status="failed"}[5m])
          / rate(clearaigc_webhook_deliveries_total[5m]) > 0.3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Webhook 投递失败率超过 30%"
```

## 4. Grafana Dashboard

### 4.1 Overview Dashboard

```
┌─────────────────────────────────────────────────────────┐
│ Row 1: Real-time Overview                                │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│ │ Active   │ │ Chunks/s │ │ LLM      │ │ Quality  │    │
│ │ Sessions │ │          │ │ Tokens/m │ │ Pass %   │    │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘    │
├─────────────────────────────────────────────────────────┤
│ Row 2: LLM Performance                                   │
│ ┌────────────────────────┐ ┌────────────────────────┐   │
│ │ LLM Latency (P50/P95)  │ │ Token Usage Over Time  │   │
│ │ by provider             │ │ by provider            │   │
│ └────────────────────────┘ └────────────────────────┘   │
├─────────────────────────────────────────────────────────┤
│ Row 3: Quality & Recovery                                │
│ ┌────────────────────────┐ ┌────────────────────────┐   │
│ │ Quality Check Results   │ │ ReAct Recovery Rate    │   │
│ │ (by check_type)         │ │ (by tool + success)    │   │
│ └────────────────────────┘ └────────────────────────┘   │
├─────────────────────────────────────────────────────────┤
│ Row 4: Quality Score Distribution                        │
│ ┌──────────────────────────────────────────────────┐    │
│ │ Score Heatmap (by round, 0-70 range)             │    │
│ └──────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────┤
│ Row 5: Infrastructure                                    │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│ │ PG Conn  │ │ Redis    │ │ Circuit  │ │ CheckPt  │    │
│ │ Pool     │ │ Memory   │ │ Breaker  │ │ Ops/s    │    │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘    │
└─────────────────────────────────────────────────────────┘
```

### 4.2 Cost Analysis Dashboard

```
┌─────────────────────────────────────────────────────────┐
│ Row 1: Cost Summary                                      │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│ │ Today's  │ │ Avg Cost │ │ Recovery │ │ Cache    │    │
│ │ LLM Cost │ │ /Session │ │ Token %  │ │ Hit Rate │    │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘    │
├─────────────────────────────────────────────────────────┤
│ Row 2: Token Usage Breakdown                             │
│ ┌────────────────────────┐ ┌────────────────────────┐   │
│ │ Tokens by Provider      │ │ Tokens: Main vs ReAct  │   │
│ │ (stacked area)          │ │ (stacked area)         │   │
│ └────────────────────────┘ └────────────────────────┘   │
├─────────────────────────────────────────────────────────┤
│ Row 3: Cost Trends                                       │
│ ┌──────────────────────────────────────────────────┐    │
│ │ Daily LLM Cost (7d trend, by provider)           │    │
│ └──────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

## 5. 部署架构

### 5.1 Docker Compose（本地开发）

```yaml
services:
  app:
    build: .
    ports: ["8080:8080"]
    environment:
      - DATABASE_URL=postgres://clearaigc:pass@postgres:5432/clearaigc
      - REDIS_URL=redis://redis:6379/0
      - LLM_PROVIDERS=openai:https://api.openai.com/v1
      - OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
    depends_on: [postgres, redis]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health/live"]
      interval: 10s
      timeout: 5s
      retries: 3
    
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: clearaigc
      POSTGRES_USER: clearaigc
      POSTGRES_PASSWORD: pass
    volumes: ["pgdata:/var/lib/postgresql/data"]
    ports: ["5432:5432"]
    
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    volumes: ["./deploy/otel-config.yaml:/etc/otelcol-contrib/config.yaml"]
    ports: ["4317:4317", "4318:4318"]
    
  prometheus:
    image: prom/prometheus:latest
    volumes: ["./deploy/prometheus.yml:/etc/prometheus/prometheus.yml"]
    ports: ["9090:9090"]
    
  grafana:
    image: grafana/grafana:latest
    volumes: ["./deploy/grafana/dashboards:/var/lib/grafana/dashboards"]
    ports: ["3000:3000"]

volumes:
  pgdata:
```

### 5.2 Kubernetes（生产）

```
┌─────────────────────────────────────────────┐
│ Kubernetes Cluster                           │
│                                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│  │ App Pod │  │ App Pod │  │ App Pod │     │
│  │ (HPA)   │  │ (HPA)   │  │ (HPA)   │     │
│  └────┬────┘  └────┬────┘  └────┬────┘     │
│       │             │             │          │
│  ┌────┴─────────────┴─────────────┴────┐    │
│  │         Service (ClusterIP)          │    │
│  └──────────────────┬──────────────────┘    │
│                     │                        │
│  ┌──────────────────┴──────────────────┐    │
│  │         Ingress (TLS 1.3)           │    │
│  └─────────────────────────────────────┘    │
│                                              │
│  ┌──────────────┐  ┌──────────────┐         │
│  │ PostgreSQL   │  │ Redis        │         │
│  │ (StatefulSet │  │ (StatefulSet │         │
│  │  or RDS)     │  │  or ElastiC) │         │
│  └──────────────┘  └──────────────┘         │
│                                              │
│  ┌──────────────┐  ┌──────────────┐         │
│  │ OTel         │  │ Prometheus   │         │
│  │ Collector    │  │ + Grafana    │         │
│  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────┘
```

HPA 配置：

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: clearaigc
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: clearaigc
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Pods
      pods:
        metric:
          name: clearaigc_sessions_active
        target:
          type: AverageValue
          averageValue: 20
```

### 5.3 健康检查

```go
func (h *HealthHandler) Check(c *gin.Context) {
    checks := map[string]string{}
    
    // PostgreSQL
    if err := h.db.Exec("SELECT 1").Error; err != nil {
        checks["postgres"] = "unhealthy: " + err.Error()
    } else {
        checks["postgres"] = "healthy"
    }
    
    // Redis
    if err := h.redis.Ping(ctx).Err(); err != nil {
        checks["redis"] = "unhealthy: " + err.Error()
    } else {
        checks["redis"] = "healthy"
    }
    
    // LLM (cached probe, 不每次真调)
    checks["llm"] = h.llmStatus.Load().(string)
    
    allHealthy := true
    for _, v := range checks {
        if v != "healthy" {
            allHealthy = false
        }
    }
    
    status := 200
    if !allHealthy {
        status = 503
    }
    c.JSON(status, gin.H{
        "status":    checks,
        "uptime":    time.Since(h.startTime).String(),
        "version":   h.version,
        "buildTime": h.buildTime,
    })
}
```

### 5.4 Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /clearaigc ./cmd/server

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /clearaigc /usr/local/bin/clearaigc
COPY prompts/ /app/prompts/
EXPOSE 8080
ENTRYPOINT ["clearaigc"]
```
