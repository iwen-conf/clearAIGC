import { useCallback, useEffect, useRef, useState } from 'react'
import { listSessions, type ListSessionsParams } from '@/api'
import type { SessionListResponse, SessionStatus } from '@/types'
import { toErrorMessage } from '@/lib/format'

export interface HistoryListParams {
  page: number
  size: number
  status?: SessionStatus | ''
  q?: string
  sort?: ListSessionsParams['sort']
}

export interface UseHistoryListResult {
  data: SessionListResponse | null
  loading: boolean
  error: string | null
  params: HistoryListParams
  setParams: (patch: Partial<HistoryListParams>) => void
  refetch: () => Promise<void>
}

const DEFAULT_PARAMS: HistoryListParams = {
  page: 1,
  size: 20,
  status: '',
  q: '',
  sort: '-created_at',
}

export function useHistoryList(initial?: Partial<HistoryListParams>): UseHistoryListResult {
  const [params, setParamsState] = useState<HistoryListParams>({ ...DEFAULT_PARAMS, ...initial })
  const [data, setData] = useState<SessionListResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const abortRef = useRef<AbortController | null>(null)
  const paramsRef = useRef(params)

  useEffect(() => {
    paramsRef.current = params
  }, [params])

  const fetchData = useCallback(async (current: HistoryListParams) => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setLoading(true)
    setError(null)
    try {
      const response = await listSessions({
        page: current.page,
        size: current.size,
        status: current.status || undefined,
        q: current.q ? current.q : undefined,
        sort: current.sort,
      })
      if (!controller.signal.aborted) {
        setData(response)
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setError(toErrorMessage(err))
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    let active = true
    void Promise.resolve().then(() => {
      if (active) void fetchData(params)
    })
    return () => {
      active = false
      abortRef.current?.abort()
    }
  }, [fetchData, params])

  const setParams = useCallback((patch: Partial<HistoryListParams>) => {
    setParamsState((previous) => ({ ...previous, ...patch }))
  }, [])

  const refetch = useCallback(() => fetchData(paramsRef.current), [fetchData])

  return { data, loading, error, params, setParams, refetch }
}
