import { useState, type CSSProperties } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  Descriptions,
  Empty,
  Divider,
  Progress,
  Row,
  Segmented,
  Space,
  Statistic,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import type { ChunkComparison, ChunkReviewState, ChunkStatus, RoundDiffResponse, ScoreSignal } from '@/types'
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

function aiRateColor(rate: number, green = 0.33, red = 0.66): string {
  if (rate >= red) return 'error'
  if (rate >= green) return 'warning'
  return 'default'
}

function calibratedRate(chunk: NormalizedChunkComparison, kind: 'input' | 'output'): number {
  const score = kind === 'input' ? chunk.score : chunk.outputScore
  return score?.calibrated?.score ?? (kind === 'input' ? chunk.aiRate : chunk.outputAiRate)
}

function thresholdGreen(score: ChunkComparison['score']): number {
  return score?.thresholdGreen ?? 0.33
}

function thresholdRed(score: ChunkComparison['score']): number {
  return score?.thresholdRed ?? 0.66
}

function scorePercent(score: ChunkComparison['score']): number {
  if (!score) return 0
  return Math.round((score.calibrated?.score ?? score.total) * 100)
}

function formatSimilarity(value: number | undefined): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '—'
  return `${Math.round(value * 100)}%`
}

function clampSignal(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(1, value))
}

function polarPoint(index: number, total: number, radius: number, center: number) {
  const angle = -Math.PI / 2 + (index / total) * Math.PI * 2
  return {
    x: center + Math.cos(angle) * radius,
    y: center + Math.sin(angle) * radius,
  }
}

function axisTextAnchor(x: number, center: number): 'start' | 'middle' | 'end' {
  if (x < center - 6) return 'end'
  if (x > center + 6) return 'start'
  return 'middle'
}

