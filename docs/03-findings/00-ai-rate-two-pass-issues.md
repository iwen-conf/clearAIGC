# AI 率与两轮润色问题清单

## 背景

本文记录 `docs/03-findings` 第一轮核查提出的问题，在 2026-04-20 代码补齐后的复核结果。目标不再是复述“最初哪里不对”，而是给出当前仓库里已经成立的事实、已经收敛的问题，以及仍需继续推进的缺口。

本次复核对应的代码面包括：

- `internal/domain/scorer/*`
- `internal/agent/coordinator.go`
- `internal/workflow/nodes/quality_gate.go`
- `internal/service/service.go`
- `web/src/components/workspace/card-review-panel.tsx`
- `web/src/components/workspace/upload-card.tsx`
- `web/src/components/workspace/session-header.tsx`

---

## 当前结论

### 1. 用户看到的已经不是“单一启发式 AI 率”

- 后端现在输出的是多信号复合分数：伪困惑度、burstiness、n-gram 异常和参考检测器分数。
- 若配置了外部检测器，系统会记录观测样本并做滚动校准，同时在 UI 中展示参考检测器、相关性、样本数和阈值。
- 若外部检测器不可用，系统会退化到离线模式，而不是伪装成第三方检测结论。

### 2. 分数已经进入优化决策，而不只是展示

- `Coordinator` 会先做句级打分，再挑出高风险句做 Best-of-N 候选比较。
- 候选排序已经把 AI 分、语义相似度和可读性一起纳入统一损失。
- `QualityGate` 也会直接拦截“润色后风险更高”的结果。

### 3. 中文不再是“必须固定两轮”

- 现在已有 `cn_single` 单轮模式。
- `cn` 仍然允许最多两轮，但第二轮是可选的，并且后台会在第一轮达标时提前停止。
- 前端文案已经调整为“先跑第一轮，可选继续第二轮精修”。

### 4. 评审界面已经能解释“为什么改”和“为什么没改”

- 卡片顶部展示校准后总分和当前模式。
- 分析面板展示四路信号雷达、信号条、参考检测器元数据和阈值。
- 句级折叠区会展示候选、loss、相似度、是否入选以及未入选原因。

---

## 历史问题复核

| 历史问题 | 当前状态 | 说明 |
| --- | --- | --- |
| `AI率` 被当成外部检测结果 | 大部分已修复 | 前端现在以“风险分”口径展示，并明确说明它是内部评分器结果；但接口字段名里仍保留 `aiRate`，内部命名尚未完全去历史包袱。 |
| 分数不参与优化决策 | 已修复 | 句级排序、候选 loss、质量门控和轮次提前停止都已依赖该分数或其校准结果。 |
| 学术中文被启发式误伤 | 部分缓解 | 已从单一路径启发式升级为多信号 + 校准 + 候选回退，但底层 `perplexity` 仍是轻量代理信号，不是真实小模型逐 token 计算。 |
| 前端把分数包装成检测结论 | 大部分已修复 | UI 已展示参考检测器、模式和校准信息，并补充风险说明；但该分数仍不应被解读为第三方官方概率。 |
| 中文固定两轮 | 已修复 | `cn_single` 可单轮，`cn` 也是“最多两轮 + 达标即停”，不再是硬编码强制跑满。 |
| 第二轮没有收益判断 | 已修复 | 第一轮完成后会按目标分判断是否继续；句级和整段两个层面都有 no-improvement 回退。 |
| 产品话术与运行逻辑不一致 | 大部分已修复 | 上传区、会话头和评审面板已改口径；剩余问题主要在部分历史文档和内部命名。 |
| coordinator 覆盖轮次 prompt | 已调整为显式协同 | 现在轮次 prompt 会进入 worker guidance，协调器不再是脱离轮次意图的另一套黑盒路径。 |

---

## 当前仍然存在的限制

### 1. 校准器已经可用，但不是最终形态

- 当前实现是持久化的滚动线性校准，而不是原方案中提到的 isotonic regression。
- 这已经足够支撑“离线基线 / 预热 / 在线校准”三种模式切换，但严格的单调校准精度仍有后续提升空间。

### 2. 评分器结构已经到位，但底层信号仍然偏轻量

- `perplexity` 是基于当前风险代理的轻量估计，不是本地小模型逐 token PPL。
- `burstiness` 和可读性是纯统计量，`n-gram` 异常来自仓库内规则特征表，而不是版本化语料库。
- 因此现在更适合称为“可解释风险评分器”，而不是“高保真检测器复刻”。

### 3. 参考检测器依赖配置与样本积累

- 外部检测器是可选能力，不配置时系统会自动回退到离线模式。
- 即使已经配置，相关性和阈值也要在样本数量达到下限后才会进入稳定校准态。
- 本轮补丁已增加 24 小时内存缓存，避免对同一文本反复调用参考检测器。

---

## 证据文件

- `internal/domain/scorer/composite.go`
- `internal/domain/scorer/external.go`
- `internal/domain/scorer/calibrator.go`
- `internal/agent/coordinator.go`
- `internal/workflow/nodes/quality_gate.go`
- `internal/service/service.go`
- `web/src/components/workspace/card-review-panel.tsx`
- `web/src/components/workspace/upload-card.tsx`
- `web/src/components/workspace/session-header.tsx`

---

## 结论

`00` 号 findings 最初指出的核心问题已经不再是“系统根本没有解”，而是进入了“口径、校准精度和轻量实现边界需要继续收敛”的阶段。

也就是说，今天仓库里的真实情况已经变成：

1. 有一个会参与决策的多信号评分器。
2. 有一个带参考检测器和持久化状态的校准层。
3. 有一个句级 Best-of-N + 质量门控 + 提前停止的闭环。
4. 仍然需要继续提高底层检测信号和校准方法的保真度。
