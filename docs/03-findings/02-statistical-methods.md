# 统计学与跨学科方法:降低 AI 率的数学武器库

> 本文档是 `01-claude-ai-rate-solution.md` 的技术附篇。上一份讲"怎么搭架构",本篇讲"每个指标具体怎么算、怎么让 AI 按这个目标改写"。
>
> 核心观点:**主流 AI 检测器的检测逻辑几乎全部建立在可公开推导的统计量上,逆着这些统计量优化就是降分的数学根据**。不是玄学,也不是"多让模型润色几遍"能解决的。

---

## 0. 为什么是统计学

| 检测器         | 核心判别指标                                           |
| -------------- | ------------------------------------------------------ |
| GPTZero        | Perplexity + Burstiness                                |
| DetectGPT      | Log-Likelihood Curvature(扰动前后似然差)            |
| Binoculars     | Cross-Perplexity Ratio(双模型 PPL 比值)              |
| GLTR           | Token Rank 分布                                        |
| Ghostbuster    | 多模型 PPL + 手工特征 + 线性分类器                     |
| Sapling        | 监督分类器 + 语言学特征                                |
| Turnitin AI    | 闭源,但公开测试显示对 Burstiness 和 Stylometry 敏感   |

**逆向这张表**:把每一列的判别量反过来,就是降 AI 率的目标函数。

---

## 1. 指标总览(12 条)

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Scorer 五路信号                             │
├──────────────┬──────────────┬──────────────┬──────────────┬─────────┤
│ Information  │  Statistical │  Stylometric │  Syntactic   │ Dynamic │
│  Theory      │              │              │              │         │
├──────────────┼──────────────┼──────────────┼──────────────┼─────────┤
│ Perplexity   │ Burstiness   │ 功能词 KL   │ 依存树深度   │ 句序列熵│
│ DetectGPT    │ MTLD / TTR   │ Zipf 斜率    │ n-gram JS   │         │
│ Binoculars   │              │ 可读性       │              │         │
│              │              │ Style Anchor │              │         │
└──────────────┴──────────────┴──────────────┴──────────────┴─────────┘
```

---

## 2. 每个指标的四段式定义

每条包含:**数学定义 / AI vs 人类基线 / 降分操作 / Go 实现草图 / Prompt 注入模板**。

### 2.1 Perplexity(困惑度)— 信息论

**定义**

```
PPL(x) = exp( -1/N · Σ_{i=1..N} log p(x_i | x_<i) )
```

由一个参考语言模型给出。PPL 越低,说明文本对该模型越"符合预期"。

**基线**

| 文本类型          | PPL 区间(参考:GPT2-Chinese 或 Qwen2.5-0.5B) |
| ----------------- | ------------------------------------------------ |
| GPT-4 生成的学术中文 | 5 ~ 12                                           |
| 人类学术中文      | 20 ~ 45                                          |
| 人类随笔/博客     | 30 ~ 80                                          |

**降分操作**

不是整体拔高 PPL(会破坏语义),而是**在 top-k 解码时反转选择**:

```
原解码: argmax p(x_i | x_<i)                      # AI 默认行为
新约束: 从 rank 3~10 的候选中按重加权分布采样     # 人类化行为
```

或等价地让 rewriter prompt 携带**句级 PPL 目标区间**,用 Best-of-N 选满足目标的候选。

**Go 实现草图**

```go
// internal/domain/scorer/perplexity.go
package scorer

type PerplexityModel interface {
    TokenLogProbs(ctx context.Context, text string) ([]float64, error)
}

type Perplexity struct{ model PerplexityModel }

