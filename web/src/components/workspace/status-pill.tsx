import { Badge } from 'antd'
import type { SessionStatus } from '@/types'
import { humanStatus, statusTone } from '@/lib/format'

interface StatusPillProps {
  status: SessionStatus | undefined
  'data-testid'?: string
}

export function StatusPill({ status, ...rest }: StatusPillProps) {
  return <Badge status={statusTone(status)} text={humanStatus(status)} {...rest} />
}
