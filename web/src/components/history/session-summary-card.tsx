import { Card, Descriptions, Space, Typography } from 'antd'
import type { ProgressPayload, Session, SessionListMetrics } from '@/types'
import { StatusPill } from '@/components/workspace/status-pill'
import { formatBytes, formatDate, promptProfileLabel } from '@/lib/format'

const { Text } = Typography

interface SessionSummaryCardProps {
  session: Session
  metrics: SessionListMetrics
  progress: ProgressPayload | null
}

export function SessionSummaryCard({ session, metrics, progress }: SessionSummaryCardProps) {
  return (
    <Card
      size="small"
      title={
        <Space direction="vertical" size={2}>
          <Text strong>{session.documentName || '未命名文档'}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {session.docId}
          </Text>
        </Space>
      }
      extra={<StatusPill status={session.status} />}
    >
      <Descriptions size="small" column={{ xs: 1, sm: 2, lg: 3 }}>
        <Descriptions.Item label="文件格式">
          {(session.fileFormat || 'txt').toUpperCase()}
        </Descriptions.Item>
        <Descriptions.Item label="文件大小">{formatBytes(session.fileSizeBytes)}</Descriptions.Item>
        <Descriptions.Item label="润色模式">
          {promptProfileLabel(session.promptProfile)}
        </Descriptions.Item>
        <Descriptions.Item label="创建时间">{formatDate(session.createdAt)}</Descriptions.Item>
        <Descriptions.Item label="最后更新">{formatDate(session.updatedAt)}</Descriptions.Item>
        <Descriptions.Item label="轮次">
          {metrics.completedRounds} / {metrics.totalRounds || '—'}
        </Descriptions.Item>
        <Descriptions.Item label="累计 Tokens">{metrics.totalTokens.toLocaleString()}</Descriptions.Item>
        <Descriptions.Item label="当前阶段">
          {progress ? progress.phase : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="最新进度">
          {progress ? `${progress.completedChunks}/${progress.totalChunks} · ${Math.round(progress.percent ?? 0)}%` : '—'}
        </Descriptions.Item>
      </Descriptions>
    </Card>
  )
}
