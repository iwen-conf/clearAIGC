import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  acceptCard as apiAcceptCard,
  applyAllCards as apiApplyAllCards,
  createSession,
  deleteSession,
  getSessionState,
  pauseRound,
  rejectCard as apiRejectCard,
  resumeRound,
  startRound,
} from '@/api'
import type {
  CompletePayload,
  ErrorPayload,
  OutputResponse,
  PausedPayload,
  ProgressPayload,
  PromptProfile,
  QualityAlertPayload,
  RecoveryPayload,
  RoundDiffResponse,
  Session,
  SessionStateResponse,
  TimelineEntry,
} from '@/types'
import { toErrorMessage } from '@/lib/format'

const STORAGE_KEY = 'naturalize.active-document'

export interface UseSessionResult {
  session: Session | null
  preview: OutputResponse | null
  comparison: RoundDiffResponse | null
  progress: ProgressPayload | null
  timeline: TimelineEntry[]
  booting: boolean
  busy: boolean
  message: string | null
  error: string | null
  completedPassCount: number
  totalPasses: number
  latestCompletedRoundNumber: number | null
  cardBusyId: string | null
  applyingAll: boolean
  start: (file: File, promptProfile: PromptProfile) => Promise<void>
  resume: () => Promise<void>
  startNextRound: () => Promise<void>
  pause: () => Promise<void>
  refresh: () => Promise<void>
  reset: () => Promise<void>
  acceptCard: (cardId: string) => Promise<void>
  rejectCard: (cardId: string) => Promise<void>
  applyAll: () => Promise<void>
  adopt: (sessionId: string) => Promise<void>
  clearNotice: () => void
}

