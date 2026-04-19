import { useState } from 'react'
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
  Typography,
} from 'antd'
import type { ChunkComparison, ChunkReviewState, ChunkStatus, RoundDiffResponse } from '@/types'
import { exportUrl } from '@/api'
import { formatCharDelta } from '@/lib/format'

const { Text, Paragraph } = Typography

interface CardReviewPanelProps {
  comparison: RoundDiffResponse | null
  sessionId: string | null
  roundNumber: number | null
  cardBusyId: string | null
  applyingAll: boolean
  onAccept: (cardId: string) => void
  onReject: (cardId: string) => void
  onApplyAll: () => void
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

  const total = comparison.chunks.length
  const pending = comparison.chunks.filter((c) => c.state === 'pending').length
  const accepted = comparison.chunks.filter((c) => c.state === 'accepted').length
  const rejected = comparison.chunks.filter((c) => c.state === 'rejected').length
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
              <Button
                type="primary"
                loading={applyingAll}
                disabled={pending === 0}
                onClick={onApplyAll}
              >
                全部应用{pending > 0 ? ` (${pending})` : ''}
              </Button>
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

      {comparison.chunks.map((chunk) => (
        <ReviewCard
          key={chunk.id}
          chunk={chunk}
          busy={cardBusyId === chunk.id}
          onAccept={() => onAccept(chunk.id)}
          onReject={() => onReject(chunk.id)}
        />
      ))}
    </Space>
  )
}

interface ReviewCardProps {
  chunk: ChunkComparison
  busy: boolean
  onAccept: () => void
  onReject: () => void
}

function ReviewCard({ chunk, busy, onAccept, onReject }: ReviewCardProps) {
  const state = stateMeta[chunk.state]
  const status = statusMeta[chunk.status]
  const rateColor = aiRateColor(chunk.aiRate)

  return (
    <Card
      size="small"
      data-testid="workspace-review-card"
      style={{
        opacity: chunk.state === 'rejected' ? 0.75 : 1,
        borderColor: chunk.state === 'accepted' ? '#b7eb8f' : undefined,
      }}
      title={
        <Space size={12} wrap>
          <Text strong>
            片段 {chunk.chunkIndex + 1} · 第 {chunk.paragraphIndex + 1} 段
          </Text>
          <Tag color={rateColor}>AI率 {aiRatePercent(chunk.aiRate)}</Tag>
          <Tag color={status.color}>{status.label}</Tag>
          <Tag color={state.color}>{state.label}</Tag>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {formatCharDelta(chunk.charDelta)}
          </Text>
        </Space>
      }
      extra={
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
      }
    >
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
              {chunk.input}
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
              {chunk.output || '—'}
            </Paragraph>
          </div>
        </Col>
      </Row>
    </Card>
  )
}
