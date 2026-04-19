import { useState } from 'react'
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  Row,
  Select,
  Space,
  Tag,
  Typography,
} from 'antd'
import { KeyOutlined, ReloadOutlined } from '@ant-design/icons'
import { ProCard } from '@ant-design/pro-components'
import { fetchAgentModels, updateAgent } from '@/api'
import type { AgentName, AgentProtocol, AgentSetting, AgentSettingInput } from '@/types'
import { toErrorMessage } from '@/lib/format'

const { Paragraph, Text } = Typography

interface AgentCardProps {
  agent: AgentSetting
  onSaved: (next: AgentSetting) => void
}

interface AgentProfile {
  role: string
  summary: string
  strengths: string[]
  modelHint: string
  protocolHint: string
  tone: 'processing' | 'success' | 'warning'
}

const AGENT_PROFILES: Record<AgentName, AgentProfile> = {
  coordinator: {
    role: '流程调度者 · Workflow Controller',
    summary:
      '只做规划,不做改写。读取片段后分析 AI 风格信号,按需调用 lexical_mutator / syntax_rebuilder,并最终提交定稿。',
    strengths: ['任务编排', '工具调用 (Function Calling)', '多步 ReAct 决策'],
    modelHint:
      '必选支持 Tool/Function Calling 的模型,推荐 GPT-4o / Claude Sonnet 4 / Gemini 2.5 Pro 等强推理型号。',
    protocolHint: '走 Chat Completions 或 Responses API 均可,但必须保证 tools 字段生效。',
    tone: 'processing',
  },
  lexical_mutator: {
    role: '词汇改写者 · Lexical Analyst',
    summary:
      '针对单个片段做词汇级改写。扫描 "Moreover" "crucial" 等高概率 AI 词,替换为更人类化、长尾的学术表达。',
    strengths: ['Diff-Patch JSON 输出', '保持原句结构', '指令遵循严格'],
    modelHint:
      '推荐指令跟随稳定、JSON 可控的中等规模模型,如 GPT-4o-mini / Claude Haiku / Gemini 2.5 Flash。',
    protocolHint: '无需工具调用,Chat Completions 即可。',
    tone: 'success',
  },
  syntax_rebuilder: {
    role: '句法重塑者 · Structure Architect',
    summary:
      '针对单个片段做句法级改写。打破 AI 的均匀句长,通过拆分长句、合并短句、主被动转换注入 burstiness,但严禁改变事实。',
    strengths: ['长短句重组', '语体变换', '保持论证链条'],
    modelHint:
      '推荐擅长长文本、语体表现力强的模型,如 Claude Sonnet 4 / GPT-4.1 / DeepSeek-V3。',
    protocolHint: '无需工具调用,Chat Completions 即可。',
    tone: 'warning',
  },
}

export function AgentCard({ agent, onSaved }: AgentCardProps) {
  const profile = AGENT_PROFILES[agent.name]
  const initialModel = agent.model ?? ''
  const [protocol, setProtocol] = useState<AgentProtocol>(agent.protocol || 'chat')
  const [baseUrl, setBaseUrl] = useState(agent.baseUrl ?? '')
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState(initialModel)
  const [models, setModels] = useState<string[]>(initialModel ? [initialModel] : [])
  const [loadingModels, setLoadingModels] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const maskedKey = agent.apiKey ?? ''

  function payload(): AgentSettingInput {
    return {
      protocol,
      baseUrl: baseUrl.trim(),
      apiKey: apiKey.trim(),
      model: model.trim(),
    }
  }

  async function handleFetchModels() {
    setLoadingModels(true)
    setError(null)
    setMessage(null)
    try {
      const list = await fetchAgentModels(agent.name, payload())
      setModels(list)
      if (list.length === 0) {
        setMessage('未获取到可用模型。')
      } else if (!model || !list.includes(model)) {
        setModel(list[0])
      }
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setLoadingModels(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const next = await updateAgent(agent.name, payload())
      onSaved(next)
      setApiKey('')
      setMessage('配置已保存,将应用到后续处理。')
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <ProCard
      headerBordered
      title={
        <Space size={12} align="center" wrap>
          <Text strong style={{ fontSize: 16 }}>
            {agent.displayName}
          </Text>
          <Tag color={profile.tone}>{profile.role}</Tag>
          <Text code style={{ fontSize: 12 }}>
            {agent.name}
          </Text>
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 20 }}
        message={<Text strong>这个智能体负责做什么</Text>}
        description={
          <div>
            <Paragraph style={{ marginBottom: 8 }}>{profile.summary}</Paragraph>
            <Space size={[6, 6]} wrap style={{ marginBottom: 8 }}>
              {profile.strengths.map((item) => (
                <Tag key={item} color="blue" bordered={false}>
                  {item}
                </Tag>
              ))}
            </Space>
            <Paragraph style={{ marginBottom: 4 }}>
              <Text type="secondary">模型建议:</Text> {profile.modelHint}
            </Paragraph>
            <Paragraph style={{ marginBottom: 0 }}>
              <Text type="secondary">协议建议:</Text> {profile.protocolHint}
            </Paragraph>
          </div>
        }
      />

      <Form
        layout="vertical"
        requiredMark={false}
        disabled={saving}
        onFinish={handleSave}
      >
        <Row gutter={16}>
          <Col xs={24} md={12}>
            <Form.Item
              label="接口协议"
              tooltip="Responses API 适合新版 OpenAI；Chat Completions 兼容性最广。"
            >
              <Select<AgentProtocol>
                value={protocol}
                onChange={setProtocol}
                options={[
                  { value: 'chat', label: 'Chat Completions' },
                  { value: 'responses', label: 'Responses API' },
                ]}
              />
            </Form.Item>
          </Col>

          <Col xs={24} md={12}>
            <Form.Item label="Base URL" required>
              <Input
                type="url"
                placeholder="https://api.openai.com"
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
              />
            </Form.Item>
          </Col>

          <Col xs={24}>
            <Form.Item
              label={
                <Space size={4}>
                  <KeyOutlined /> API Key
                </Space>
              }
            >
              <Input.Password
                placeholder={
                  maskedKey ? `已保存: ${maskedKey},留空表示不修改` : '请输入 API Key'
                }
                value={apiKey}
                onChange={(event) => setApiKey(event.target.value)}
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>

          <Col xs={24}>
            <Form.Item label="模型">
              <Space.Compact style={{ width: '100%' }}>
                <Select
                  style={{ flex: 1 }}
                  value={model || undefined}
                  onChange={setModel}
                  placeholder='点击"获取模型"加载选项'
                  options={models.map((item) => ({ value: item, label: item }))}
                  notFoundContent="暂无模型,请先点击右侧按钮获取"
                />
                <Button
                  icon={<ReloadOutlined spin={loadingModels} />}
                  onClick={handleFetchModels}
                  loading={loadingModels}
                >
                  获取模型
                </Button>
              </Space.Compact>
            </Form.Item>
          </Col>
        </Row>

        <Form.Item style={{ marginBottom: 0 }}>
          <Button type="primary" htmlType="submit" loading={saving}>
            保存配置
          </Button>
        </Form.Item>

        {(message || error) && (
          <Alert
            style={{ marginTop: 16 }}
            showIcon
            type={error ? 'error' : 'success'}
            message={error ? '出现问题' : '已保存'}
            description={error ?? message}
            closable
            onClose={() => {
              setError(null)
              setMessage(null)
            }}
          />
        )}
      </Form>
    </ProCard>
  )
}
