# 业务目标差距分析与开源/学术界对标

> 本文档回答一个具体问题：**项目宣称的"降低 AIGC 率"是否真的成立？**
> 结论先放在前面：当前仓库已经把"评分 + 改写 + 质量门"工程做出来了，但**整条链路从未与真实 AI 检测器闭环**，所以严格说不是"业务目标失败"，而是"业务目标无法被验证"。下文用证据展开，并对照开源/学术界给出可执行的改进路径。

---

## 0. TL;DR

- **真正的问题不是"没做"，而是"评分尺子和检测器尺子可能不是同一把"。**
  改写目标、改写过程、后验校验都来自同一份 40 行正则表达式（`internal/domain/airate.go`）。
  这是一个自洽闭环——分数能掉到 ≤ 0.30，但 GPTZero、Sapling、Originality.ai 不一定承认。
- **缺三件事**：① 真实检测器对接默认关闭 ② 没有标注集回归 ③ 没有 PPL 真值与 Binoculars 类的多模型信号。
- **能拿来就用的开源资产**：Binoculars（ICML 2024，零样本 90% 召回，CPU 也能跑）、DIPPER（NeurIPS 2023，段级 paraphraser，把 DetectGPT 召回从 70.3% 打到 4.6%）、blader/humanizer（基于 Wikipedia AI 写作信号清单的 prompt 规则集）。
- **下一步最小动作**：建一个 100 条样本的离线评测集，对接一个第三方检测器跑回归，让所有改进都可量化。在这个底座之前的任何"算法升级"都是没有刻度的。

---

## 1. 业务目标 vs 当前现状

### 1.1 项目自己写的目标

`README.md:3` — "refining AI-written academic and technical drafts into more natural writing while preserving structure, terminology, and meaning"。
`tasks/todo.md` 与 `docs/03-findings/` 反复出现"AI 率/风险分/降 AI 化"等口径。

业务目标可以总结为一句话：**输入 AI 草稿，输出能通过主流 AI 检测器的版本，且不破坏事实与结构**。

### 1.2 当前实现盘点（基于实际代码，非文档话术）

| 能力 | 文件 | 现状 |
| ---- | ---- | ---- |
| AI 率本地估算 | `internal/domain/airate.go:73-124` | **40 行正则 + 词表** 加权打分，与任何真实检测器无关 |
| 多信号复合分 | `internal/domain/scorer/composite.go:31-81` | PPL-like 34% + Burstiness 33% + n-gram 33%，再可选混入外部检测器 |
| 外部检测器接入 | `internal/domain/scorer/external.go` | OpenAI-compatible chat endpoint，**默认未配置**（`.env.example` 是空 key） |
| 校准 | `internal/domain/scorer/calibrator.go` | 持久化滚动线性回归，不是 isotonic |
| 句级 Best-of-N 改写 | `internal/agent/coordinator.go:46-56` | lexical / syntax / lexical+syntax / 原文回退 |
| 质量门控 | `internal/workflow/nodes/quality_gate.go:59-72` | 12 道（含"AI 分升高"拦截） |
| 轮次提前停止 | `internal/domain/scoring.go:5` | 硬阈值 ≤ 0.30 |
| 端到端验证 | `scripts/smoke_mock_e2e.sh:40-72` | **只验流程跑通，不验降 AI 效果** |

工程层面其实做得不薄。问题完全集中在**判别基准是什么**这一层。

### 1.3 一句话核心问题

> **当前的"降 AI 率"是一个用正则表达式定义、由 LLM 按正则表达式改写、再用同一组正则表达式自我验证的闭环。它能保证"避开几个常见 AI 短语"，不能保证"避开 GPTZero"。**

`airate.go` 里的 `aiStrongPatterns`、`aiTransitionPhrases`、`aiAbstractTerms` 既是评分依据，又通过 `ForbiddenAIPhrases()` 注入到改写 prompt 里作为目标。这是为什么用户体感"系统在跑，但 AI 检测率没降"——因为内部分掉了，但内部分本身不是检测率。

---

## 2. 学术界标杆方法

