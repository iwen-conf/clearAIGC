import type { SessionStatus } from '@/types'

export function humanStatus(status: SessionStatus | undefined): string {
  switch (status) {
    case 'pending':
      return '待开始下一轮'
    case 'processing':
      return '润色进行中'
    case 'paused':
      return '已暂停'
    case 'completed':
      return '已完成'
    case 'failed':
      return '需要处理'
    default:
      return '就绪'
  }
}

export type StatusTone = 'default' | 'processing' | 'success' | 'warning' | 'error'

export function statusTone(status: SessionStatus | undefined): StatusTone {
  switch (status) {
    case 'processing':
      return 'processing'
    case 'paused':
      return 'warning'
    case 'completed':
      return 'success'
    case 'failed':
      return 'error'
    default:
      return 'default'
  }
}

export function statusDescription(
  status: SessionStatus | undefined,
  completedPasses: number,
  totalPasses: number,
): string {
  switch (status) {
    case 'pending':
      return completedPasses === 0
        ? '文档已上传，正在等待第一轮润色。'
        : `已完成第 ${completedPasses} 轮。${
            completedPasses < totalPasses ? '可以开始下一轮了。' : '文档已全部完成。'
          }`
    case 'processing':
      return '正在逐段润色文档。'
    case 'paused':
      return '处理已暂停，随时可以继续。'
    case 'completed':
      return '全部轮次已完成,最新稿可下载。'
    case 'failed':
      return '处理中止,请查看活动记录后重试。'
    default:
      return '上传文档以开始。'
  }
}

export function formatCharDelta(charDelta: number): string {
  if (charDelta > 0) return `字数变化 +${charDelta}`
  if (charDelta < 0) return `字数变化 ${charDelta}`
  return '字数基本未变'
}

export function formatDate(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

export function formatClock(value: number): string {
  return new Intl.DateTimeFormat('zh-CN', {
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

export function toErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  return '请求未能完成,请稍后再试。'
}

export function formatBytes(bytes: number): string {
  if (!bytes) return '—'
  const kb = bytes / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  return `${(kb / 1024).toFixed(2)} MB`
}