func (p *Perplexity) Score(ctx context.Context, text string) (float64, error) {
    logProbs, err := p.model.TokenLogProbs(ctx, text)
    if err != nil || len(logProbs) == 0 {
        return 0, err
    }
    sum := 0.0
    for _, lp := range logProbs {
        sum += lp
    }
    avgNegLog := -sum / float64(len(logProbs))
    return math.Exp(avgNegLog), nil
}
```

本地模型建议:Qwen2.5-0.5B(中文效果好,CPU 可跑,~50ms/段)。

**Prompt 注入**

```
[数学目标]
本句 PPL 目标区间:[20, 50]。
生成时,对于非关键语义 token,优先从候选概率 rank 3~8 的表达中选择,
避免一直选最高概率词。"最顺口的写法"往往是 AI 陷阱。
```

---

### 2.2 Burstiness(突发性)— 统计学

**定义**

以句为单位,计算句长序列 `L = [l_1, l_2, ..., l_n]` 或句级 PPL 序列 `P` 的变异系数:

```
B_length = σ(L) / μ(L)
B_ppl    = σ(P) / μ(P)
Burstiness = 0.6 * B_length + 0.4 * B_ppl
```

**基线**

| 文本类型      | 句长 σ(字) | Burstiness |
| ------------- | ----------- | ---------- |
| AI 生成中文   | 5 ~ 12      | 0.15 ~ 0.30 |
| 人类学术中文  | 18 ~ 30     | 0.45 ~ 0.80 |
| 人类博客/随笔 | 25 ~ 45     | 0.65 ~ 1.10 |

GPTZero 的 Burstiness 就是这一族指标,是第二重要的判别量。

**降分操作**

强制**长短句交替**。在段落尺度上:

```
至少包含: 1 个 ≤ 15 字短句 + 1 个 ≥ 40 字长句
段内句长标准差 σ ≥ 18
```

**Go 草图**

```go
func Burstiness(text string) float64 {
    sents := SplitSentences(text)  // 按 。!?;分句
    if len(sents) < 3 { return 0 }
    lens := make([]float64, len(sents))
    for i, s := range sents {
        lens[i] = float64(utf8.RuneCountInString(s))
    }
    mu, sigma := meanStd(lens)
    if mu == 0 { return 0 }
    return sigma / mu
}
```

**Prompt 注入**

```
[结构约束]
- 每段至少 1 个 ≤ 15 字的短句(可以是独立判断、反问、感叹)。
- 每段至少 1 个 ≥ 40 字的长句(复合结构、嵌套修饰)。
- 禁止连续 3 句以上长度差异 < 10 字。
- 这条不是风格建议,是硬约束:改写后会自动校验句长标准差。
```

---

### 2.3 DetectGPT 曲率 — 微分几何 / 信息论

**定义**

DetectGPT(Mitchell et al., 2023 ICML)发现:AI 文本处于对数似然函数的**局部极大值**,任何微小扰动都会让似然下降;人类文本不在极大值,扰动后似然可能上升也可能下降。

```
d(x) = log p(x) - E_{x' ~ T(x)}[ log p(x') ]
```

其中 `T(x)` 是对 `x` 做 mask 填空扰动得到的变体集合。

- AI 文本:`d(x) >> 0`(尖峰)
- 人类文本:`d(x) ≈ 0`(平坦)

**降分操作**

让改写候选处于"**多等价表达都合理**"的平坦区。Best-of-N 里不要挑 L 最小的那个,而是挑 **L 最小且 d(x) 接近 0** 的那个。

**Go 草图**

```go
func DetectGPTCurvature(ctx context.Context, m PerplexityModel, text string, k int) (float64, error) {
    base, err := m.LogLikelihood(ctx, text)
    if err != nil { return 0, err }
    sum := 0.0
    for i := 0; i < k; i++ {
        perturbed := maskFillPerturb(text, 0.15) // 随机 mask 15% token,用小模型填回
        ll, err := m.LogLikelihood(ctx, perturbed)
        if err != nil { return 0, err }
        sum += ll
    }
    return base - sum/float64(k), nil // 越接近 0 越像人类
}
```

**Prompt 注入**

```
[等价冗余原则]
每个关键表达,在脑子里准备 2~3 个语义等价的同义说法,
然后**故意不选那个最标准、最高概率的**,
让整句处在"多种说法都可以"的区域,而不是"只能这么说"的尖峰。
```

---

### 2.4 功能词分布 KL 散度 — 计量文体学

**定义**

中文功能词表(建议起始,可扩充到 80+ 词):

```
的 了 是 在 和 也 又 还 却 但 而 并 或 由 与
虽然 但是 因此 所以 由于 然而 而且 不仅 即使 尽管
这 那 其 此 之 以 于 从 把 被 将 给 对 向 为
```

统计这些词在文本中的频率向量 `P`,与参考分布 `Q`(来自大规模人类语料)算 KL 散度:

```
KL(P || Q) = Σ_i P(i) · log( P(i) / Q(i) )
```

**基线**

| 文本类型     | KL(P_output || Q_human) |
| ------------ | ----------------------- |
| AI 生成中文  | 0.15 ~ 0.40             |
| 人类学术中文 | 0.02 ~ 0.10             |

AI 倾向于**过度使用"因此/综合来看/进一步来说"**,功能词分布明显偏移。

**降分操作**

维护目标分布 `Q`,要求改写后 `KL ≤ 0.10`。硬加黑名单(与当前 `aiTransitionPhrases` 合流)。

**Go 草图**

```go
type FuncWordDist struct{ Ref map[string]float64 }

