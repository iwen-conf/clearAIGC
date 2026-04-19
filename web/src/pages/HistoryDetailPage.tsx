import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Alert,
  Button,
  Progress,
  Result,
  Space,
  Spin,
  Tabs,
  message,
} from 'antd'
import { PageContainer } from '@ant-design/pro-components'
import { ArrowLeftOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import { useHistoryDetail } from '@/hooks/use-history-detail'
import { useSession } from '@/hooks/use-session'
import { SessionSummaryCard } from '@/components/history/session-summary-card'
import { RoundHistoryList } from '@/components/history/round-history-list'
import { GroupedActivityTimeline } from '@/components/history/grouped-activity-timeline'
import type { SessionStatus } from '@/types'
import { toErrorMessage } from '@/lib/format'

const RESUMABLE_STATUSES: SessionStatus[] = ['pending', 'paused', 'processing', 'failed']

export default function HistoryDetailPage() {
  const params = useParams<{ sessionId: string }>()
  const sessionId = params.sessionId
  const navigate = useNavigate()
  const sessionCtx = useSession()
  const [messageApi, messageContext] = message.useMessage()
  const [activeTab, setActiveTab] = useState<'rounds' | 'timeline'>('rounds')

  const { data, loading, error, reload } = useHistoryDetail(sessionId)

  const showNotFound = useMemo(
    () => !loading && !data && error && /not found|没找到|不存在/i.test(error),
    [loading, data, error],
  )

  if (!sessionId) {
    return (
      <PageContainer>
        <Result
          status="404"
          title="缺少会话 ID"
          subTitle="URL 中没有提供 sessionId。"
          extra={
            <Button type="primary" onClick={() => navigate('/history')}>
              返回历史列表
            </Button>
          }
        />
      </PageContainer>
    )
  }

  if (loading && !data) {
    return (
      <PageContainer>
        <div style={{ display: 'flex', justifyContent: 'center', padding: 64 }}>
          <Spin tip="正在加载历史详情…" />
        </div>
      </PageContainer>
    )
  }

  if (showNotFound) {
    return (
      <PageContainer>
        <Result
          status="404"
          title="找不到这份文档"
          subTitle={error ?? undefined}
          extra={
            <Button type="primary" onClick={() => navigate('/history')}>
              返回历史列表
            </Button>
          }
        />
      </PageContainer>
    )
  }

  if (!data) {
    return (
      <PageContainer>
        <Alert
          type="error"
          showIcon
          message="无法加载历史详情"
          description={error ?? '请求未能完成,请稍后重试。'}
          action={
            <Button size="small" onClick={reload}>
              重试
            </Button>
          }
        />
      </PageContainer>
    )
  }

  const canResume = RESUMABLE_STATUSES.includes(data.session.status)
  const showLiveProgress =
    data.session.status !== 'completed' &&
    data.progress !== null &&
    Number.isFinite(data.progress.percent)

  async function handleResume() {
    if (!sessionId) return
    try {
      await sessionCtx.adopt(sessionId)
      messageApi.success('已加载历史会话。')
      navigate('/')
    } catch (err) {
      messageApi.error(toErrorMessage(err))
    }
  }

  return (
    <PageContainer
      header={{
        title: data.session.documentName || '未命名文档',
        onBack: () => navigate('/history'),
        backIcon: <ArrowLeftOutlined />,
      }}
      extra={[
        <Button key="reload" icon={<ReloadOutlined />} onClick={reload} loading={loading}>
          刷新
        </Button>,
        canResume ? (
          <Button
            key="resume"
            type="primary"
            icon={<PlayCircleOutlined />}
            onClick={handleResume}
          >
            继续处理
          </Button>
        ) : null,
      ].filter(Boolean)}
    >
      {messageContext}

      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {error ? (
          <Alert type="warning" showIcon message="部分数据加载失败" description={error} />
        ) : null}

        <SessionSummaryCard
          session={data.session}
          metrics={data.metrics}
          progress={data.progress}
        />

        {showLiveProgress && data.progress ? (
          <Progress
            percent={Math.round(data.progress.percent)}
            status={data.session.status === 'failed' ? 'exception' : 'active'}
          />
        ) : null}

        <Tabs
          activeKey={activeTab}
          onChange={(key) => setActiveTab(key as 'rounds' | 'timeline')}
          items={[
            {
              key: 'rounds',
              label: `处理轮次 (${data.rounds.length})`,
              children: (
                <RoundHistoryList sessionId={sessionId} rounds={data.rounds} />
              ),
            },
            {
              key: 'timeline',
              label: `活动时间线 (${data.timeline.length})`,
              children: <GroupedActivityTimeline entries={data.timeline} />,
            },
          ]}
        />

        <Space>
          <Button onClick={() => navigate('/history')}>回到列表</Button>
          {canResume ? (
            <Button type="primary" onClick={handleResume}>
              继续处理
            </Button>
          ) : null}
        </Space>
      </Space>
    </PageContainer>
  )
}