export function useSession(): UseSessionResult {
  const [session, setSession] = useState<Session | null>(null)
  const [preview, setPreview] = useState<OutputResponse | null>(null)
  const [comparison, setComparison] = useState<RoundDiffResponse | null>(null)
  const [progress, setProgress] = useState<ProgressPayload | null>(null)
  const [timeline, setTimeline] = useState<TimelineEntry[]>([])
  const [booting, setBooting] = useState(true)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [cardBusyId, setCardBusyId] = useState<string | null>(null)
  const [applyingAll, setApplyingAll] = useState(false)

  const activeSessionId = session?.id ?? null

  const completedRounds = useMemo(
    () => session?.rounds.filter((round) => round.status === 'completed') ?? [],
    [session],
  )
  const completedPassCount = completedRounds.length
  const totalPasses = session
    ? session.promptProfile === 'cn'
      ? 2
      : 1
    : 0
  const latestCompletedRoundNumber = completedRounds.at(-1)?.number ?? null

  const pushTimeline = useCallback(
    (entry: Omit<TimelineEntry, 'id' | 'timestamp'>) => {
      setTimeline((current) =>
        [
          {
            ...entry,
            id: crypto.randomUUID(),
            timestamp: Date.now(),
          },
          ...current,
        ].slice(0, 20),
      )
    },
    [],
  )

  const applySessionState = useCallback((state: SessionStateResponse) => {
    setSession(state.session)
    setPreview(state.preview)
    setComparison(state.comparison)
    setProgress(state.progress)
    setTimeline(state.timeline ?? [])
    setError(null)
  }, [])

  const refreshSession = useCallback(
    async (sessionId: string) => {
      const next = await getSessionState(sessionId)
      applySessionState(next)
    },
    [applySessionState],
  )

  const refreshRef = useRef(refreshSession)
  refreshRef.current = refreshSession

  useEffect(() => {
    const saved = window.localStorage.getItem(STORAGE_KEY)
    if (!saved) {
      setBooting(false)
      return
    }
    refreshRef
      .current(saved)
      .catch(() => {
        window.localStorage.removeItem(STORAGE_KEY)
      })
      .finally(() => setBooting(false))
  }, [])

  useEffect(() => {
    if (!session) return
    window.localStorage.setItem(STORAGE_KEY, session.id)
  }, [session])

  useEffect(() => {
    if (!activeSessionId) return

    const source = new EventSource(`/api/v1/sessions/${activeSessionId}/stream`)

    const onProgress = (event: MessageEvent<string>) => {
      const payload = JSON.parse(event.data) as ProgressPayload
      setProgress(payload)
      pushTimeline({
        tone: 'neutral',
        title: `第 ${payload.round} 轮 · ${payload.phase}`,
        detail: `${payload.completedChunks}/${payload.totalChunks} 片段已完成 · ${payload.providerUsed}`,
      })
    }

    const onQualityAlert = (event: MessageEvent<string>) => {
      const payload = JSON.parse(event.data) as QualityAlertPayload
      pushTimeline({
        tone: 'warning',
        title: '质量检查提示',
        detail: payload.reason,
      })
    }

    const onRecovery = (event: MessageEvent<string>) => {
      const payload = JSON.parse(event.data) as RecoveryPayload
      pushTimeline({
        tone: payload.success ? 'success' : 'warning',
        title: payload.success ? '片段已修复' : '修复未通过',
        detail: payload.success
          ? `经过 ${payload.steps} 次尝试通过 · 消耗 ${payload.tokenCost} tokens`
          : `片段 ${payload.chunkId} 无法自动确认`,
      })
    }

    const onComplete = (event: Event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as CompletePayload
      pushTimeline({
        tone: 'success',
        title: `第 ${payload.round} 轮完成`,
        detail: `${payload.passedChunks + payload.recoveredChunks}/${payload.chunkCount} 片段就绪 · 消耗 ${payload.totalTokens} tokens`,
      })
      void refreshRef.current(payload.sessionId)
    }

    const onPaused = (event: Event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as PausedPayload
      pushTimeline({
        tone: 'warning',
        title: '处理已暂停',
        detail:
          payload.reason === 'provider_unavailable'
            ? '模型服务暂时不可用。'
            : '您已手动暂停本次处理。',
      })
      void refreshRef.current(payload.sessionId)
    }

    const onError = (event: Event) => {
      try {
        const payload = JSON.parse((event as MessageEvent<string>).data) as ErrorPayload
        setError(payload.message)
        pushTimeline({
          tone: 'error',
          title: '处理出错',
          detail: payload.message,
        })
      } catch {
        // non-JSON error frames from the EventSource are heartbeats/disconnects
      }
      void refreshRef.current(activeSessionId)
    }

    source.addEventListener('progress', onProgress as EventListener)
    source.addEventListener('quality_alert', onQualityAlert as EventListener)
    source.addEventListener('recovery', onRecovery as EventListener)
    source.addEventListener('complete', onComplete)
    source.addEventListener('paused', onPaused)
    source.addEventListener('error', onError)

    return () => {
      source.close()
    }
  }, [activeSessionId, pushTimeline])

  const start = useCallback(
    async (file: File, promptProfile: PromptProfile) => {
      setBusy(true)
      setError(null)
      setMessage(null)
      try {
        const created = await createSession(file, promptProfile)
        await refreshRef.current(created.id)
        await startRound(created.id)
        await refreshRef.current(created.id)
      } catch (err) {
        setError(toErrorMessage(err))
      } finally {
        setBusy(false)
      }
    },
    [],
  )

  const resume = useCallback(async () => {
    if (!session) return
    setBusy(true)
    setError(null)
    try {
      await resumeRound(session.id)
      await refreshRef.current(session.id)
      setMessage('已继续本次润色。')
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }, [session])

  const startNextRound = useCallback(async () => {
    if (!session) return
    setBusy(true)
    setError(null)
    try {
      await startRound(session.id)
      await refreshRef.current(session.id)
      setMessage('下一轮处理已开始。')
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }, [session])

  const pause = useCallback(async () => {
    if (!session) return
    setBusy(true)
    setError(null)
    try {
      await pauseRound(session.id)
      setMessage('将在当前片段保存后暂停。')
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }, [session])

  const refresh = useCallback(async () => {
    if (!session) return
    setBusy(true)
    try {
      await refreshRef.current(session.id)
      setMessage('会话状态已刷新。')
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }, [session])

  const reset = useCallback(async () => {
    if (session) {
      try {
        await deleteSession(session.id)
      } catch {
        // ignore cleanup failures
      }
    }
    window.localStorage.removeItem(STORAGE_KEY)
    setSession(null)
    setPreview(null)
    setComparison(null)
    setProgress(null)
    setTimeline([])
    setMessage(null)
    setError(null)
  }, [session])

  const clearNotice = useCallback(() => {
    setMessage(null)
    setError(null)
  }, [])

  const adopt = useCallback(async (sessionId: string) => {
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      window.localStorage.setItem(STORAGE_KEY, sessionId)
      await refreshRef.current(sessionId)
      setMessage('已加载历史会话，可继续处理。')
    } catch (err) {
      window.localStorage.removeItem(STORAGE_KEY)
      setError(toErrorMessage(err))
      throw err
    } finally {
      setBusy(false)
    }
  }, [])

  const mutateCardState = useCallback(
    async (cardId: string, next: 'accepted' | 'rejected') => {
      if (!session || latestCompletedRoundNumber == null) return
      setCardBusyId(cardId)
      setError(null)
      try {
        if (next === 'accepted') {
          await apiAcceptCard(session.id, latestCompletedRoundNumber, cardId)
        } else {
          await apiRejectCard(session.id, latestCompletedRoundNumber, cardId)
        }
        setComparison((prev) =>
          prev
            ? {
                ...prev,
                chunks: prev.chunks.map((chunk) =>
                  chunk.id === cardId ? { ...chunk, state: next } : chunk,
                ),
              }
            : prev,
        )
      } catch (err) {
        setError(toErrorMessage(err))
      } finally {
        setCardBusyId(null)
      }
    },
    [session, latestCompletedRoundNumber],
  )

  const acceptCard = useCallback((cardId: string) => mutateCardState(cardId, 'accepted'), [mutateCardState])
  const rejectCard = useCallback((cardId: string) => mutateCardState(cardId, 'rejected'), [mutateCardState])

  const applyAll = useCallback(async () => {
    if (!session || latestCompletedRoundNumber == null) return
    setApplyingAll(true)
    setError(null)
    try {
      const { updated } = await apiApplyAllCards(session.id, latestCompletedRoundNumber)
      setComparison((prev) =>
        prev
          ? {
              ...prev,
              chunks: prev.chunks.map((chunk) =>
                chunk.state === 'pending' ? { ...chunk, state: 'accepted' } : chunk,
              ),
            }
          : prev,
      )
      setMessage(updated > 0 ? `已接受 ${updated} 张待审核卡片。` : '没有需要应用的待审核卡片。')
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setApplyingAll(false)
    }
  }, [session, latestCompletedRoundNumber])

  return {
    session,
    preview,
    comparison,
    progress,
    timeline,
    booting,
    busy,
    message,
    error,
    completedPassCount,
    totalPasses,
    latestCompletedRoundNumber,
    cardBusyId,
    applyingAll,
    start,
    resume,
    startNextRound,
    pause,
    refresh,
    reset,
    acceptCard,
    rejectCard,
    applyAll,
    adopt,
    clearNotice,
  }
}