func (f *FuncWordDist) KL(text string) float64 {
    counts := countTokens(text, f.words())
    total := sumMap(counts)
    if total == 0 { return 0 }
    kl := 0.0
    for w, refFreq := range f.Ref {
        p := float64(counts[w]) / float64(total)
        if p > 0 && refFreq > 0 {
            kl += p * math.Log(p/refFreq)
        }
    }
    return kl
}
```

**Prompt 注入**

```
[功能词硬约束]
以下词在整段中不得超过各自配额(括号内为每 500 字上限):
综合来看(0) 进一步来说(0) 由此可见(0) 换句话说(1)
因此(2) 所以(3) 然而(2) 不仅(1) 意义(1) 价值(1)
层面(1) 维度(1) 路径(1) 前景(0)

以下鼓励使用(模拟人类论说文):
其实 不过 而 并 倒是 反而 至于 说到底
```

---

### 2.5 Zipf 斜率 — 语料库语言学

**定义**

Zipf 定律:词频 `f` 与排名 `r` 满足 `f(r) ∝ 1/r^s`,典型 `s ≈ 1`。

对文本做词频统计,取 `(log r, log f)` 做线性回归,斜率即 `s`。

**基线**

| 文本类型     | Zipf 斜率 s |
| ------------ | ----------- |
| AI 生成中文  | 1.15 ~ 1.40(高频词占比过大) |
| 人类学术中文 | 0.95 ~ 1.10 |

**降分操作**

- 减少高频词重复(特别是抽象名词)
- 引入低频实词(具体例子、具体数字、具体场景)

**Go 草图**

```go
func ZipfSlope(text string) float64 {
    freq := countContentWords(text) // 剔除功能词后统计
    sorted := sortByFreqDesc(freq)
    xs, ys := []float64{}, []float64{}
    for r, kv := range sorted {
        if kv.count < 2 { break }
        xs = append(xs, math.Log(float64(r+1)))
        ys = append(ys, math.Log(float64(kv.count)))
    }
    return -linearRegressionSlope(xs, ys) // 取负号
}
```

**Prompt 注入**

```
[词汇具体化]
- 同一实义词每 100 字出现不超过 2 次;超过则换同义词或直接重构句子。
- 每 300 字至少引入 5 个低频具体词(具体人名/地名/数字/机构/事件)。
- 禁止用"意义/价值/作用/影响"替代具体描述。
```

---

### 2.6 MTLD(词汇多样性)— 计量语言学

**定义**

MTLD(McCarthy 2005):**从文本起始处读 token,累计 TTR 降到 0.72 所需的 token 数即为一个 "factor"**。正向+反向都算,取平均。

```
MTLD = (len_forward_avg + len_backward_avg) / 2
```

**基线**

| 文本类型     | MTLD      |
| ------------ | --------- |
| AI 生成中文  | 40 ~ 70   |
| 人类学术中文 | 70 ~ 120  |
| 人类文学散文 | 100 ~ 180 |

**降分操作**

目标 MTLD ≥ 80。对同一实词出现 ≥ 3 次的场景触发同义替换。

**Go 草图**

```go
func MTLD(tokens []string, threshold float64) float64 {
    fwd := mtldOneDirection(tokens, threshold)
    bwd := mtldOneDirection(reverse(tokens), threshold)
    return (fwd + bwd) / 2
}