| 论文 | 年份 | 干什么 | 与本项目关系 |
| ---- | ---- | ---- | ---- |
| **DetectGPT** (Mitchell et al.) | ICML 2023 | 用扰动前后对数似然曲率判别 AI 文本 | 提供"局部似然极大值"的判别理论，反向就是改写目标 |
| **Binoculars** (Hans et al., [arxiv 2401.12070](https://arxiv.org/abs/2401.12070)) | ICML 2024 | 双模型 cross-PPL 比值，零样本检测，FPR 0.01% 下召回 ≥ 90% | **可以直接接进来作为内部"高保真信号"**，[ahans30/Binoculars](https://github.com/ahans30/Binoculars) 开源 |
| **DIPPER** (Krishna et al., [arxiv 2303.13408](https://arxiv.org/abs/2303.13408)) | NeurIPS 2023 | 11B 段级 paraphraser，可控词汇多样性与重排 | 把 DetectGPT 检测召回从 70.3% 打到 4.6%；当前项目的"句级 Best-of-N"是其弱化版 |
| **RADAR** (Hu et al., [arxiv 2307.03838](https://arxiv.org/abs/2307.03838)) | NeurIPS 2023 | paraphraser ↔ detector 对抗训练 | 给出对抗式优化范式，可用作未来 reward loop 蓝本 |
| **SICO** (Lu et al., [arxiv 2305.10847](https://arxiv.org/abs/2305.10847)) | 2023 | 只用 40 条人类样本迭代优化 prompt，让 GPT-3.5 同时绕过 6 个检测器，AUC 平均掉 0.5 | 给出 prompt-only 的可行性证据，无需训练 |
| **Paraphrasing evades detectors but retrieval is a defense** | NeurIPS 2023 | 系统性指出 paraphrasing 是当前检测器的天敌 | 反过来说，**段级 paraphrasing 才是 humanize 的主线**，句级改写收益有限 |
| **Can AI-Generated Text be Reliably Detected?** (Sadasivan et al., 2023) | arXiv | 理论上证明可靠检测不可能存在 | 给本项目提供合法性背书 |
| **GLTR** (Gehrmann et al.) | ACL 2019 | Token rank 可视化，是 GPTZero 的源头 | 解释为什么 burstiness/perplexity 仍然是检测器主流信号 |

**关键洞察**：检测器主流信号是 **PPL + Burstiness + Token Rank + n-gram**，本项目都覆盖了，但全部用代理实现。论文里这些信号都是**真模型逐 token 概率**算出来的，差别是天和地。

---

## 3. 开源项目对标

### 3.1 检测器侧（可拿来当真值用）

| 仓库 | 思路 | 部署成本 | 推荐度 |
| ---- | ---- | ---- | ---- |
| [ahans30/Binoculars](https://github.com/ahans30/Binoculars) | 双 LLM cross-PPL，零样本 | Python，需要两个小模型权重，CPU 可跑慢 | ⭐⭐⭐⭐⭐ **首选自建参考检测器** |
| [thinkst/zippy](https://github.com/thinkst/zippy) | 用压缩比检测 AI 文本 | 纯 Python，无模型依赖，<1MB | ⭐⭐⭐⭐ 极轻量，可做 CI smoke 信号 |
| [martiansideofthemoon/ai-detection-paraphrases](https://github.com/martiansideofthemoon/ai-detection-paraphrases) | DIPPER 官方仓库 + paraphrase attack 评测基线 | 需要 GPU 跑 11B 模型 | ⭐⭐⭐ 主要是论文复现资产 |
| [IBM/RADAR](https://github.com/IBM/RADAR) | 对抗训练后的 RoBERTa-large 检测器 | Python，GPU 友好 | ⭐⭐⭐ 比 GPTZero 更鲁棒的开源 baseline |

### 3.2 改写器侧（humanizer）

| 仓库 | 思路 | 与本项目的相似度 |
| ---- | ---- | ---- |
| [blader/humanizer](https://github.com/blader/humanizer) | 一个 Claude Code skill，列出 29 种 AI 写作模式 + 两轮改写 + 个性化 voice 校准 | **跟本项目 prompt 层做的事高度重叠**，但更系统化，可以直接抄它的模式清单升级 `airate.go` |
| OrbitWebTools/Humanize-AI | 客户端 JS，靠 burstiness 干扰 | 思路重叠，没工程价值 |
| DadaNanjesha/AI-Text-Humanizer-App | 调 GPT 套规则 prompt | 同上 |
| xszcs546/ai-text-humanizer | 多 humanizer 集合的导航站 | 资源页，可以拿来调研 |

### 3.3 对比矩阵：本项目 vs 业界

| 维度 | Naturalize 现状 | 学术 SOTA | 开源 humanizer 普遍水平 | 差距 |
| ---- | ---- | ---- | ---- | ---- |
| PPL 信号 | 轻量代理 | 本地小模型逐 token | 通常没有 | 缺 |
| Burstiness | 句长方差 ✓ | 句长 + 句级 PPL 序列 | 通常有句长方差 | 部分有 |
| Cross-PPL (Binoculars) | 无 | ✓ | 无 | **完全缺** |
| 真实检测器对接 | 可选/默认关 | 训练时就用真检测器做 reward | 通常无 | **完全缺** |
| 标注评测集 | 无 | 标配 | 通常无 | **完全缺** |
| 段级 paraphrase | 无（只有句级 Best-of-N） | DIPPER 级别 | 通常 prompt 改写 | 缺 |
| Style Anchor | 无 | Lee 2023、Krishna 2024 验证有效 | blader/humanizer 有 voice calibration | 缺 |
| 改写质量门 | 12 道，工程做得比业界足 | 论文一般不做 | 几乎没有 | **领先项** |
| 中文支持 | 一等公民 | 几乎都只研究英文 | 极少 | **领先项** |

读这张表的方式：**质量门和中文是本项目的护城河，不要丢；缺的是"判别真实性"和"段级改写能力"**。

---

## 4. 改进建议（按 ROI 排序）

下列建议都假设一个前提：**先把"刻度尺"做出来，再谈改写策略**。

### 🔴 P0｜建立真实评测基线（1~2 周）

没有它，下面任何改动的效果都是"我觉得"。

1. **离线标注集**：≥ 200 条样本，每条含原文、AI 草稿、人类参考、改写后文本，标注来源（人/GPT-4/Claude/Gemini）。
2. **接入一个真实检测器作为 ground truth**。优先级：
   - **第一选择**：自建 [Binoculars](https://github.com/ahans30/Binoculars)，Python 起一个独立服务，OpenAI-compatible chat 包装一层（项目 `external.go` 已经支持），CPU 也能跑，零样本不用调参；
   - **第二选择**：付费 GPTZero / Sapling / Originality.ai API 跑一次性回归（不接生产，只做评测）；
   - **第三选择**：thinkst/zippy，量级最轻，能塞进 CI。
3. **回归脚本**：扩 `scripts/smoke_mock_e2e.sh`，在 `--evaluate` 模式下跑整个标注集，输出三列数字：改写前 AI 分均值、改写后 AI 分均值、降幅。
4. **报告卡**：在 `var/data/reports/` 落 markdown，每次跑测都对比上次。

> **判定标准**：当"改写后 AI 分均值 - 改写前"≥ 25pp 时才说"业务目标达成"。当前完全不知道这个数是多少。

### 🟠 P1｜把 Binoculars 类多模型信号塞进 Scorer（1 周）

- 在 `internal/domain/scorer/` 新增 `binoculars.go`，调用本地或远端的 Binoculars 服务，结果作为一路独立信号。
- 重新平衡 `composite.go` 权重：当 Binoculars 在线时它占 50%，其它三路占 50%；不在线时退化到当前权重。
- 这一步直接把"评分尺子"从 40 行正则升级成有论文背书的高保真信号。

### 🟠 P2｜段级 paraphrase 通道（2 周）

- 现状是句级 Best-of-N，覆盖不到"段落节奏"和"信息重排"。
- 增加一条 `internal/agent/paragraph_paraphraser.go`，逻辑：以整段为输入，要求模型在保留事实清单的前提下做信息重排 + 句序打散；候选用 Scorer + 语义相似度（已有 `embeddings.go`）评分。
- prompt 直接参考 DIPPER 的两个 control knob：lexical diversity、content reordering。
- 这条通道只对"AI 草稿味儿太重的段落"启用，不替换现有句级链路。

### 🟡 P3｜把 `airate.go` 升级成可版本化模式库（3~5 天）

- 当前 40 行正则是硬编码常量，更新就改代码。
- 改成 `internal/domain/airate/patterns_zh.json` 和 `patterns_en.json`，按月更新，git 跟踪，CI 跑回归。
- 模式清单直接抄 [blader/humanizer](https://github.com/blader/humanizer) 列的 29 种 AI 写作信号（加粗滥用、em-dash 滥用、无主语片段、套话引用、"actually/additionally"等），覆盖会比现在大 3 倍。

### 🟡 P4｜Style Anchor（用户风格锚点，2 周）

- `docs/03-findings/02-statistical-methods.md §2.12` 已经写了完整方案，但没落代码。
- 这是**最有产品差异化的能力**：让用户上传 3~5 篇历史文章，提取其功能词分布、句长分布、习惯短语，注入 prompt 和 Scorer。
- Lee et al. 2023 和 Krishna et al. 2024 都验证了"对齐到某个具体人类分布"能让通用检测器判为人类。
- 工程依赖很小：磁盘上一个 json 文件 + prompt 模板拼接 + Scorer 增加一路 distance。

### 🟢 P5｜对抗式 reward loop（探索，4 周+）

- 把 Scorer 改造成 `tool.InvokableTool`（`02-statistical-methods.md §3 路径 C` 已规划）。
- coordinator 在 ReAct 循环里调用，让 LLM 拿到具体的"为什么没达标"反馈再改。
- 这是 RADAR 的思路在推理时实现，不需要训练。

### 不建议做的事

1. **不要自己训练检测器**。RoBERTa-large 类小模型训练成本不大，但维护成本高，开源已有；除非 Style Anchor 走训练路线，否则用 Binoculars 作为参考检测器即可。
2. **不要追加更多正则模式**。`airate.go` 已经过拟合到学术中文套话，继续堆只是治标。
3. **不要把内部分校准成 GPTZero 同一刻度**。校准是给用户看的"风险分"，与检测器是不同物。
4. **不要删句级 Best-of-N**。它是质量门的依据，砍掉会回退。
5. **不要在没有 P0 基线之前调阈值**。`0.30` 这个数是凭直觉拍的，没数据支撑前怎么调都是迷信。

---

## 5. 最小可执行下一步（建议本周做）

```text
Day 1-2  造 100 条标注样本（人类 50 + AI 50，按学术/技术/口语各占 1/3）
Day 3    本地起 Binoculars Python 服务，包成 OpenAI-compatible chat
Day 3    在 .env 配上检测器 URL/KEY，验证 external.go 通路
Day 4    写 scripts/evaluate.sh：跑标注集 → 输出三个数字
Day 5    跑一次基线，存到 var/data/reports/baseline-2026-05.md
```

完成后：

- **你会第一次知道当前管线把 AI 检测率从多少降到了多少。**
- 后续每次代码修改都能用同一份脚本验证，再没有"我觉得变好了"。
- 这一步打底之后，P1~P4 才是有意义的优化。

---

## 6. 引用资料

### 论文

- DetectGPT — Mitchell et al., ICML 2023
- Binoculars — Hans et al., ICML 2024 [arxiv 2401.12070](https://arxiv.org/abs/2401.12070)
- DIPPER — Krishna et al., NeurIPS 2023 [arxiv 2303.13408](https://arxiv.org/abs/2303.13408)
- RADAR — Hu et al., NeurIPS 2023 [arxiv 2307.03838](https://arxiv.org/abs/2307.03838)
- SICO — Lu et al., 2023 [arxiv 2305.10847](https://arxiv.org/abs/2305.10847)
- Can AI-Generated Text be Reliably Detected? — Sadasivan et al., 2023
- GLTR — Gehrmann et al., ACL 2019
- Do Language Models Plagiarize? — Lee et al., WWW 2023

### 开源仓库

- 检测器：[ahans30/Binoculars](https://github.com/ahans30/Binoculars)、[thinkst/zippy](https://github.com/thinkst/zippy)、[IBM/RADAR](https://github.com/IBM/RADAR)
- Paraphraser：[martiansideofthemoon/ai-detection-paraphrases](https://github.com/martiansideofthemoon/ai-detection-paraphrases)
- Humanizer：[blader/humanizer](https://github.com/blader/humanizer)、[ai-detected/ai-content-detectors](https://github.com/ai-detected/ai-content-detectors)

### 项目内相关文档

- `docs/03-findings/00-ai-rate-two-pass-issues.md` — 历史问题复核
- `docs/03-findings/01-claude-ai-rate-solution.md` — 当前已落地方案说明
- `docs/03-findings/02-statistical-methods.md` — 统计学方法库（12 指标 + Style Anchor 详案）

---

## 7. 结论

本项目不是"什么都没做"，恰恰相反，它已经把工程骨架搭得比大多数开源 humanizer 都完整：评分器、校准、质量门、句级 Best-of-N、可解释 UI 一应俱全。

它的核心缺陷只有一个——**缺少与真实 AI 检测器的闭环测量**。

这意味着：

1. 用户感受到的"AI 率没降"是真实的，因为系统从未真正测量过它降了多少；
2. 修复这一点的代价比想象中小，**接一个 Binoculars 服务 + 写一个评测脚本就够**；
3. 一旦闭环建立，现有的 P1~P4 改进才有刻度可言，否则全是空转。

把 P0 做完，再回头看 `tasks/todo.md`，路径会突然清晰很多。
