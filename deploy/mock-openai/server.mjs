import http from 'node:http'

const port = Number(process.env.PORT ?? 8787)

function sendJson(res, status, payload) {
  const body = JSON.stringify(payload)
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(body),
    'x-request-id': `mock-${Date.now()}`,
  })
  res.end(body)
}

function parseInputText(prompt) {
  if (typeof prompt !== 'string' || prompt.length === 0) {
    return ''
  }

  const marker = '[INPUT TEXT]'
  const index = prompt.lastIndexOf(marker)
  const source = index >= 0 ? prompt.slice(index + marker.length) : prompt
  return source.trim()
}

function parseRound(prompt) {
  if (typeof prompt !== 'string' || prompt.length === 0) {
    return 1
  }

  const match = prompt.match(/\[ROUND\s+(\d+)\]/i)
  if (!match) {
    return 1
  }

  const round = Number.parseInt(match[1], 10)
  return Number.isFinite(round) && round > 0 ? round : 1
}

function applyPatterns(text, patterns) {
  return patterns.reduce((current, [pattern, replacement]) => current.replace(pattern, replacement), text)
}

function normalizeChineseParagraph(paragraph) {
  return paragraph
    .replace(/\s+/g, '')
    .replace(/，{2,}/g, '，')
    .replace(/。{2,}/g, '。')
    .replace(/，。/g, '。')
    .trim()
}

function humanizeChineseRhythm(paragraph) {
  return paragraph
    .replace(/，但是/g, '。但')
    .replace(/，但/g, '。但')
    .replace(/，因为/g, '。因为')
    .replace(/，这意味着/g, '。这意味着')
    .replace(/，最终/g, '。最终')
    .replace(/，结果/g, '。结果')
    .replace(/，而是/g, '；而是')
}

const chineseRound1Patterns = [
  [/人工智能生成文本在课程作业、技术报告和学术写作中越来越常见/g, '人工智能生成文本如今常见于课程作业、技术报告和学术写作'],
  [/许多内容会出现明显的模板化痕迹/g, '不少内容仍会留下明显的模板化痕迹'],
  [/为了让表达更自然/g, '为了让表达读起来更自然'],
  [/对原文进行分段改写/g, '按段落改写原文'],
  [/并且持续提供清晰的处理进度/g, '同时持续提供清晰的处理进度'],
  [/在当前([^，。；\n]{0,30}?)的大背景下，?/g, (_, topic) => `随着${topic.replace(/不断深入推进/g, '持续推进')}，`],
  [/在当前([^，。；\n]{0,30}?)背景下，?/g, (_, topic) => `随着${topic.replace(/不断深入推进/g, '持续推进')}，`],
  [/正在呈现出快速发展、持续演进以及多元融合的总体趋势/g, '正加快演进，也在和更多场景融合'],
  [/与此同时，/g, '同时，'],
  [/越来越多的团队开始认识到，/g, '越来越多的团队意识到，'],
  [/仅仅依靠传统的文档整理方式已经较难满足/g, '只靠传统的文档整理方式，已经很难满足'],
  [/高频协作、实时更新以及跨部门共享/g, '高频协作、实时更新和跨部门共享'],
  [/因此，构建一种/g, '因此，需要构建'],
  [/已经成为一个具有现实意义和实践价值的重要课题/g, '已经成了必须解决的现实问题'],
  [/从实际情况来看，/g, '从实际情况看，'],
  [/进一步来说，/g, '进一步看，'],
  [/表达风格不统一、结构层次不清晰、术语使用不稳定以及语句冗长重复/g, '表达风格不统一、结构层次不清晰、术语不稳定、语句也偏冗长'],
  [/最终导致文档虽然信息很多，却不够自然、不够凝练，也不够适合直接对外输出/g, '结果是文档虽然信息不少，却不够自然、不够凝练，也不适合直接对外输出'],
  [/由此可见，/g, '这也说明，'],
  [/如果从方法层面进行分析，可以发现/g, '从方法上看，'],
  [/主要聚焦于/g, '大多集中在'],
  [/但是当文本涉及/g, '但文本一旦涉及'],
  [/仅靠简单修补往往难以获得理想效果/g, '简单修补往往达不到理想效果'],
  [/因为用户真正需要的，并不只是把句子改对，而是希望/g, '用户真正需要的，不只是把句子改对，更是希望'],
  [/呈现出更加自然、更加顺畅、更加符合人类写作习惯的最终形态/g, '让文本更自然、更顺畅，也更符合真实写作习惯'],
  [/基于以上认识，我们可以进一步提出这样一个判断：/g, '基于这些判断，'],
  [/不在于机械地替换词语，而在于/g, '不只是机械地替换词语，更在于'],
  [/换句话说，/g, '也就是说，'],
  [/一个更成熟的系统应当能够识别哪里需要压缩、哪里需要展开、哪里需要调整逻辑顺序、哪里需要弱化模板化表达/g, '更成熟的系统应当知道哪里该压缩，哪里该展开，哪里该调整逻辑顺序，哪里该淡化模板化表达'],
  [/当然，在推进这项工作的过程中，也需要正视一些客观挑战。/g, '不过，推进这项工作时也要看到一些现实挑战。'],
  [/首先，不同文档类型之间差异较大/g, '首先，不同文档类型差异很大'],
  [/其次，用户通常不希望系统过度改写/g, '其次，用户通常不希望系统改得过重'],
  [/最后，很多用户对结果的期望并不是绝对统一的/g, '最后，用户对结果的期待并不完全一致'],
  [/综合来看，/g, '总体来看，'],
  [/并不是一个只靠单点优化就可以完全解决的问题/g, '不是靠单点优化就能彻底解决的问题'],
  [/进一步发展为/g, '走向'],
  [/总而言之，/g, '归根结底，'],
  [/这一方向具备较强的应用前景，也值得在后续实践中持续探索和不断完善/g, '这一方向有明确的应用前景，也值得继续打磨'],
  [/更加/g, '更'],
]