func mtldOneDirection(tokens []string, threshold float64) float64 {
    factors, start := 0, 0
    seen := map[string]bool{}
    for i, t := range tokens {
        seen[t] = true
        ttr := float64(len(seen)) / float64(i-start+1)
        if ttr <= threshold {
            factors++
            start = i + 1
            seen = map[string]bool{}
        }
    }
    remainder := 1 - (float64(len(seen))/float64(len(tokens)-start+1))
    remainderFactor := remainder / (1 - threshold)
    return float64(len(tokens)) / (float64(factors) + remainderFactor)
}
```

**Prompt 注入**

```
[多样性约束]
- 100 字内,任意实义词(排除功能词)出现次数 ≤ 2。
- 若必须反复提及某个概念,使用:代称、省略、具体化(换成它的例子)三种手段轮换。
```

---

### 2.7 依存树深度 — 句法学

**定义**

对句子做依存句法分析(hanlp / stanza / LTP),得到依存树,计算**最大深度**与**平均深度**。

```
depth(sentence) = max path length from ROOT to any leaf
```

**基线**

| 文本类型     | 句法深度均值 | 深度标准差 |
| ------------ | ------------ | ---------- |
| AI 生成中文  | 4.2 ~ 5.0    | 0.6 ~ 1.2(集中) |
| 人类学术中文 | 3.5 ~ 6.5    | 1.5 ~ 2.8(离散) |

**降分操作**

强制段内混入**极浅句**(深度 ≤ 3,比如"这站不住脚。")和**极深句**(深度 ≥ 6,多层嵌套修饰 + 从句)。

**Prompt 注入**

```
[句法深度扰动]
- 每段至少 1 个极简短句(主谓 + 一个宾语,≤ 8 字,如"这不成立。")
- 每段至少 1 个包含 ≥ 3 层嵌套的长句(主句 + 定语从句 + 状语从句)
- 禁止连续 4 句以上句法结构相似(都是"主语 + 状语 + 谓语 + 宾语")
```

---

### 2.8 n-gram Jensen-Shannon 散度 — 统计 NLP

**定义**

建立 **AI 高频 3-gram 词表**(基于大量 AI 回流语料统计),记频率分布 `P_ai`。对输出文本算其 3-gram 分布 `P_out`,计算 JS 散度:

```
JS(P || Q) = 1/2 · KL(P || M) + 1/2 · KL(Q || M),  M = (P + Q) / 2
```

**降分操作**

让 `JS(P_out || P_ai) ≥ 0.3`,即"离 AI 典型 3-gram 分布足够远"。

这是**可升级的规则库**:每月更新 `P_ai` 的 top-1000 3-gram,持续对抗。

**Go 草图**

```go
func NGramJSDivergence(text string, aiRef map[string]float64, n int) float64 {
    outFreq := ngramFreq(text, n)
    return jensenShannon(outFreq, aiRef)
}
```

**Prompt 注入**

```
[负面 n-gram 黑名单](示例,完整列表从 scorer 动态注入)
禁用 3-gram:
- "的 意义 在于"
- "进一步 来说"
- "从 某种 程度"
- "在 当前 背景"
- "具有 重要 意义"
- "值得 深入 探讨"
...共 1000 条,每月更新。
```

---

### 2.9 可读性 Grade — 心理语言学

**中文可读性**(王蕾 2005,适用于学术中文):

```
R = 100 - (1.6 · 平均句长 + 0.9 · 难词率 · 100)
```

**英文**(Flesch-Kincaid Grade):

```
FKGL = 0.39 · (words/sentences) + 11.8 · (syllables/words) - 15.59
```

**基线**

| 文本类型          | 中文 R    | 英文 FKGL |
| ----------------- | --------- | --------- |
| AI 生成学术文本   | 30 ~ 45   | 14 ~ 18   |
| 人类高质量学术    | 45 ~ 65   | 10 ~ 14   |

**降分操作**

目标区间:中文 R ∈ [45, 65],英文 FKGL ∈ [10, 14]。过低过高都偏离人类典型分布。

---

### 2.10 句序列 Permutation Entropy — 动力系统

**定义**

把段落内每句的句长 / PPL / 情感极性视作时间序列 `X = [x_1, ..., x_n]`,取 order `m=3` 的相邻滑窗,统计每种排列模式出现频率 `p_π`,计算香农熵:

```
PE = -Σ_π p_π · log(p_π)
```

**基线**

| 文本类型     | PE(order=3,归一化到 [0,1]) |
| ------------ | ------------------------------ |
| AI 生成中文  | 0.45 ~ 0.65(太规律)          |
| 人类学术中文 | 0.75 ~ 0.95                    |

**降分操作**

注入话题微跳跃、情感微波动、节奏随机性。

**Prompt 注入**

```
[节奏随机性]
- 不要保持稳定推进节奏。
- 允许偶尔"岔题一句"再拉回(比如插入一个具体例子打破抽象推进)。
- 允许偶尔"情感反转"(整段理性分析中插入一句轻度质疑或感叹)。
```

---

### 2.11 Cross-Perplexity(Binoculars)— 信息论 / 对抗

**定义**

Binoculars(Hans et al., 2024)用两个模型 M1、M2:

```
Binoculars(x) = log PPL_M1(x) / log CrossPPL(M1 | M2)
```

AI 文本:两个模型对它的预期高度一致,Binoculars 分高。
人类文本:两模型看法不一致,分低。

**降分操作**

让文本的"**多模型一致性**"降低 — 即在多个模型眼里,它都不是最标准的写法。本地用 Qwen2.5-0.5B + GPT2-Chinese 两个小模型交叉验证,取 Binoculars 分作为 scorer 子信号。

---

### 2.12 Style Anchor(目标作者对齐)— 迁移学习 + 统计

**这是最强的降分武器,也是最大的产品差异化点**。

**原理**

任何检测器本质是**"把文本归到 AI 分布还是人类一般分布"**。但"人类一般分布"是一个聚合态 — 如果文本能对齐到**某个具体作者的真实分布**,检测器会把它归为"像某个人写的",从而判为人类。

这一点已被多个研究复现(Lee et al. 2023, Krishna et al. 2024)。

**实现**

用户上传 **3~5 篇历史文本**(毕业论文、公众号文章、博客均可,≥ 3000 字),系统提取:

| 维度           | 提取物                                  |
| -------------- | --------------------------------------- |
| 功能词分布     | per-user 频率向量 `Q_user`             |
| 句长分布       | 直方图 + 均值 + 方差                    |
| PPL 基线       | 均值 + 方差                             |
| 常用短语库     | top-50 bigram / trigram                 |
| 标点习惯       | "。!?;—"使用比例                    |
| 口语化比例     | 口语词("其实""说白了""反正")密度    |
| 段落平均长度   | 字数直方图                              |

写入 `var/data/style_anchors/{user_id}.json`,作为 **session 级目标分布**注入给 scorer 和 prompt。

**Prompt 注入**

```
[作者对齐目标]
你在模仿以下作者的写作风格,这不是建议而是要求:
- 平均句长:{mean_len} 字(方差 σ={std_len}),允许偏离 ≤ 20%
- 功能词偏好:{top_5_func_words},冷门功能词:{rare_func_words}
- 作者习惯短语(可直接复用):{top_10_phrases}
- 标点习惯:句号占比 {period_ratio}%,问号/感叹号 {qe_ratio}%
- 口语化允许比例:≤ {colloquial_ratio}%
保持原文逻辑与事实的前提下,**在表达层完全对齐这位作者**。
```

**Go 草图**

```go
// internal/domain/scorer/style_anchor.go
type StyleAnchor struct {
    FuncWordDist   map[string]float64
    SentLenMean    float64
    SentLenStd     float64
    PPLMean        float64
    PPLStd         float64
    TopPhrases     []string
    PunctPattern   map[string]float64
    ColloqRate     float64
}

