import {
  Button,
  Divider,
  Space,
  Tooltip,
  Typography,
} from 'antd'
import {
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  DownloadOutlined,
  DeleteOutlined,
} from '@ant-design/icons'
import { ProCard } from '@ant-design/pro-components'
import { StatusPill } from './status-pill'
import { exportUrl } from '@/api'
import type { Session } from '@/types'
import { formatBytes, formatDate } from '@/lib/format'

const { Title, Text } = Typography

interface SessionHeaderProps {
  session: Session
  busy: boolean
  completedPassCount: number
  totalPasses: number
  latestCompletedRoundNumber: number | null
  onPause: () => void
  onResume: () => void
  onStartNext: () => void
  onRefresh: () => void
  onReset: () => void
}

export function SessionHeader({
  session,
  busy,
  completedPassCount,
  totalPasses,
  latestCompletedRoundNumber,
  onPause,
  onResume,
  onStartNext,
  onRefresh,
  onReset,
}: SessionHeaderProps) {
  const primaryLabel =
    session.status === 'paused'
      ? '继续润色'
      : completedPassCount === 0
        ? '开始润色'
        : completedPassCount < totalPasses
          ? '开始最终润色'
          : '已完成'

  const primaryAction = () => {
    if (session.status === 'paused') return onResume()
    if (session.status === 'pending') return onStartNext()
  }

  const primaryDisabled =
    busy ||
    session.status === 'completed' ||
    session.status === 'processing' ||
    session.status === 'failed'

  return (
    <ProCard style={{ marginBottom: 16 }}>
      <Space
        align="start"
        style={{ width: '100%', justifyContent: 'space-between' }}
        wrap
      >
        <Space direction="vertical" size={6} style={{ minWidth: 0 }}>
          <Space size={8} align="center">
            <Text type="secondary" style={{ fontSize: 12, letterSpacing: 1 }}>
              会话
            </Text>
            <Text code>{session.id.slice(0, 8)}</Text>
          </Space>
          <Title level={4} style={{ margin: 0 }} ellipsis={{ tooltip: session.documentName }}>
            {session.documentName}
          </Title>
          <Space size={4} wrap split={<Divider type="vertical" style={{ margin: 0 }} />}>
            <StatusPill status={session.status} data-testid="workspace-status-pill" />
            <Text type="secondary">
              {session.promptProfile === 'cn' ? '中文模式' : '英文模式'}
            </Text>
            <Text type="secondary">{formatBytes(session.fileSizeBytes)}</Text>
            <Text type="secondary">创建于 {formatDate(session.createdAt)}</Text>
            <Text type="secondary">
              第 {Math.max(latestCompletedRoundNumber ?? 0, completedPassCount)}/{totalPasses} 轮
            </Text>
          </Space>
        </Space>

        <Space wrap>
          {session.status === 'processing' ? (
            <Button icon={<PauseCircleOutlined />} onClick={onPause} disabled={busy}>
              暂停
            </Button>
          ) : session.status !== 'completed' && session.status !== 'failed' ? (
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={primaryAction}
              loading={busy}
              disabled={primaryDisabled}
              data-testid="workspace-primary-action"
            >
              {primaryLabel}
            </Button>
          ) : null}

          <Tooltip title="刷新会话">
            <Button icon={<ReloadOutlined />} onClick={onRefresh} disabled={busy}>
              刷新
            </Button>
          </Tooltip>

          {latestCompletedRoundNumber !== null ? (
            <>
              <Button
                icon={<DownloadOutlined />}
                href={exportUrl(session.id, latestCompletedRoundNumber, 'txt')}
                data-testid="workspace-download-txt"
              >
                下载 TXT
              </Button>
              <Button
                icon={<DownloadOutlined />}
                href={exportUrl(session.id, latestCompletedRoundNumber, 'docx')}
                data-testid="workspace-download-docx"
              >
                下载 DOCX
              </Button>
            </>
          ) : null}

          <Button danger type="text" icon={<DeleteOutlined />} onClick={onReset}>
            新建文档
          </Button>
        </Space>
      </Space>
    </ProCard>
  )
}