const chineseRound2Patterns = [
  [/同时，/g, '而且，'],
  [/因此，需要构建/g, '要建立'],
  [/因此，需要/g, '要'],
  [/因此，/g, '所以，'],
  [/从实际情况看，/g, '实际看，'],
  [/进一步看，/g, '再往下看，'],
  [/越来越多的团队意识到，/g, '很多团队已经意识到，'],
  [/只靠传统的文档整理方式，已经很难满足/g, '传统的文档整理方式已经很难支撑'],
  [/已经成了必须解决的现实问题/g, '已经是绕不开的问题'],
  [/从方法上看，/g, '方法上，'],
  [/大多集中在/g, '多半停留在'],
  [/简单修补往往达不到理想效果/g, '简单修补通常不够用'],
  [/用户真正需要的，不只是把句子改对，更是希望/g, '用户真正要的，不只是把句子改对，更希望'],
  [/基于这些判断，/g, '顺着这个思路看，'],
  [/也就是说，/g, '换个说法，'],
  [/不过，推进这项工作时也要看到一些现实挑战。/g, '不过，这件事做起来也有现实挑战。'],
  [/首先，不同文档类型差异很大/g, '首先，不同文档类型的差异很大'],
  [/其次，用户通常不希望系统改得过重/g, '其次，用户通常不希望系统改动过大'],
  [/最后，用户对结果的期待并不完全一致/g, '最后，不同用户看重的结果并不一样'],
  [/总体来看，/g, '说到底，'],
  [/归根结底，/g, '说到底，'],
  [/这一方向有明确的应用前景，也值得继续打磨/g, '这一方向有明确的应用空间，也值得继续完善'],
]

const englishRound1Patterns = [
  [/keep the original meaning intact while making the tone read more naturally/gi, 'keep the original meaning intact and make the tone read more naturally'],
  [/It should process the document in sections, report progress clearly, and return a clean result without extra notes or markup\./gi, 'It should process the document in sections and report progress clearly. The returned result should stay clean, with no extra notes or markup.'],
]

