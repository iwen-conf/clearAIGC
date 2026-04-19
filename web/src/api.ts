import type {
  AgentName,
  AgentSetting,
  AgentSettingInput,
  OutputResponse,
  PromptProfile,
  Round,
  RoundDiffResponse,
  Session,
  SessionHistoryResponse,
  SessionListResponse,
  SessionStateResponse,
  SessionStatus,
  SessionSummary,
} from './types'

export interface ListSessionsParams {
  page?: number
  size?: number
  status?: SessionStatus | ''
  q?: string
  sort?: '-created_at' | 'created_at' | '-updated_at' | 'updated_at'
}

type ErrorBody = {
  error?: {
    code?: string
    message?: string
  }
}

async function request<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init)
  if (!response.ok) {
    let message = 'The request could not be completed.'
    try {
      const payload = (await response.json()) as ErrorBody
      message = payload.error?.message ?? message
    } catch {
      // ignore parse failures
    }
    throw new Error(message)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export async function createSession(file: File, promptProfile: PromptProfile): Promise<SessionSummary> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('promptProfile', promptProfile)

  return request<SessionSummary>('/api/v1/sessions', {
    method: 'POST',
    body: formData,
  })
}

export async function getSession(sessionId: string): Promise<Session> {
  return request<Session>(`/api/v1/sessions/${sessionId}`)
}

export async function listSessions(params: ListSessionsParams = {}): Promise<SessionListResponse> {
  const search = new URLSearchParams()
  if (params.page) search.set('page', String(params.page))
  if (params.size) search.set('size', String(params.size))
  if (params.status) search.set('status', params.status)
  if (params.q) search.set('q', params.q)
  if (params.sort) search.set('sort', params.sort)
  const qs = search.toString()
  return request<SessionListResponse>(`/api/v1/sessions${qs ? `?${qs}` : ''}`)
}

export async function getSessionHistory(sessionId: string): Promise<SessionHistoryResponse> {
  return request<SessionHistoryResponse>(`/api/v1/sessions/${sessionId}/history`)
}

export async function getSessionState(sessionId: string): Promise<SessionStateResponse> {
  return request<SessionStateResponse>(`/api/v1/sessions/${sessionId}/state`)
}

export async function startRound(sessionId: string): Promise<{ roundId: string; roundNumber: number; status: string }> {
  return request(`/api/v1/sessions/${sessionId}/start`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({}),
  })
}

export async function pauseRound(sessionId: string): Promise<{ status: string; checkpointId: string }> {
  return request(`/api/v1/sessions/${sessionId}/pause`, {
    method: 'POST',
  })
}

export async function resumeRound(sessionId: string): Promise<{ roundId: string; status: string }> {
  return request(`/api/v1/sessions/${sessionId}/resume`, {
    method: 'POST',
  })
}

export async function getRound(sessionId: string, roundNumber: number): Promise<Round> {
  return request<Round>(`/api/v1/sessions/${sessionId}/rounds/${roundNumber}`)
}

export async function getOutput(sessionId: string, roundNumber: number): Promise<OutputResponse> {
  return request<OutputResponse>(`/api/v1/sessions/${sessionId}/output?round=${roundNumber}`)
}

export async function getDiff(sessionId: string, roundNumber: number): Promise<RoundDiffResponse> {
  return request<RoundDiffResponse>(`/api/v1/sessions/${sessionId}/diff?round=${roundNumber}`)
}

export async function acceptCard(sessionId: string, roundNumber: number, cardId: string): Promise<void> {
  await request<unknown>(`/api/v1/sessions/${sessionId}/cards/${cardId}/accept?round=${roundNumber}`, {
    method: 'POST',
  })
}

export async function rejectCard(sessionId: string, roundNumber: number, cardId: string): Promise<void> {
  await request<unknown>(`/api/v1/sessions/${sessionId}/cards/${cardId}/reject?round=${roundNumber}`, {
    method: 'POST',
  })
}

export async function applyAllCards(sessionId: string, roundNumber: number): Promise<{ updated: number }> {
  const body = await request<{ updated: number }>(
    `/api/v1/sessions/${sessionId}/cards/apply-all?round=${roundNumber}`,
    { method: 'POST' },
  )
  return { updated: body.updated ?? 0 }
}

export function exportUrl(
  sessionId: string,
  roundNumber: number,
  format: 'txt' | 'docx',
  selection: 'all' | 'accepted' = 'all',
): string {
  return `/api/v1/sessions/${sessionId}/export?round=${roundNumber}&format=${format}&selection=${selection}`
}

export async function deleteSession(sessionId: string): Promise<void> {
  return request<void>(`/api/v1/sessions/${sessionId}`, {
    method: 'DELETE',
  })
}

export async function listAgents(): Promise<AgentSetting[]> {
  const body = await request<{ items: AgentSetting[] }>('/api/v1/agents')
  return body.items ?? []
}

export async function updateAgent(name: AgentName, payload: AgentSettingInput): Promise<AgentSetting> {
  return request<AgentSetting>(`/api/v1/agents/${name}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

export async function fetchAgentModels(name: AgentName, payload: AgentSettingInput): Promise<string[]> {
  const body = await request<{ models: string[] }>(`/api/v1/agents/${name}/models`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  return body.models ?? []
}
