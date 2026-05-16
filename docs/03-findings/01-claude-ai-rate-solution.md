# Claude 解决 AI 率优化问题的思路

> 本文档已经从“理想方案草稿”改为“当前仓库已落地方案 + 明确未落地项”的实现说明。目标不是再画一次大图，而是把 `docs/03-findings` 与真实代码重新对齐。

---

## 1. 当前已经落地的架构

### 1.1 评分层

当前评分层已经从单一路径启发式升级为 `internal/domain/scorer/` 包，核心文件如下：

- `composite.go`：汇总总分、目标分、阈值和四路信号。
- `signals.go`：句长统计、burstiness、可读性和 n-gram 特征提取。
- `external.go`：OpenAI-compatible 参考检测器接入，包含速率受控调用和 24h 内存缓存。
- `calibrator.go`：滚动校准状态、阈值估计和持久化。
- `embeddings.go`：OpenAI-compatible `/v1/embeddings` 语义相似度比较。

当前四路信号仍然是：

| 信号 | 当前实现 | 备注 |
| --- | --- | --- |
| Perplexity-like | 轻量代理值 | 不是本地小模型逐 token PPL，而是当前仓库可稳定运行的代理实现。 |
| Burstiness | 句长方差 / 节奏统计 | 已进入总分和质量门控。 |
| n-gram anomaly | 基于现有特征表和命中项 | 来自 `IdentifyAIFeatures` / 禁用词表，不是独立语料库服务。 |
| Detector reference | 可选外部检测器 | 通过 OpenAI-compatible chat endpoint 接入，结果可参与校准。 |

### 1.2 校准层

校准器已经真实落地，但采用的是更务实的第一版实现：

- 状态持久化到 `var/data/calibration/`。
- 观测样本记录 `internalTotal + detectorScore + recordedAt`。
- 达到最小样本量前进入 `warming_up`。
- 达到样本门槛后使用滚动线性拟合，输出：
  - `Score`
  - `Correlation`
  - `Green / Red` 阈值
  - `Mode`
  - `SampleCount`

也就是说，当前系统已经具备“离线基线 -> 预热 -> 在线校准”的完整状态机，但不是原文草案里的 isotonic regression。

### 1.3 目标驱动改写层

句级改写闭环已经在 `internal/agent/coordinator.go` 中落地：

- 先对句子打分并排序。
- 只处理 top-K 高风险句。
- 为每句构建多候选路径：
  - lexical
  - syntax
  - lexical + syntax
  - original fallback
- 用统一 loss 选择候选，loss 已综合：
  - AI 分
  - 语义距离
  - 可读性惩罚
  - 无收益回退奖励

### 1.4 验证与门控层

`internal/workflow/nodes/quality_gate.go` 已扩展为多道门：

- 空输出
- 禁止模式
- Markdown 注入
- 异常膨胀
- 事实不变量
- 结构破坏
- 命名实体漂移
- 语义相似度 / semantic-anchor fallback
- 术语漂移
- 低 burstiness 回归
- 可读性回归
- AI 分升高拦截

这意味着“润色后 AI 分更高但仍然放行”的旧行为已经不再成立。

### 1.5 轮次与停止条件

轮次控制已经不是固定串行：

- `cn_single`：单轮。
- `cn`：最多两轮，但第一轮达标时直接停止。
- `en`：单轮。

后台通过 `shouldStopAfterRound(...)` 用当前评分器判断是否需要第二轮；前端只消费后台给出的 `totalRounds / nextRound / canStartNextRound`，不再自己猜轮次。

### 1.6 评审界面

`web/src/components/workspace/card-review-panel.tsx` 现在已经展示：

- 校准后总分
- 原始总分
- 目标分
- 四路信号雷达
- 每路信号条和说明
- 参考检测器、模式、相关性、样本数、阈值
- 质量门控失败项
- 句级决策与候选对比

这部分已经从“单个标签”升级成可解释的分析面板。

---

## 2. 相比原草案，哪些是有意调整的

### 2.1 没有拆成 `perplexity.go` / `burstiness.go` / `ngram.go`

原草案把信号拆成独立文件是为了表达职责边界。当前实现为了控制改动面，把轻量统计逻辑集中在 `signals.go`，而不是机械追求文件名一一对应。

结论：职责已经落地，文件拆分没有强制必要。

### 2.2 没有直接上本地 PPL 小模型

原草案中的本地 PPL 模型会显著增加依赖、部署复杂度和 CI 不确定性。当前仓库先采用轻量代理信号，优先解决：

- 分数进入决策
- 候选可比较
- 校准状态可持久化
- UI 可解释

结论：这是工程上刻意选择的渐进式实现，不是漏做。

### 2.3 没有采用 isotonic regression

当前 `calibrator.go` 用滚动线性回归 + 分位阈值，是为了先把“有观测、有状态、有模式切换”的链路做通。它比 placeholder 更真实，但还不是最终校准精度上限。

结论：校准层已经可用，但方法仍可升级。

### 2.4 语义相似度采用 OpenAI-compatible embeddings，而不是本地 SimCSE / bge-m3

原方案把 SimCSE / bge-m3 作为主要例子。现在仓库实际落地的是：

- 优先使用远程 embedding API
- 不可用时退化到 semantic-anchor

结论：这和原方案的目标一致，只是把部署路径改成了更适合当前仓库的远程依赖形式。

---

## 3. 当前仍未完全落地的部分

### 3.1 高保真检测信号仍然偏轻量

- `perplexity` 不是语言模型真值。
- `n-gram` 仍依赖仓库内规则表，而不是版本化 AI 语料库。
- `external` 虽然可接入真实服务，但默认并非强依赖。

### 3.2 校准方法仍然是第一版

- 还没有 isotonic regression。
- 还没有专门的离线标注集训练管线。
- 相关性监控目前体现在运行态元数据中，还未形成独立 dashboard。

### 3.3 术语还残留少量历史命名

- 部分接口字段仍叫 `aiRate`。
- 文档和代码中的“AI 率”与“风险分”口径已经明显收敛，但还没有做到全仓库统一重命名。

---

## 4. 代码映射

| 能力 | 当前代码入口 |
| --- | --- |
| 复合评分 | `internal/domain/scorer/composite.go` |
| 统计信号 | `internal/domain/scorer/signals.go` |
| 外部检测器 + 缓存 | `internal/domain/scorer/external.go` |
| 校准状态 | `internal/domain/scorer/calibrator.go` |
| 语义向量相似度 | `internal/domain/scorer/embeddings.go` |
| 句级 Best-of-N | `internal/agent/coordinator.go` |
| 质量门控 | `internal/workflow/nodes/quality_gate.go` |
| 轮次提前停止 | `internal/service/service.go` |
| 评审分析面板 | `web/src/components/workspace/card-review-panel.tsx` |

---

## 5. 结论

`01` 号 findings 最重要的变化是：它已经不再只是“建议怎么做”，而是可以明确说出“现在仓库已经做到了什么”。

当前仓库真实具备的链路是：

1. 多信号评分。
2. 可选参考检测器与持久化校准。
3. 句级 Best-of-N 改写。
4. 语义 / 事实 / 结构 / 可读性的质量门控。
5. 后端驱动的提前停止与可选第二轮。
6. 前端可解释分析面板。

剩下的工作主要是“提高检测信号和校准方法的保真度”，而不再是“把闭环从 0 做到 1”。
