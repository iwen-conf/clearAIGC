import { useState, type CSSProperties } from 'react'
import {
  Button,
  Card,
  Col,
  Empty,
  Row,
  Segmented,
  Space,
  Statistic,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import type { ChunkComparison, ChunkReviewState, ChunkStatus, RoundDiffResponse } from '@/types'
import { exportUrl } from '@/api'
import { formatCharDelta } from '@/lib/format'

const { Text, Paragraph } = Typography

type NormalizedChunkComparison = ChunkComparison & {
  state: ChunkReviewState
  status: ChunkStatus
  aiRate: number
  outputAiRate: number
  detector: string
}

interface HighlightSegment {
  text: string
  changed: boolean
}

interface HighlightDiff {
  original: HighlightSegment[]
  rewritten: HighlightSegment[]
}

interface CardReviewPanelProps {
  comparison: RoundDiffResponse | null
  sessionId: string | null
  roundNumber: number | null
  cardBusyId: string | null
  applyingAll: boolean
  onAccept: (cardId: string) => void
  onReject: (cardId: string) => void
  onApplyAll: () => void
  readOnly?: boolean
}

const stateMeta: Record<ChunkReviewState, { label: string; color: string }> = {
  pending: { label: '待审核', color: 'default' },
  accepted: { label: '已接受', color: 'green' },
  rejected: { label: '保留原文', color: 'default' },
}

const statusMeta: Record<ChunkStatus, { label: string; color: string }> = {
  passed: { label: '已通过', color: 'success' },
  recovered: { label: '二次复核', color: 'warning' },
  failed: { label: '需人工复核', color: 'error' },
}

function aiRatePercent(rate: number): string {
  return `${Math.round(rate * 100)}%`
}

function aiRateColor(rate: number): string {
  if (rate >= 0.66) return 'error'
  if (rate >= 0.33) return 'warning'
  return 'default'
}

function normalizeChunkState(state: string | undefined): ChunkReviewState {
  if (state === 'accepted' || state === 'rejected') return state
  return 'pending'
}

function normalizeChunkStatus(status: string | undefined): ChunkStatus {
  if (status === 'recovered' || status === 'failed') return status
  return 'passed'
}

function normalizeAiRate(aiRate: number | undefined): number {
  if (!Number.isFinite(aiRate)) return 0
  const safeRate = aiRate ?? 0
  return Math.min(1, Math.max(0, safeRate))
}

function normalizeChunk(chunk: ChunkComparison): NormalizedChunkComparison {
  return {
    ...chunk,
    state: normalizeChunkState(chunk.state),
    status: normalizeChunkStatus(chunk.status),
    aiRate: normalizeAiRate(chunk.aiRate),
    outputAiRate: normalizeAiRate(chunk.outputAiRate),
    detector: chunk.detector?.trim() || '启发式规则',
  }
}

const chineseDigits = ['零', '一', '二', '三', '四', '五', '六', '七', '八', '九'] as const
const chineseUnits = ['', '十', '百', '千'] as const

function toChineseNumber(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return '零'
  }

  const numbers = String(Math.trunc(value))
    .split('')
    .map((digit) => Number(digit))

  let result = ''
  let pendingZero = false

  numbers.forEach((digit, index) => {
    const unitIndex = numbers.length - index - 1
    if (digit === 0) {
      pendingZero = result !== ''
      return
    }

    if (pendingZero) {
      result += chineseDigits[0]
      pendingZero = false
    }

    if (!(digit === 1 && unitIndex === 1 && result === '')) {
      result += chineseDigits[digit]
    }
    result += chineseUnits[unitIndex] ?? ''
  })

  return result
}

function formatParagraphLabel(paragraphIndex: number): string {
  return `第${toChineseNumber(paragraphIndex)}段`
}

function normalizeComparableText(text: string): string {
  return text.replace(/\r\n/g, '\n')
}

function hasVisibleRewriteChange(chunk: ChunkComparison): boolean {
  return normalizeComparableText(chunk.input) !== normalizeComparableText(chunk.output)
}