function axisBaseline(y: number, center: number): 'auto' | 'hanging' | 'middle' {
  if (y < center - 16) return 'auto'
  if (y > center + 16) return 'hanging'
  return 'middle'
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
    detector: chunk.detector?.trim() || '内部启发式风险评估',
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
      <Alert
        showIcon
        type="info"
        message="风险分说明"
        description="下方风险分来自内部多信号评分器；配置参考检测器后会做校准，但仍只用于比较版本间风险变化，不等同于第三方检测服务的官方结论。"
      />
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
  const originalRateColor = aiRateColor(calibratedRate(chunk, 'input'), thresholdGreen(chunk.score), thresholdRed(chunk.score))
  const outputRateColor = aiRateColor(calibratedRate(chunk, 'output'), thresholdGreen(chunk.outputScore), thresholdRed(chunk.outputScore))
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
            <Tooltip title="用于辅助比较原文与润色结果的内部规则，不代表外部检测器结论">
              <Text type="secondary">评估方式：{chunk.detector}</Text>
            </Tooltip>
          </Space>
          <Space size={8} wrap>
            <Text type="secondary">原文：</Text>
            <Tooltip title="内部多信号风险分；配置参考检测器后会做校准，但不等同于第三方官方概率">
              <Tag color={originalRateColor}>校准后 {aiRatePercent(calibratedRate(chunk, 'input'))}</Tag>
            </Tooltip>
            {chunk.score?.calibrated?.mode ? <Tag>{chunk.score.calibrated.mode}</Tag> : null}
          </Space>
          <Space size={8} wrap>
            <Text type="secondary">润色后：</Text>
            <Tooltip title="内部多信号风险分；配置参考检测器后会做校准，但不等同于第三方官方概率">
              <Tag color={outputRateColor}>校准后 {aiRatePercent(calibratedRate(chunk, 'output'))}</Tag>
            </Tooltip>
            {chunk.outputScore?.target ? <Tag>目标 {Math.round(chunk.outputScore.target * 100)}%</Tag> : null}
            <Tag color={status.color}>{status.label}</Tag>
            <Tag color={state.color}>{state.label}</Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {formatCharDelta(chunk.charDelta)}
            </Text>
          </Space>
          {chunk.outputScore?.signals?.length ? (
            <Space size={8} wrap>
              {chunk.outputScore.signals.map((signal) => (
                <Tooltip key={signal.name} title={signal.description ?? signal.label}>
                  <Tag>{signal.label} {Math.round(signal.normalized * 100)}%</Tag>
                </Tooltip>
              ))}
            </Space>
          ) : null}
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
      <Divider style={{ margin: '16px 0 12px' }}>分析面板</Divider>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <ScorePanel title="原文评分" score={chunk.score} fallbackRate={chunk.aiRate} tagColor={originalRateColor} />
        </Col>
        <Col xs={24} lg={12}>
          <ScorePanel title="润色后评分" score={chunk.outputScore} fallbackRate={chunk.outputAiRate} tagColor={outputRateColor} />
        </Col>
      </Row>
      {chunk.checks.length > 0 ? (
        <>
          <Divider style={{ margin: '16px 0 12px' }}>质量门控</Divider>
          <Space size={8} wrap>
            {chunk.checks.map((check) => (
              <Tooltip key={`${chunk.id}-${check.type}`} title={check.reason || check.type}>
                <Tag color={check.passed ? 'success' : 'error'}>
                  {check.type}
                </Tag>
              </Tooltip>
            ))}
          </Space>
        </>
      ) : null}
      {chunk.sentences && chunk.sentences.length > 0 ? (
        <>
          <Divider style={{ margin: '16px 0 12px' }}>句级决策</Divider>
          <Collapse
            items={chunk.sentences.map((sentence) => ({
              key: `${chunk.id}-sentence-${sentence.index}`,
              label: (
                <Space size={8} wrap>
                  <Text strong>句子 {sentence.index + 1}</Text>
                  <Tag color={sentence.accepted ? 'success' : 'default'}>
                    {sentence.accepted ? '采用候选' : '保留原文'}
                  </Tag>
                  <Text type="secondary">原分 {scorePercent(sentence.original)}%</Text>
                  <Text type="secondary">结果 {scorePercent(sentence.final)}%</Text>
                </Space>
              ),
              children: (
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  <Text>{sentence.accepted ? sentence.output : sentence.input}</Text>
                  <Text type="secondary">原因：{sentence.reason ?? '—'}</Text>
                  {sentence.candidates && sentence.candidates.length > 0 ? (
                    <Collapse
                      size="small"
                      items={sentence.candidates.map((candidate) => ({
                        key: `${chunk.id}-sentence-${sentence.index}-${candidate.id}`,
                        label: (
                          <Space size={8} wrap>
                            <Text strong>{candidate.label}</Text>
                            <Tag>{candidate.worker}</Tag>
                            <Tag color={candidate.accepted ? 'success' : 'default'}>
                              {candidate.accepted ? '选中' : candidate.reason ?? '未选中'}
                            </Tag>
                            <Text type="secondary">loss {candidate.loss.toFixed(3)}</Text>
                            <Text type="secondary">相似度 {formatSimilarity(candidate.similarity)}</Text>
                          </Space>
                        ),
                        children: (
                          <Space direction="vertical" size={8} style={{ width: '100%' }}>
                            <Text>{candidate.output}</Text>
                            <Descriptions size="small" column={2}>
                              <Descriptions.Item label="候选分数">
                                {scorePercent(candidate.score)}%
                              </Descriptions.Item>
                              <Descriptions.Item label="Tokens">
                                {candidate.tokenCost}
                              </Descriptions.Item>
                              <Descriptions.Item label="相似度来源">
                                {candidate.similarityMode || '—'}
                              </Descriptions.Item>
                              <Descriptions.Item label="原因">
                                {candidate.reason || '—'}
                              </Descriptions.Item>
                            </Descriptions>
                            <ScorePanel title="候选评分" score={candidate.score} fallbackRate={candidate.score.total} tagColor={aiRateColor(candidate.score.calibrated?.score ?? candidate.score.total, thresholdGreen(candidate.score), thresholdRed(candidate.score))} compact />
                          </Space>
                        ),
                      }))}
                    />
                  ) : null}
                </Space>
              ),
            }))}
          />
        </>
      ) : null}
    </Card>
  )
}

