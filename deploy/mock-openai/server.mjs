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

function rewrite(text) {
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
        return paragraph
          .replace(/\s+/g, ' ')
          .replace(/^(.{0,70}?[.!?])\s*/u, '$1 ')
          .trim()
      }

      return paragraph
        .replace(/，/g, '，')
        .replace(/。/g, '。')
        .trim()
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
    const text = rewrite(parseInputText(payload.input))
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
    const text = rewrite(parseInputText(last?.content))
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
