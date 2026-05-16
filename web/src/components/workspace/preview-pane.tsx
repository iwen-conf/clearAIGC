import { Empty, Space, Typography } from 'antd'
import { ProCard } from '@ant-design/pro-components'
import type { OutputResponse } from '@/types'

const { Text } = Typography

interface PreviewPaneProps {
  preview: OutputResponse | null
}

export function PreviewPane({ preview }: PreviewPaneProps) {
  if (!preview) {
    return (
      <ProCard style={{ height: '100%' }} styles={{ body: { height: '100%' } }}>
        <div
          style={{
            display: 'flex',
            flex: 1,
            height: '100%',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Empty description="暂无已完成稿件，当某一轮处理完成后,预览将在此显示。" />
        </div>
      </ProCard>
    )
  }

  const paragraphs = preview.text.split('\n\n').filter((item) => item.trim().length > 0)

  return (
    <ProCard style={{ height: '100%' }} styles={{ body: { height: '100%', overflow: 'hidden' } }}>
      <Space direction="vertical" size={12} style={{ width: '100%', height: '100%' }}>
        <Space size={8} data-testid="workspace-preview-meta">
          <Text type="secondary">
            {preview.paragraphCount ?? paragraphs.length} 段
          </Text>
          <Text type="secondary">·</Text>
          <Text type="secondary">已处理 {preview.segmentCount ?? 0} 个片段</Text>
        </Space>
        <article
          data-testid="workspace-preview-body"
          style={{
            flex: 1,
            overflow: 'auto',
            padding: 16,
            border: '1px solid rgba(0,0,0,0.06)',
            borderRadius: 8,
            background: '#fafafa',
          }}
        >
          {paragraphs.map((paragraph, index) => (
            <p key={`${index}-${paragraph.slice(0, 24)}`} style={{ margin: '0 0 12px' }}>
              {paragraph}
            </p>
          ))}
        </article>
      </Space>
    </ProCard>
  )
}
