import { useState } from 'react'
import {
  Alert,
  Button,
  Segmented,
  Space,
  Typography,
  Upload,
} from 'antd'
import type { UploadProps } from 'antd'
import { InboxOutlined, FileTextOutlined } from '@ant-design/icons'
import { ProCard } from '@ant-design/pro-components'
import type { PromptProfile } from '@/types'
import { formatBytes } from '@/lib/format'

const { Title, Paragraph, Text } = Typography
const { Dragger } = Upload

interface UploadCardProps {
  busy: boolean
  error: string | null
  message: string | null
  onSubmit: (file: File, profile: PromptProfile) => Promise<void> | void
}

export function UploadCard({ busy, error, message, onSubmit }: UploadCardProps) {
  const [file, setFile] = useState<File | null>(null)
  const [profile, setProfile] = useState<PromptProfile>('cn')
  const [localMessage, setLocalMessage] = useState<string | null>(null)

  const draggerProps: UploadProps = {
    multiple: false,
    maxCount: 1,
    accept: '.txt,.docx',
    showUploadList: false,
    beforeUpload(next) {
      setFile(next)
      setLocalMessage(`${next.name} 已就绪,可以上传。`)
      return false
    },
    onRemove() {
      setFile(null)
      setLocalMessage(null)
    },
  }

  async function handleSubmit() {
    if (!file) {
      setLocalMessage('请先选择一个 .txt 或 .docx 文件。')
      return
    }
    setLocalMessage(null)
    await onSubmit(file, profile)
  }

  const notice = error ?? message ?? localMessage

  return (
    <ProCard
      style={{ maxWidth: 860, width: '100%', margin: '0 auto' }}
      headerBordered
      title={
        <Space direction="vertical" size={2}>
          <Text type="secondary" style={{ fontSize: 12, letterSpacing: 1 }}>
            让学术与技术写作更自然
          </Text>
          <Title level={3} style={{ margin: 0 }}>
            将 AI 生成的稿件润色为自然、像人写的文档。
          </Title>
          <Paragraph type="secondary" style={{ margin: 0 }}>
            上传论文初稿或技术文档，Naturalize 会按段落分块改写,保留原有结构与术语,并实时展示处理进度。
          </Paragraph>
        </Space>
      }
    >
      <Space direction="vertical" size={20} style={{ width: '100%' }}>
        <Dragger
          data-testid="workspace-file-input"
          {...draggerProps}
        >
          <p className="ant-upload-drag-icon">
            {file ? <FileTextOutlined /> : <InboxOutlined />}
          </p>
          <p className="ant-upload-text">
            {file ? file.name : '将 .txt 或 .docx 文件拖到此处，或点击选择'}
          </p>
          <p className="ant-upload-hint">
            {file
              ? `${formatBytes(file.size)} · 已就绪`
              : '最大 50 MB · 适合论文稿件与技术文档'}
          </p>
        </Dragger>

        <div>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>
            润色模式
          </Text>
          <Segmented<PromptProfile>
            block
            value={profile}
            onChange={(value) => setProfile(value)}
            options={[
              {
                value: 'cn',
                label: (
                  <span data-testid="workspace-mode-cn">
                    <div style={{ fontSize: 14, fontWeight: 600 }}>中文模式</div>
                    <div style={{ fontSize: 12, opacity: 0.65 }}>两轮润色 · 自然学术中文</div>
                  </span>
                ),
              },
              {
                value: 'en',
                label: (
                  <span data-testid="workspace-mode-en">
                    <div style={{ fontSize: 14, fontWeight: 600 }}>英文模式</div>
                    <div style={{ fontSize: 12, opacity: 0.65 }}>单轮润色 · 自然学术英文</div>
                  </span>
                ),
              },
            ]}
          />
        </div>

        <Space>
          <Button
            type="primary"
            loading={busy}
            disabled={!file}
            onClick={handleSubmit}
            data-testid="workspace-primary-action"
          >
            {busy ? '处理中…' : '上传并开始'}
          </Button>
          <Button
            onClick={() => {
              setFile(null)
              setLocalMessage(null)
            }}
            disabled={busy || !file}
          >
            清除
          </Button>
        </Space>

        {notice ? (
          <Alert
            showIcon
            type={error ? 'error' : 'info'}
            message={error ? '出现问题' : '提示'}
            description={notice}
          />
        ) : null}
      </Space>
    </ProCard>
  )
}