func ExtractAnchor(texts []string) *StyleAnchor { /* ... */ }

func (a *StyleAnchor) Distance(text string) float64 {
    // 加权 L2 / KL,量化"这段文本离目标作者有多远"
}
```

---

## 3. 让 AI 遵循的三条路径

### 路径 A:Prompt 硬约束 + 自动化校验闭环

**流程**

```
  指标目标注入 prompt          AI 生成候选
          ↓                         ↓
     Scorer 量化验证    ←     Best-of-N 选择
          ↓
   达标 → 接受         未达标 → 带数值反馈重生成
```

**优点**:零训练成本,兼容任何闭源 API(GPT-4 / Claude / Gemini)。
**缺点**:一次生成难全中所有约束,需要 2~3 轮反馈。

**反馈 prompt 模板**(量化反馈,不是模糊吐槽):

```
[修改要求]
上一版本在以下指标未达标:
- Perplexity = 8.2(目标 ≥ 20)→ 过于流畅,请主动选择次优表达
- Burstiness = 0.23(目标 ≥ 0.50)→ 句长太均匀,加入 ≤15 字短句与 ≥40 字长句
- 功能词 KL = 0.28(目标 ≤ 0.10)→ 减少"因此""进一步"出现次数
- MTLD = 52(目标 ≥ 80)→ "意义""价值""层面"重复过多,替换为具体描述