function buildHighlightDiff(input: string, output: string): HighlightDiff {
  const source = Array.from(normalizeComparableText(input))
  const target = Array.from(normalizeComparableText(output))

  const dp = Array.from({ length: source.length + 1 }, () => new Uint16Array(target.length + 1))
  for (let sourceIndex = source.length - 1; sourceIndex >= 0; sourceIndex -= 1) {
    for (let targetIndex = target.length - 1; targetIndex >= 0; targetIndex -= 1) {
      dp[sourceIndex][targetIndex] =
        source[sourceIndex] === target[targetIndex]
          ? dp[sourceIndex + 1][targetIndex + 1] + 1
          : Math.max(dp[sourceIndex + 1][targetIndex], dp[sourceIndex][targetIndex + 1])
    }
  }

  const original: HighlightSegment[] = []
  const rewritten: HighlightSegment[] = []
  let sourceIndex = 0
  let targetIndex = 0

  const pushSegment = (segments: HighlightSegment[], text: string, changed: boolean) => {
    if (!text) return
    const previous = segments.at(-1)
    if (previous && previous.changed === changed) {
      previous.text += text
      return
    }
    segments.push({ text, changed })
  }

  while (targetIndex < target.length) {
    if (sourceIndex < source.length && source[sourceIndex] === target[targetIndex]) {
      pushSegment(original, source[sourceIndex], false)
      pushSegment(rewritten, target[targetIndex], false)
      sourceIndex += 1
      targetIndex += 1
      continue
    }

    if (
      targetIndex + 1 <= target.length &&
      (sourceIndex === source.length || dp[sourceIndex][targetIndex + 1] >= dp[sourceIndex + 1][targetIndex])
    ) {
      pushSegment(rewritten, target[targetIndex], true)
      targetIndex += 1
      continue
    }

    if (sourceIndex < source.length) {
      pushSegment(original, source[sourceIndex], true)
    }
    sourceIndex += 1
  }

  while (sourceIndex < source.length) {
    pushSegment(original, source[sourceIndex], true)
    sourceIndex += 1
  }

  return { original, rewritten }
}

function renderHighlightSegments(
  keyPrefix: string,
  segments: HighlightSegment[],
  changedStyle: CSSProperties,
  emptyText = '—',
) {
  if (segments.length === 0) {
    return emptyText
  }

  return segments.map((segment, index) =>
    segment.changed ? (
      <mark key={`${keyPrefix}-segment-${index}`} style={changedStyle}>
        {segment.text}
      </mark>
    ) : (
      <span key={`${keyPrefix}-segment-${index}`}>{segment.text}</span>
    ),
  )
}

