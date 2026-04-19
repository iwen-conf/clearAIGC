import { Alert, Col, Row, Space, Spin, Tag, Typography } from 'antd'
import { PageContainer } from '@ant-design/pro-components'
import { AgentCard } from '@/components/settings/agent-card'
import { useAgents } from '@/hooks/use-agents'

const { Paragraph, Text } = Typography

export default function SettingsPage() {
  const { agents, loading, error, replaceAgent } = useAgents()

  return (
    <PageContainer
      title="为每个智能体配置独立的模型"
      subTitle={
        <Text type="secondary">
          本页介绍每个智能体的角色职责与模型建议,帮助你为不同岗位挑选合适的模型。
        </Text>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message={<Text strong>三个智能体如何协同</Text>}
          description={
            <div>
              <Paragraph style={{ marginBottom: 8 }}>
                每个片段会按 <Text strong>协调者 → 词汇改写 / 句法重塑 → 提交定稿</Text> 的顺序流转:
              </Paragraph>
              <Space size={[6, 6]} wrap>
                <Tag color="processing">coordinator 规划调度</Tag>
                <Tag color="success">lexical_mutator 稀释 AI 词汇</Tag>
                <Tag color="warning">syntax_rebuilder 重塑句法节奏</Tag>
              </Space>
              <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
                协调者必须支持 Function Calling;两个下游 Worker 只做局部改写,选模型时侧重指令遵循与 JSON 稳定性即可。
              </Paragraph>
            </div>
          }
        />

        {loading ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: 48 }}>
            <Spin tip="正在加载智能体…" />
          </div>
        ) : error ? (
          <Alert
            type="error"
            showIcon
            message="无法加载智能体配置"
            description={error}
          />
        ) : (
          <Row gutter={[16, 16]}>
            {agents.map((agent) => (
              <Col xs={24} key={agent.name}>
                <AgentCard agent={agent} onSaved={replaceAgent} />
              </Col>
            ))}
          </Row>
        )}
      </Space>
    </PageContainer>
  )
}
