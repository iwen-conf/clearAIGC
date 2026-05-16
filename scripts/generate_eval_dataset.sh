#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_FILE="${1:-$ROOT_DIR/testdata/eval/dataset.100.jsonl}"
mkdir -p "$(dirname "$OUT_FILE")"

cat >"$OUT_FILE" <<'JSONL'
JSONL

node - "$OUT_FILE" <<'NODE'
const fs = require('fs')
const out = process.argv[2]

const cnAiTemplates = [
  '在当前{topic}持续推进的背景下，{subject}正在呈现出{trend}的总体趋势。与此同时，越来越多的团队开始意识到，{problem}，因此{solution}已经成为一个具有现实意义的重要课题。',
  '如果从{angle}角度进行分析，可以发现现有方案主要聚焦于{focus}。但是当{condition}时，仅靠{quick_fix}往往难以获得理想效果，因为用户真正需要的是{need}。',
  '综合来看，{domain}并不是一个只靠单点优化就能完全解决的问题，而是涉及{factors}的综合任务。未来若能在{future_path}方面形成更完整的工作流，这一方向仍有明确应用前景。',
  '从实际情况看，很多团队虽已积累大量{asset}，但内容常出现{issues}。进一步来说，跨角色协作会放大这种波动，最终导致文档信息虽多，却不够自然。',
]

const cnHumanTemplates = [
  '这周我把{asset}重新梳理了一遍。原来那版信息挺全，但句子太绕，我就把几段顺序调了调，顺手删了几句套话，读起来就顺多了。',
  '我们最近在做{topic}，文档量一下子上来了。后来发现不是信息不够，而是写法不统一，所以先定了术语表，再按段改，效率明显高了。',
  '昨天评审的时候，大家对{problem}意见挺一致：内容没错，就是表达发硬。后来按读者视角改了一轮，保留原意，效果比预期好。',
  '我个人的做法是先把结论放前面，再补证据。这样处理{domain}类材料时，团队更容易对齐，也不容易在细节里绕远。',
]

const enAiTemplates = [
  'In the current stage of {topic}, many teams have recognized that {problem}. Therefore, {solution} is becoming a practical requirement for organizations that need consistent output quality.',
  'From an implementation perspective, existing approaches mainly focus on {focus}. However, when {condition}, lightweight edits are often insufficient because users need {need}.',
  'Overall, {domain} is not a problem that can be solved by one isolated optimization. It depends on {factors}, and it benefits from a workflow that combines revision, review, and export controls.',
  'In real projects, teams usually accumulate plenty of {asset}, but quality still fluctuates due to {issues}. As a result, documents remain informative yet feel rigid and repetitive.',
]

const enHumanTemplates = [
  'I reviewed the {asset} again this morning. The facts were fine, but the tone felt stiff, so I shortened a few long lines and moved one paragraph up. It reads cleaner now.',
  'We hit this during {topic}: everyone wrote valid content, but each section sounded like a different author. After we aligned terms and trimmed filler phrases, the draft finally felt cohesive.',
  'During the demo prep, the main issue was not correctness but readability. We kept the same claims, simplified sentence rhythm, and the audience followed it much more easily.',
  'My rule is simple: keep structure stable, cut generic transitions, and avoid over-polishing. That balance usually works well for {domain} documents.',
]