export function CardReviewPanel(props: CardReviewPanelProps) {
  const {
    comparison,
    sessionId,
    roundNumber,
    cardBusyId,
    applyingAll,
    onAccept,
    onReject,
    onApplyAll,
    readOnly = false,
  } = props
  const [selection, setSelection] = useState<'all' | 'accepted'>('all')

  if (!comparison || comparison.chunks.length === 0) {
    return (
      <div
        style={{
          display: 'flex',
          minHeight: 240,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Empty description="暂无评审卡片。最新一轮润色完成后，所有段落将在此以卡片形式显示。" />
      </div>
    )
  }

  const chunks = comparison.chunks.map(normalizeChunk).filter(hasVisibleRewriteChange)
  if (chunks.length === 0) {
    return (
      <div
        style={{
          display: 'flex',
          minHeight: 240,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Empty description="当前轮次没有实际改动的片段，未改动内容已自动隐藏。" />
      </div>
    )
  }

  const total = chunks.length
  const pending = chunks.filter((chunk) => chunk.state === 'pending').length
  const accepted = chunks.filter((chunk) => chunk.state === 'accepted').length
  const rejected = chunks.filter((chunk) => chunk.state === 'rejected').length
  const downloadReady = sessionId !== null && roundNumber !== null

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card size="small">
        <Row gutter={[16, 16]} align="middle">
          <Col xs={24} md={14}>
            <Space size={24} wrap>
              <Statistic title="总卡片" value={total} />
              <Statistic
                title="待审核"
                value={pending}
                valueStyle={{ color: pending > 0 ? '#faad14' : undefined }}
              />
              <Statistic
                title="已接受"
                value={accepted}
                valueStyle={{ color: accepted > 0 ? '#52c41a' : undefined }}
              />
              <Statistic title="保留原文" value={rejected} />
            </Space>
          </Col>
          <Col xs={24} md={10}>
            <Space direction="vertical" size={8} style={{ width: '100%', alignItems: 'flex-end' }}>
              {!readOnly && (
                <Button
                  type="primary"
                  loading={applyingAll}
                  disabled={pending === 0}
                  onClick={onApplyAll}
                >
                  全部应用{pending > 0 ? ` (${pending})` : ''}
                </Button>
              )}
              <Space size={8} wrap>
                <Segmented
                  size="small"
                  value={selection}
                  onChange={(value) => setSelection(value as 'all' | 'accepted')}
                  options={[
                    { label: '全部', value: 'all' },
                    { label: '仅已接受', value: 'accepted' },
                  ]}
                />
                <Button
                  disabled={!downloadReady}
                  href={downloadReady ? exportUrl(sessionId!, roundNumber!, 'txt', selection) : undefined}
                >
                  下载 TXT
                </Button>
                <Button
                  type="primary"
                  disabled={!downloadReady}
                  href={downloadReady ? exportUrl(sessionId!, roundNumber!, 'docx', selection) : undefined}
                >
                  下载 DOCX
                </Button>
              </Space>
            </Space>
          </Col>
        </Row>
      </Card>

      {chunks.map((chunk) => (
        <ReviewCard
          key={chunk.id}
          chunk={chunk}
          busy={cardBusyId === chunk.id}
          readOnly={readOnly}
          onAccept={() => onAccept(chunk.id)}
          onReject={() => onReject(chunk.id)}
        />
      ))}
    </Space>
  )
}

interface ReviewCardProps {
  chunk: NormalizedChunkComparison
  busy: boolean
  readOnly?: boolean
  onAccept: () => void
  onReject: () => void
}

function ReviewCard({ chunk, busy, readOnly = false, onAccept, onReject }: ReviewCardProps) {
  const state = stateMeta[chunk.state]
  const status = statusMeta[chunk.status]
  const originalRateColor = aiRateColor(chunk.aiRate)
  const outputRateColor = aiRateColor(chunk.outputAiRate)
  const highlightDiff = buildHighlightDiff(chunk.input, chunk.output)

  return (
    <Card
      size="small"
      data-testid="workspace-review-card"
      style={{
        opacity: chunk.state === 'rejected' ? 0.75 : 1,
        borderColor: chunk.state === 'accepted' ? '#b7eb8f' : undefined,
      }}
      title={
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <Space size={12} wrap>
            <Text strong>
              片段 {chunk.chunkIndex + 1} · {formatParagraphLabel(chunk.paragraphIndex + 1)}
            </Text>
            <Text type="secondary">评估规则：{chunk.detector}</Text>
          </Space>
          <Space size={8} wrap>
            <Text type="secondary">原文：</Text>
            <Tooltip title="内部启发式风险分数，非外部AI检测概率">
              <Tag color={originalRateColor}>启发风险 {aiRatePercent(chunk.aiRate)}</Tag>
            </Tooltip>
          </Space>
          <Space size={8} wrap>
            <Text type="secondary">润色后：</Text>
            <Tooltip title="内部启发式风险分数，非外部AI检测概率">
              <Tag color={outputRateColor}>启发风险 {aiRatePercent(chunk.outputAiRate)}</Tag>
            </Tooltip>
            <Tag color={status.color}>{status.label}</Tag>
            <Tag color={state.color}>{state.label}</Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {formatCharDelta(chunk.charDelta)}
            </Text>
          </Space>
        </div>
      }
      extra={
        readOnly ? null : (
          <Space size={8}>
            <Button
              size="small"
              type={chunk.state === 'accepted' ? 'primary' : 'default'}
              loading={busy}
              onClick={onAccept}
            >
              接受
            </Button>
            <Button
              size="small"
              danger={chunk.state === 'rejected'}
              loading={busy}
              onClick={onReject}
            >
              拒绝
            </Button>
          </Space>
        )
      }
    >
      <Space size={8} wrap style={{ marginBottom: 12 }}>
        <Tag bordered={false} color="error">
          原文改动处
        </Tag>
        <Tag bordered={false} color="success">
          润色替换处
        </Tag>
      </Space>
      <Row gutter={16}>
        <Col xs={24} md={12}>
          <div
            style={{
              padding: 12,
              background: '#fff1f0',
              border: '1px solid #ffccc7',
              borderRadius: 4,
            }}
          >
            <Text type="secondary" style={{ fontSize: 11, letterSpacing: 1 }}>
              原文
            </Text>
            <Paragraph style={{ marginTop: 4, marginBottom: 0, whiteSpace: 'pre-wrap' }}>
              {renderHighlightSegments(`${chunk.id}-original`, highlightDiff.original, {
                background: '#ffd6bf',
                color: '#871400',
                padding: 0,
                borderRadius: 2,
              })}
            </Paragraph>
          </div>
        </Col>
        <Col xs={24} md={12}>
          <div
            style={{
              padding: 12,
              background: '#f6ffed',
              border: '1px solid #b7eb8f',
              borderRadius: 4,
            }}
          >
            <Text type="secondary" style={{ fontSize: 11, letterSpacing: 1 }}>
              润色后
            </Text>
            <Paragraph style={{ marginTop: 4, marginBottom: 0, whiteSpace: 'pre-wrap' }}>
              {renderHighlightSegments(`${chunk.id}-rewrite`, highlightDiff.rewritten, {
                background: '#d9f7be',
                color: '#135200',
                padding: 0,
                borderRadius: 2,
              })}
            </Paragraph>
          </div>
        </Col>
      </Row>
    </Card>
  )
}
