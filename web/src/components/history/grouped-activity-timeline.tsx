import { Empty, Space, Timeline, Typography } from 'antd'
import { ProCard } from '@ant-design/pro-components'
import type { TimelineEntry } from '@/types'
import { formatClock } from '@/lib/format'

const { Text } = Typography

interface GroupedActivityTimelineProps {
  entries: TimelineEntry[]
}

const toneColor: Record<TimelineEntry['tone'], string> = {
  neutral: 'blue',
  success: 'green',
  warning: 'orange',
  error: 'red',
}

interface Group {
  key: string
  label: string
  entries: TimelineEntry[]
}

function groupEntries(entries: TimelineEntry[]): Group[] {
  const buckets = new Map<number, TimelineEntry[]>()
  const orderedKeys: number[] = []
  entries.forEach((entry) => {
    const round = entry.round ?? 0
    if (!buckets.has(round)) {
      buckets.set(round, [])
      orderedKeys.push(round)
    }
    buckets.get(round)!.push(entry)
  })
  orderedKeys.sort((a, b) => {
    if (a === 0) return 1
    if (b === 0) return -1
    return a - b
  })
  return orderedKeys.map((round) => ({
    key: String(round),
    label: round === 0 ? '其他活动' : `第 ${round} 轮`,
    entries: [...(buckets.get(round) ?? [])].sort((a, b) => a.timestamp - b.timestamp),
  }))
}

export function GroupedActivityTimeline({ entries }: GroupedActivityTimelineProps) {
  if (entries.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="暂无活动记录。"
      />
    )
  }

  const groups = groupEntries(entries)

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      {groups.map((group) => (
        <ProCard key={group.key} title={group.label} headerBordered size="small">
          <Timeline
            items={group.entries.map((entry) => ({
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
        </ProCard>
      ))}
    </Space>
  )
}
