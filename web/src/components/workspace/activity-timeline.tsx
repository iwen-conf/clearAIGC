import { Empty, Space, Timeline, Typography } from 'antd'
import { ClockCircleOutlined } from '@ant-design/icons'
import { ProCard } from '@ant-design/pro-components'
import type { TimelineEntry } from '@/types'
import { formatClock } from '@/lib/format'

const { Text } = Typography

interface ActivityTimelineProps {
  entries: TimelineEntry[]
}

const toneColor: Record<TimelineEntry['tone'], string> = {
  neutral: 'blue',
  success: 'green',
  warning: 'orange',
  error: 'red',
}

export function ActivityTimeline({ entries }: ActivityTimelineProps) {
  return (
    <ProCard
      title={
        <Space size={8}>
          <ClockCircleOutlined />
          活动记录
        </Space>
      }
      headerBordered
      size="small"
    >
      {entries.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={<Text type="secondary">处理开始后,动态将在此显示</Text>}
        />
      ) : (
        <Timeline
          items={entries.map((entry) => ({
            key: entry.id,
            color: toneColor[entry.tone],
            children: (
              <Space direction="vertical" size={2} style={{ width: '100%' }}>
                <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                  <Text strong style={{ fontSize: 13 }}>
                    {entry.title}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {formatClock(entry.timestamp)}
                  </Text>
                </Space>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {entry.detail}
                </Text>
              </Space>
            ),
          }))}
        />
      )}
    </ProCard>
  )
}