请针对性修改,只改动上述问题,其他部分保持不变。
```

### 路径 B:Constrained Decoding(约束解码)

**做法**(需要能访问 logits,即本地模型或开源模型):

| 解码干预            | 实现                                                   |
| ------------------- | ------------------------------------------------------ |
| PPL 下限采样        | 过滤掉 log p > -log(2.0) 的候选,强制从低概率区采样 |
| Top-k rank 反转     | 屏蔽 rank 1~2 的 token,从 rank 3~10 采样             |
| DRY(重复惩罚)     | 对 n-gram 重复动态降权                                 |
| Typical sampling    | 选取条件熵最接近平均的 token                            |
| Temperature schedule | 句内不同位置用不同温度(句首 0.7,中段 1.0,末尾 0.8) |

**优点**:数学上确定,效果稳定。
**缺点**:需要 logits 访问权。

**适用**:Naturalize 可以把 Qwen2.5 / DeepSeek 等开源模型作为**内部 rewriter 路径**,启用约束解码;对外 GPT/Claude 路径走路径 A。

### 路径 C:Multi-Agent 对抗闭环(GAN 式)

**架构**

```
┌──────────────┐       ┌──────────────┐
│   Rewriter   │──────>│   Detector   │
│   (Agent A)  │<──────│   (Agent B)  │
└──────────────┘ score └──────────────┘
       ▲                        │
       │   "你的第 3 句 PPL=8, │
       │    burstiness=0.2, 重写"│
       └────────────────────────┘