const pools = {
  topic: ['数字化转型','知识管理','技术写作','研发协作','交付流程','文档工程','AI写作治理','模型评测'],
  subject: ['企业文档体系','跨团队知识流转','技术文档生产','质量控制机制','内容审核流程'],
  trend: ['快速发展、持续演进和多场景融合','高频协同与实时更新并行','标准化与个性化并存'],
  problem: ['传统文档整理方式已难以支撑高频协作','文本风格和术语难以保持一致','评审成本持续上升'],
  solution: ['构建分段改写与质量门控的一体化流程','建立可回归的评测闭环','引入可解释的风险评分与人工复核'],
  angle: ['工程实现','业务落地','系统稳定性','协作效率'],
  focus: ['拼写纠错和模板替换','局部句法调整','单轮润色结果'],
  condition: ['文本包含大量术语与约束结构','多人在不同时间协作写作','需要同时满足准确性与可读性'],
  quick_fix: ['简单修补','只替换同义词','机械降重'],
  need: ['在不改变事实的前提下提升自然度','可解释且可复现的降风险结果','兼顾速度与质量的稳定流程'],
  domain: ['文本润色系统','技术文档优化','AI草稿后编辑'],
  factors: ['语言质量、结构一致性、术语稳定性和读者预期','模型能力、流程设计和人工复核策略'],
  future_path: ['分段处理、逐块比对、人工确认和结果导出','多信号评分、回归评测与风格锚点'],
  asset: ['历史文档','项目材料','评审记录','版本草稿'],
  issues: ['表达风格不统一、结构层次不清晰、术语波动','句式重复与连接词过度依赖','结论位置不稳定和段落节奏单一'],
}

const enPools = {
  topic: ['digital transformation','knowledge operations','technical communication','engineering collaboration','delivery workflows','AI-assisted writing governance'],
  problem: ['traditional editing workflows cannot keep up with collaboration speed','teams struggle to keep terminology and tone consistent','review overhead grows as drafts scale'],
  solution: ['a section-based rewrite and quality-gate pipeline','a reproducible benchmark loop with detector feedback','an explainable scoring and human-review workflow'],
  focus: ['spelling correction and template swaps','surface-level sentence edits','single-pass polishing outputs'],
  condition: ['documents include dense terminology and strict structure constraints','multiple contributors edit the same draft asynchronously','teams must preserve facts while improving readability'],
  need: ['more natural phrasing without factual drift','reproducible quality gains with measurable detector deltas','stable outputs that balance speed and quality'],
  domain: ['document refinement','technical draft editing','AI-assisted post-editing'],
  factors: ['readability, structure consistency, terminology stability, and audience expectations','model behavior, workflow controls, and review discipline'],
  asset: ['draft documents','design notes','review records','handoff writeups'],
  issues: ['style inconsistency and repetitive transitions','uneven sentence rhythm and paragraph structure','terminology drift across contributors'],
}

function pick(arr, i) {
  return arr[i % arr.length]
}

function fill(template, index, dict) {
  return template.replace(/\{(\w+)\}/g, (_, key) => pick(dict[key] || [key], index + key.length))
}

const rows = []
let id = 1

function pad(n) {
  return String(n).padStart(3, '0')
}

for (let i = 0; i < 35; i++) {
  const text = fill(pick(cnAiTemplates, i), i, pools)
  rows.push({ id: `cn-ai-${pad(id++)}`, profile: 'cn', source: 'ai', domain: i % 3 === 0 ? 'academic' : (i % 3 === 1 ? 'technical' : 'spoken'), text })
}
for (let i = 0; i < 15; i++) {
  const text = fill(pick(cnHumanTemplates, i), i, pools)
  rows.push({ id: `cn-human-${pad(id++)}`, profile: 'cn', source: 'human', domain: i % 3 === 0 ? 'academic' : (i % 3 === 1 ? 'technical' : 'spoken'), text })
}
id = 1
for (let i = 0; i < 35; i++) {
  const text = fill(pick(enAiTemplates, i), i, enPools)
  rows.push({ id: `en-ai-${pad(id++)}`, profile: 'en', source: 'ai', domain: i % 3 === 0 ? 'academic' : (i % 3 === 1 ? 'technical' : 'spoken'), text })
}
for (let i = 0; i < 15; i++) {
  const text = fill(pick(enHumanTemplates, i), i, enPools)
  rows.push({ id: `en-human-${pad(id++)}`, profile: 'en', source: 'human', domain: i % 3 === 0 ? 'academic' : (i % 3 === 1 ? 'technical' : 'spoken'), text })
}

if (rows.length !== 100) {
  throw new Error(`expected 100 rows, got ${rows.length}`)
}

const content = rows.map((row) => JSON.stringify(row)).join('\n') + '\n'
fs.writeFileSync(out, content, 'utf8')
console.log(`wrote ${rows.length} samples to ${out}`)
NODE

echo "Done: $OUT_FILE"
