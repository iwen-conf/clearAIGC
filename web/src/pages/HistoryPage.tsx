import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Alert,
  Button,
  Empty,
  Input,
  Modal,
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PageContainer } from '@ant-design/pro-components'
import { DeleteOutlined, EyeOutlined, PlayCircleOutlined } from '@ant-design/icons'
import { deleteSession } from '@/api'
import { useHistoryList, type HistoryListParams } from '@/hooks/use-history-list'
import { useDebouncedValue } from '@/hooks/use-debounced-value'
import { useSession } from '@/hooks/use-session'
import type { SessionListItem, SessionStatus } from '@/types'
import {
  formatBytes,
  formatRelativeTime,
  promptProfileLabel,
  statusTone,
  toErrorMessage,
} from '@/lib/format'
import { StatusPill } from '@/components/workspace/status-pill'

const { Text } = Typography

const RESUMABLE_STATUSES: SessionStatus[] = ['pending', 'paused', 'processing', 'failed']

const STATUS_FILTER_OPTIONS: { label: string; value: HistoryListParams['status'] }[] = [
  { label: '全部状态', value: '' },
  { label: '处理中', value: 'processing' },
  { label: '已暂停', value: 'paused' },
  { label: '待开始', value: 'pending' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' },
]

const SORT_OPTIONS: { label: string; value: NonNullable<HistoryListParams['sort']> }[] = [
  { label: '最新上传', value: '-created_at' },
  { label: '最早上传', value: 'created_at' },
  { label: '最近活动', value: '-updated_at' },
  { label: '最早活动', value: 'updated_at' },
]

export default function HistoryPage() {
  const navigate = useNavigate()
  const sessionCtx = useSession()
  const [messageApi, messageContext] = message.useMessage()
  const [modalApi, modalContext] = Modal.useModal()

  const [searchInput, setSearchInput] = useState('')
  const debouncedSearch = useDebouncedValue(searchInput, 300)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [batchDeleting, setBatchDeleting] = useState(false)

  const { data, loading, error, params, setParams, refetch } = useHistoryList()

  useEffect(() => {
    if ((params.q ?? '') === debouncedSearch) return
    setParams({ q: debouncedSearch, page: 1 })
  }, [debouncedSearch, params.q, setParams])

  const handleResume = useCallback(
    async (sessionId: string) => {
      try {
        await sessionCtx.adopt(sessionId)
        messageApi.success('已加载历史会话。')
        navigate('/')
      } catch (err) {
        messageApi.error(toErrorMessage(err))
      }
    },
    [messageApi, navigate, sessionCtx],
  )

  const handleDelete = useCallback(
    (item: SessionListItem) => {
      modalApi.confirm({
        title: '确定要删除这份文档吗?',
        content: `《${item.session.documentName}》的处理记录与生成文件将被清除,操作不可撤销。`,
        okText: '删除',
        okButtonProps: { danger: true },
        cancelText: '取消',
        async onOk() {
          setDeletingId(item.session.id)
          try {
            await deleteSession(item.session.id)
            messageApi.success('已删除。')
            setSelectedIds((prev) => prev.filter((id) => id !== item.session.id))
            await refetch()
          } catch (err) {
            messageApi.error(toErrorMessage(err))
            throw err
          } finally {
            setDeletingId(null)
          }
        },
      })
    },
    [messageApi, modalApi, refetch],
  )

  const handleBatchDelete = useCallback(() => {
    if (selectedIds.length === 0) return
    modalApi.confirm({
      title: `确定要删除选中的 ${selectedIds.length} 份文档吗?`,
      content: '所选文档的处理记录与生成文件将被清除,操作不可撤销。',
      okText: '全部删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      async onOk() {
        setBatchDeleting(true)
        try {
          const results = await Promise.allSettled(selectedIds.map((id) => deleteSession(id)))
          const failed = results.filter((r) => r.status === 'rejected').length
          const succeeded = results.length - failed
          if (succeeded > 0) {
            messageApi.success(`已删除 ${succeeded} 份文档。`)
          }
          if (failed > 0) {
            messageApi.error(`${failed} 份文档删除失败,请重试。`)
          }
          const failedIds = selectedIds.filter((_, idx) => results[idx].status === 'rejected')
          setSelectedIds(failedIds)
          await refetch()
        } finally {
          setBatchDeleting(false)
        }
      },
    })
  }, [messageApi, modalApi, refetch, selectedIds])

  const columns: ColumnsType<SessionListItem> = [
      {
        title: '文档',
        key: 'document',
        render: (_, item) => (
          <Space direction="vertical" size={2}>
            <Text strong>{item.session.documentName || '未命名'}</Text>
            <Space size={6} wrap>
              <Tag color="geekblue">{(item.session.fileFormat || 'txt').toUpperCase()}</Tag>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {formatBytes(item.session.fileSizeBytes)}
              </Text>
            </Space>
          </Space>
        ),
      },
      {
        title: '模式',
        dataIndex: ['session', 'promptProfile'],
        key: 'profile',
        width: 110,
        responsive: ['md'],
        render: (value: string) => promptProfileLabel(value),
      },
      {
        title: '状态',
        key: 'status',
        width: 140,
        render: (_, item) => <StatusPill status={item.session.status} />,
      },
      {
        title: '轮次',
        key: 'rounds',
        width: 100,
        align: 'center',
        responsive: ['md'],
        render: (_, item) => (
          <Text>
            {item.metrics.completedRounds}
            <Text type="secondary"> / {item.metrics.totalRounds || '—'}</Text>
          </Text>
        ),
      },
      {
        title: '进度',
        key: 'progress',
        width: 180,
        responsive: ['lg'],
        render: (_, item) => {
          const percent = item.progress?.percent
          if (percent === undefined || percent === null) {
            return <Text type="secondary">—</Text>
          }
          return (
            <Progress
              percent={Math.round(percent)}
              size="small"
              status={statusTone(item.session.status) === 'error' ? 'exception' : undefined}
            />
          )
        },
      },
      {
        title: '最近活动',
        key: 'lastActivity',
        width: 150,
        render: (_, item) => (
          <Tooltip title={item.metrics.lastActivityAt}>
            <Text type="secondary">{formatRelativeTime(item.metrics.lastActivityAt)}</Text>
          </Tooltip>
        ),
      },
      {
        title: '操作',
        key: 'actions',
        width: 220,
        fixed: 'right',
        render: (_, item) => {
          const canResume = RESUMABLE_STATUSES.includes(item.session.status)
          return (
            <Space size={4} wrap>
              <Button
                type="link"
                size="small"
                icon={<EyeOutlined />}
                onClick={() => navigate(`/history/${item.session.id}`)}
              >
                查看详情
              </Button>
              {canResume && (
                <Button
                  type="link"
                  size="small"
                  icon={<PlayCircleOutlined />}
                  onClick={() => handleResume(item.session.id)}
                >
                  继续处理
                </Button>
              )}
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
                loading={deletingId === item.session.id}
                onClick={() => handleDelete(item)}
              >
                删除
              </Button>
            </Space>
          )
        },
      },
  ]

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const hasFilters = Boolean(params.q || params.status)

  return (
    <PageContainer
      title="历史记录"
      subTitle="查看所有上传过的文档与处理进度"
      extra={[
        <Button
          key="batch-delete"
          danger
          icon={<DeleteOutlined />}
          disabled={selectedIds.length === 0}
          loading={batchDeleting}
          onClick={handleBatchDelete}
        >
          批量删除{selectedIds.length > 0 ? ` (${selectedIds.length})` : ''}
        </Button>,
        <Button key="refresh" onClick={() => refetch()}>
          刷新
        </Button>,
      ]}
    >
      {messageContext}
      {modalContext}

      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Space wrap size={12}>
          <Input.Search
            placeholder="搜索文档名"
            allowClear
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            onSearch={(value) => setSearchInput(value)}
            style={{ width: 260 }}
          />
          <Select
            value={params.status ?? ''}
            onChange={(value) => setParams({ status: value, page: 1 })}
            options={STATUS_FILTER_OPTIONS.map((opt) => ({
              label: opt.label,
              value: opt.value ?? '',
            }))}
            style={{ width: 140 }}
          />
          <Select
            value={params.sort ?? '-created_at'}
            onChange={(value) => setParams({ sort: value, page: 1 })}
            options={SORT_OPTIONS}
            style={{ width: 150 }}
          />
        </Space>

        {error ? (
          <Alert
            type="error"
            showIcon
            message="无法加载历史记录"
            description={error}
            action={
              <Button size="small" onClick={() => refetch()}>
                重试
              </Button>
            }
          />
        ) : null}

        <Table<SessionListItem>
          rowKey={(item) => item.session.id}
          columns={columns}
          dataSource={items}
          loading={loading}
          scroll={{ x: 960 }}
          rowSelection={{
            selectedRowKeys: selectedIds,
            onChange: (keys) => setSelectedIds(keys.map(String)),
            preserveSelectedRowKeys: true,
          }}
          locale={{
            emptyText: hasFilters ? (
              <Empty description="没有匹配的记录" />
            ) : (
              <Empty description="还没有上传过文档">
                <Button type="primary" onClick={() => navigate('/')}>
                  去工作台上传
                </Button>
              </Empty>
            ),
          }}
          pagination={{
            current: params.page,
            pageSize: params.size,
            total,
            pageSizeOptions: ['10', '20', '50'],
            showSizeChanger: true,
            showTotal: (count) => `共 ${count} 条`,
            onChange: (page, size) => setParams({ page, size }),
          }}
        />
      </Space>
    </PageContainer>
  )
}