```

Agent B 是一个**校准过的内部 scorer**,扮演"检测器",逐句给分并反馈给 Agent A。Agent A 按反馈改,直到 Agent B 满意或达上限。

**与 Naturalize 现有架构的关系**:现有 coordinator 已经是 ReAct 结构,**Detector 可以作为一个新的 tool 注入**:

```go
// internal/agent/tools/scorer_tool.go
func NewScorerTool(s *scorer.Composite) tool.InvokableTool {
    return utils.NewTool(
        "ai_score_inspector",
        "Score a sentence for AI-likeness, return detailed signals",
        func(ctx context.Context, args ScorerArgs) (ScorerResult, error) {
            return s.Score(ctx, args.Text)
        },
    )
}
```

---

## 4. 集成架构(对齐到现有代码)

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Session 启动                                  │
│                              │                                        │
│          ┌───────────────────┴──────────────────┐                    │
│          ▼                                      ▼                    │
│  [可选] Style Anchor                  全局参考分布(默认)             │
│  从用户上传历史提取                    人类学术语料统计                │
│          │                                      │                    │
│          └──────────────┬───────────────────────┘                    │
│                         ▼                                            │
│              target_dist 注入 session                                │
└─────────────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Pipeline(每个 chunk)                            │
│                                                                      │
│  Parse → Chunk → Scorer.Initial ──────────┐                         │
│                         │                  │                        │
│                         ▼                  │                        │
│              [top-K 高分句定位]            │                        │
│                         │                  │                        │
│                         ▼                  │                        │
│         Coordinator (ReAct + ScorerTool)   │                        │
│            │         │         │           │                        │
│            ▼         ▼         ▼           │                        │
│         lexical   syntax   (new)           │                        │
│                            style_aligner   │                        │
│            │         │         │           │                        │
│            └────┬────┴─────────┘           │                        │
│                 ▼                          │                        │
│            Best-of-N(4 候选)             │                        │
│                 │                          │                        │
│                 ▼                          │                        │
│         Scorer.Rescore + Fidelity Gate    │                        │
│                 │                          │                        │
│          达标?─┴─ 否 → 重试(上限 3)──┘                        │
│              │                                                      │
│              是                                                     │
│              ▼                                                      │
│           Validator → Merge → Export                                │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 5. 主 Prompt 模板(整合版)

这份模板就是 **"让 AI 遵循统计学方案"** 的最终形态,可直接写入 `prompts/sentence_rewriter_cn.md`:

```
你是一个中文文本改写引擎。本次任务的目标不是"润色",而是让下面这段文本在
多个可量化的统计指标上对齐人类写作分布。

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
【待改写句】
{sentence}

【上下文(前后各一句,仅供参考,不改写)】
prev: {prev_sentence}
next: {next_sentence}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

【本句当前状态】
- Perplexity: {ppl_current}           目标: ≥ {ppl_target}
- 句长: {len_current} 字               目标方差贡献: σ 提升 ≥ 15
- 命中 AI 高频短语: {hit_phrases}     目标: 全部剔除
- 词汇多样性 MTLD(段级): {mtld}       目标: ≥ {mtld_target}
- 功能词 KL(段级): {kl}               目标: ≤ 0.10

【硬约束】
1. 禁用以下短语(命中即失败):
   {forbidden_phrases}
2. 不得使用以下抽象名词替代具体描述:
   意义 价值 作用 影响 层面 维度 路径 前景 趋势 机制
3. 数字 / 日期 / 引用 / 实体必须 1:1 保留(详见下方"事实清单")。

【可选软约束(当有 Style Anchor 时启用)】
- 目标作者平均句长: {anchor.sent_len} 字
- 目标作者功能词偏好: {anchor.top_func_words}
- 目标作者习惯短语(可直接复用): {anchor.phrases}

【事实清单(必须原样保留)】
- 数字: {numbers}
- 日期: {dates}
- 命名实体: {entities}
- 引用标记: {citations}

【输出格式】
只输出改写后的句子,不要解释、不要标注、不要前后缀。
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**关键点**:每一条指令背后都有一个数学目标 + 后置校验,AI 不是在"努力做好",而是在**命中可验证的数字目标**。

---

## 6. 上线验证方法

### 6.1 指标回归基线

构建 500 条标注语料,每条同时记录:

