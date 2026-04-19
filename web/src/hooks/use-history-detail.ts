import { useCallback, useEffect, useRef, useState } from 'react'
import { getSessionHistory } from '@/api'
import type { SessionHistoryResponse } from '@/types'
import { toErrorMessage } from '@/lib/format'

export interface UseHistoryDetailResult {
  data: SessionHistoryResponse | null
  loading: boolean
  error: string | null
  reload: () => Promise<void>
}

export function useHistoryDetail(sessionId: string | undefined): UseHistoryDetailResult {
  const [data, setData] = useState<SessionHistoryResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const abortRef = useRef<AbortController | null>(null)

  const fetchHistory = useCallback(async (id: string) => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setLoading(true)
    setError(null)
    try {
      const response = await getSessionHistory(id)
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
    if (!sessionId) {
      void Promise.resolve().then(() => {
        if (!active) return
        setData(null)
        setError(null)
      })
      return () => {
        active = false
      }
    }
    void Promise.resolve().then(() => {
      if (active) void fetchHistory(sessionId)
    })
    return () => {
      active = false
      abortRef.current?.abort()
    }
  }, [fetchHistory, sessionId])

  const reload = useCallback(async () => {
    if (!sessionId) return
    await fetchHistory(sessionId)
  }, [fetchHistory, sessionId])

  return { data, loading, error, reload }
}