function SignalRadar({ signals }: { signals: ScoreSignal[] }) {
  if (signals.length < 3) {
    return null
  }

  const size = 220
  const center = 110
  const radius = 62
  const rings = [0.25, 0.5, 0.75, 1]

  const polygonPoints = signals
    .map((signal, index) => {
      const point = polarPoint(index, signals.length, radius*clampSignal(signal.normalized), center)
      return `${point.x},${point.y}`
    })
    .join(' ')

  return (
    <div
      style={{
        border: '1px solid #f0f0f0',
        borderRadius: 8,
        background: '#fafafa',
        padding: 12,
      }}
    >
      <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
        四路信号雷达
      </Text>
      <svg viewBox={`0 0 ${size} ${size}`} style={{ width: '100%', height: 220 }}>
        {rings.map((ring) => (
          <polygon
            key={`ring-${ring}`}
            points={signals
              .map((_, index) => {
                const point = polarPoint(index, signals.length, radius * ring, center)
                return `${point.x},${point.y}`
              })
              .join(' ')}
            fill="none"
            stroke="#e5e7eb"
            strokeWidth="1"
          />
        ))}
        {signals.map((signal, index) => {
          const axis = polarPoint(index, signals.length, radius, center)
          const label = polarPoint(index, signals.length, radius + 22, center)
          return (
            <g key={`axis-${signal.name}`}>
              <line x1={center} y1={center} x2={axis.x} y2={axis.y} stroke="#d9d9d9" strokeWidth="1" />
              <text
                x={label.x}
                y={label.y}
                fill="#595959"
                fontSize="11"
                textAnchor={axisTextAnchor(label.x, center)}
                dominantBaseline={axisBaseline(label.y, center)}
              >
                {signal.label}
              </text>
            </g>
          )
        })}
        <polygon points={polygonPoints} fill="rgba(22, 119, 255, 0.18)" stroke="#1677ff" strokeWidth="2" />
        {signals.map((signal, index) => {
          const point = polarPoint(index, signals.length, radius*clampSignal(signal.normalized), center)
          return <circle key={`point-${signal.name}`} cx={point.x} cy={point.y} r="4" fill="#0958d9" />
        })}
      </svg>
    </div>
  )
}

interface ScorePanelProps {
  title: string
  score?: ChunkComparison['score']
  fallbackRate: number
  tagColor: string
  compact?: boolean
}

function ScorePanel({ title, score, fallbackRate, tagColor, compact = false }: ScorePanelProps) {
  const effectiveScore = score?.calibrated?.score ?? score?.total ?? fallbackRate
  return (
    <Card size="small" title={title}>
      <Space direction="vertical" size={compact ? 8 : 12} style={{ width: '100%' }}>
        <Space size={8} wrap>
          <Tag color={tagColor}>校准后 {aiRatePercent(effectiveScore)}</Tag>
          {score ? <Tag>原始总分 {Math.round(score.total * 100)}%</Tag> : null}
          {score?.target ? <Tag>目标 {Math.round(score.target * 100)}%</Tag> : null}
        </Space>
        {!compact && score?.signals?.length ? <SignalRadar signals={score.signals} /> : null}
        {score?.calibrated ? (
          <Descriptions size="small" column={2}>
            <Descriptions.Item label="参考检测器">{score.calibrated.detector || '—'}</Descriptions.Item>
            <Descriptions.Item label="校准模式">{score.calibrated.mode || '—'}</Descriptions.Item>
            <Descriptions.Item label="相关性">
              {Math.round(score.calibrated.correlation * 100)}%
            </Descriptions.Item>
            <Descriptions.Item label="样本数">
              {score.calibrated.sampleCount ?? 0}
            </Descriptions.Item>
            <Descriptions.Item label="绿区阈值">
              {Math.round((score.thresholdGreen ?? 0.33) * 100)}%
            </Descriptions.Item>
            <Descriptions.Item label="红区阈值">
              {Math.round((score.thresholdRed ?? 0.66) * 100)}%
            </Descriptions.Item>
          </Descriptions>
        ) : null}
        {score?.signals?.length ? (
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            {score.signals.map((signal) => (
              <div key={`${title}-${signal.name}`}>
                <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                  <Tooltip title={signal.description ?? signal.label}>
                    <Text>{signal.label}</Text>
                  </Tooltip>
                  <Text type="secondary">{Math.round(signal.normalized * 100)}%</Text>
                </Space>
                <Progress percent={Math.round(signal.normalized * 100)} showInfo={false} size="small" />
              </div>
            ))}
          </Space>
        ) : null}
        {score?.features?.length ? (
          <Space size={6} wrap>
            {score.features.map((feature) => (
              <Tag key={`${title}-feature-${feature}`}>{feature}</Tag>
            ))}
          </Space>
        ) : null}
        {score?.forbidden?.length ? (
          <Space size={6} wrap>
            {score.forbidden.map((phrase) => (
              <Tag key={`${title}-forbidden-${phrase}`} color="volcano">
                {phrase}
              </Tag>
            ))}
          </Space>
        ) : null}
      </Space>
    </Card>
  )
}