| 字段            | 来源                                    |
| --------------- | --------------------------------------- |
| 原文            | 真人 / AI / 已润色                      |
| PPL(内部)     | 本地 Qwen2.5-0.5B                      |
| Burstiness      | 计算                                    |
| 功能词 KL       | 计算                                    |
| Zipf / MTLD     | 计算                                    |
| Binoculars      | 双模型计算                              |
| GPTZero 真分    | API                                     |
| Sapling 真分    | API                                     |
| 人工标注(1-5) | 评审                                    |

### 6.2 合格门槛

| 指标                                       | 阈值            |
| ------------------------------------------ | --------------- |
| 内部总分 vs GPTZero:Spearman 秩相关       | ≥ 0.70          |
| 改写前后 GPTZero AI 分平均下降             | ≥ 25 pp         |
| 改写后语义相似度(SimCSE)均值             | ≥ 0.85          |
| 事实不变量违规率                           | 0               |
| 人工可读性评分均值                         | ≥ 3.8 / 5       |
| **Style Anchor 用户组 GPTZero AI 分均值**  | **≤ 20%**       |

最后一项是 Style Anchor 路径的核心指标 — **带锚点的用户应该比不带锚点的用户降分效果强 1.5x 以上**。

---

## 7. 与设计文档 `01` 的对应关系

| `01` 中的抽象模块      | `02` 中的具体实现                                       |
| ---------------------- | ------------------------------------------------------- |
| Scorer 五路信号        | §2.1 PPL、§2.2 Burstiness、§2.4 KL、§2.6 MTLD、§2.11 Binoculars |
| 损失函数 `L(t')` 的 α  | §2.1 + §2.2 + §2.3 + §2.8 的加权组合                   |
| Best-of-N 的候选差异化 | §3 路径 A 的温度差异 + 路径 B 的约束解码                |
| Validator 保真门       | §2.5 Zipf、§2.7 句法、§2.9 可读性                       |
| coordinator tool 扩展  | §3 路径 C 的 Multi-Agent 架构                           |
| **用户个性化(新)**   | §2.12 **Style Anchor** — 这是 01 里未充分展开的关键点   |

---

## 8. 学术参考(按重要性排序)

这些不是装点门面,是每个指标背后的第一性依据,落地前建议工程师至少读摘要。

1. **Mitchell et al., 2023** — DetectGPT: Zero-Shot Machine-Generated Text Detection using Probability Curvature. ICML.
2. **Hans et al., 2024** — Spotting LLMs With Binoculars: Zero-Shot Detection of Machine-Generated Text. ICML.
3. **Gehrmann et al., 2019** — GLTR: Statistical Detection and Visualization of Generated Text. ACL.
4. **McCarthy & Jarvis, 2010** — MTLD, vocd-D, and HD-D: A validation study of sophisticated approaches to lexical diversity assessment.
5. **王蕾, 2005** — 初中高级汉语阅读课文易读性公式研究. 世界汉语教学.
6. **Lee et al., 2023** — Do Language Models Plagiarize? WWW.(Style Anchor 的理论支持)
7. **Krishna et al., 2024** — Paraphrasing evades detectors of AI-generated text, but retrieval is an effective defense. NeurIPS.
8. **Sadasivan et al., 2023** — Can AI-Generated Text be Reliably Detected? arXiv.(解释为什么纯检测不可能百分百可靠,反向给改写提供了理论空间)

---

## 9. 结论

**降 AI 率不是玄学,是一组可量化可验证的数学约束**。主流检测器的检测逻辑是公开的,逆向它们的判别量就是降分的目标函数。

本文档给出的 12 个指标 + 3 条落地路径 + 1 份主 prompt 模板,可以把 Naturalize 从"让模型多润色几遍"的盲改模式,升级为**"带数学目标的闭环优化系统"**。

**最强武器是 Style Anchor** — 当系统对齐到用户本人的历史分布时,任何通用检测器都失去判别基础。这也是产品真正可持续的差异化壁垒。