const englishRound2Patterns = [
  [/make the tone read more naturally/gi, 'make the tone feel more natural'],
  [/The returned result should stay clean, with no extra notes or markup\./gi, 'The returned result should stay clean and avoid extra notes or markup.'],
]

function rewriteChinese(paragraph, round) {
  const patterns = round >= 2 ? chineseRound2Patterns : chineseRound1Patterns
  const rewritten = applyPatterns(normalizeChineseParagraph(paragraph), patterns)
  return (round >= 2 ? humanizeChineseRhythm(rewritten) : rewritten)
    .replace(/，。/g, '。')
    .trim()
}

function rewriteEnglish(paragraph, round) {
  const patterns = round >= 2 ? englishRound2Patterns : englishRound1Patterns
  return applyPatterns(paragraph.replace(/\s+/g, ' ').trim(), patterns)
}

function debugRewrite(round, input, output) {
  const clip = (value) => (value.length > 80 ? `${value.slice(0, 80)}...` : value)
  console.log(
    JSON.stringify({
      type: 'mock-rewrite',
      round,
      changed: input !== output,
      input: clip(input),
      output: clip(output),
    }),
  )
}

function rewrite(text, round = 1) {
  if (!text) {
    return 'This is a mock response.'
  }

  const paragraphs = text
    .split(/\n\s*\n/g)
    .map((part) => part.trim())
    .filter(Boolean)

  return paragraphs
    .map((paragraph) => {
      if (paragraph.length < 24) {
        return paragraph
      }

      if (/[A-Za-z]/.test(paragraph)) {
        return rewriteEnglish(paragraph, round)
      }

      return rewriteChinese(paragraph, round)
    })
    .join('\n\n')
}

function tokenUsage(text) {
  return {
    input_tokens: Math.max(1, Math.ceil(text.length / 4)),
    output_tokens: Math.max(1, Math.ceil(text.length / 5)),
    prompt_tokens: Math.max(1, Math.ceil(text.length / 4)),
    completion_tokens: Math.max(1, Math.ceil(text.length / 5)),
  }
}

const server = http.createServer(async (req, res) => {
  if (req.method !== 'POST') {
    sendJson(res, 404, { error: { message: 'Not found' } })
    return
  }

  const chunks = []
  for await (const chunk of req) {
    chunks.push(chunk)
  }

  let payload = {}
  try {
    payload = JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    sendJson(res, 400, { error: { message: 'Invalid JSON payload' } })
    return
  }

  if (req.url === '/v1/responses') {
    const prompt = typeof payload.input === 'string' ? payload.input : ''
    const inputText = parseInputText(prompt)
    const round = parseRound(prompt)
    const text = rewrite(inputText, round)
    debugRewrite(round, inputText, text)
    const usage = tokenUsage(text)
    sendJson(res, 200, {
      id: `resp_${Date.now()}`,
      output_text: text,
      usage: {
        input_tokens: usage.input_tokens,
        output_tokens: usage.output_tokens,
      },
    })
    return
  }

  if (req.url === '/v1/chat/completions') {
    const messages = Array.isArray(payload.messages) ? payload.messages : []
    const last = messages.at(-1)
    const prompt = typeof last?.content === 'string' ? last.content : ''
    const inputText = parseInputText(prompt)
    const round = parseRound(prompt)
    const text = rewrite(inputText, round)
    debugRewrite(round, inputText, text)
    const usage = tokenUsage(text)
    sendJson(res, 200, {
      id: `chatcmpl_${Date.now()}`,
      choices: [
        {
          message: {
            role: 'assistant',
            content: text,
          },
        },
      ],
      usage: {
        prompt_tokens: usage.prompt_tokens,
        completion_tokens: usage.completion_tokens,
      },
    })
    return
  }

  sendJson(res, 404, { error: { message: 'Not found' } })
})

server.listen(port, '0.0.0.0', () => {
  console.log(`mock-openai listening on ${port}`)
})
