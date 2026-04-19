import { useState } from 'react'
import {
  Alert,
  Button,
  Collapse,
  Drawer,
  Empty,
  Space,
  Spin,
  Statistic,
  Tag,
  Typography,
} from 'antd'
import { DownloadOutlined, FileSearchOutlined } from '@ant-design/icons'
import type { RoundDiffResponse, RoundHistoryEntry } from '@/types'
import { exportUrl, getDiff } from '@/api'
import { formatDate, formatDuration, humanStatus, statusTone, toErrorMessage } from '@/lib/format'
import { CardReviewPanel } from '@/components/workspace/card-review-panel'

const { Text } = Typography

interface RoundHistoryListProps {
  sessionId: string
  rounds: RoundHistoryEntry[]
}

export function RoundHistoryList({ sessionId, rounds }: RoundHistoryListProps) {
  const [openRound, setOpenRound] = useState<number | null>(null)
  const [diff, setDiff] = useState<RoundDiffResponse | null>(null)
  const [diffLoading, setDiffLoading] = useState(false)
  const [diffError, setDiffError] = useState<string | null>(null)

  if (rounds.length === 0) {
    return (
      <Empty
        description="这份文档还没有任何处理记录。"
        image={Empty.PRESENTED_IMAGE_SIMPLE}
      />
    )
  }

  async function openDiff(round: number) {
    setOpenRound(round)
    setDiff(null)
    setDiffError(null)
    setDiffLoading(true)
    try {
      const response = await getDiff(sessionId, round)
      setDiff(response)
    } catch (err) {
      setDiffError(toErrorMessage(err))
    } finally {
      setDiffLoading(false)
    }
  }

  function closeDiff() {
    setOpenRound(null)
    setDiff(null)
    setDiffError(null)
  }

  return (
    <>
      <Collapse
        accordion
        items={rounds.map((entry) => {
          const round = entry.round
          const summary = entry.summary
          const tone = statusTone(round.status)
          const toneColor = tone === 'error' ? 'red' : tone === 'warning' ? 'orange' : tone === 'success' ? 'green' : 'default'
          return {
            key: String(round.number),
            label: (
              <Space size={12} wrap>
                <Text strong>第 {round.number} 轮</Text>
                <Tag color={toneColor}>{humanStatus(round.status)}</Tag>
                <Text type="secondary">{round.providerUsed || '—'}</Text>
                <Text type="secondary">{formatDuration(summary.durationSeconds)}</Text>
                <Text type="secondary">{round.totalTokens.toLocaleString()} tokens</Text>
              </Space>
            ),
            children: (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Space size={24} wrap>
                  <Statistic title="片段总数" value={summary.chunkCount} />
                  <Statistic title="一次通过" value={summary.passedChunks} />
                  <Statistic
                    title="二次复核"
                    value={summary.recoveredChunks}
                    valueStyle={{ color: summary.recoveredChunks > 0 ? '#faad14' : undefined }}
                  />
                  <Statistic
                    title="需人工复核"
                    value={summary.failedChunks}
                    valueStyle={{ color: summary.failedChunks > 0 ? '#ff4d4f' : undefined }}
                  />
                  <Statistic
                    title="评分"
                    value={summary.scoreTotal ?? '—'}
                    precision={summary.scoreTotal ? 0 : undefined}
                  />
                </Space>
                <Space size={12} wrap>
                  <Text type="secondary">开始于 {round.startedAt ? formatDate(round.startedAt) : '—'}</Text>
                  <Text type="secondary">完成于 {round.completedAt ? formatDate(round.completedAt) : '—'}</Text>
                </Space>
                {round.recoveryJustification ? (
                  <Alert
                    type={round.status === 'failed' ? 'error' : 'info'}
                    showIcon
                    message="复核说明"
                    description={round.recoveryJustification}
                  />
                ) : null}
                <Space size={8} wrap>
                  <Button
                    icon={<DownloadOutlined />}
                    href={exportUrl(sessionId, round.number, 'txt')}
                  >
                    下载 TXT
                  </Button>
                  <Button
                    type="primary"
                    icon={<DownloadOutlined />}
                    href={exportUrl(sessionId, round.number, 'docx')}
                  >
                    下载 DOCX
                  </Button>
                  <Button
                    icon={<FileSearchOutlined />}
                    onClick={() => openDiff(round.number)}
                  >
                    查看对比
                  </Button>
                </Space>
              </Space>
            ),
          }
        })}
      />

      <Drawer
        title={openRound ? `第 ${openRound} 轮 · 片段对比` : '片段对比'}
        open={openRound !== null}
        width={Math.min(960, typeof window !== 'undefined' ? window.innerWidth - 48 : 720)}
        onClose={closeDiff}
        destroyOnClose
      >
        {diffLoading ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: 48 }}>
            <Spin tip="正在加载对比…" />
          </div>
        ) : diffError ? (
          <Alert type="error" showIcon message="无法加载对比" description={diffError} />
        ) : (
          <CardReviewPanel
            comparison={diff}
            sessionId={sessionId}
            roundNumber={openRound}
            cardBusyId={null}
            applyingAll={false}
            readOnly
            onAccept={() => undefined}
            onReject={() => undefined}
            onApplyAll={() => undefined}
          />
        )}
      </Drawer>
    </>
  )
}
