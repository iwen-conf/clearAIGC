import { useCallback, useEffect, useState } from 'react'
import { listAgents } from '@/api'
import type { AgentSetting } from '@/types'
import { toErrorMessage } from '@/lib/format'

export interface UseAgentsResult {
  agents: AgentSetting[]
  loading: boolean
  error: string | null
  replaceAgent: (next: AgentSetting) => void
  reload: () => Promise<void>
}

export function useAgents(): UseAgentsResult {
  const [agents, setAgents] = useState<AgentSetting[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const items = await listAgents()
      setAgents(items)
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    let cancelled = false

    listAgents()
      .then((items) => {
        if (!cancelled) {
          setAgents(items)
          setError(null)
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(toErrorMessage(err))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [])

  const replaceAgent = useCallback((next: AgentSetting) => {
    setAgents((current) =>
      current.map((item) => (item.name === next.name ? next : item)),
    )
  }, [])

  return { agents, loading, error, replaceAgent, reload: load }
}
