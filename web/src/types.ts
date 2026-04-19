export type PromptProfile = 'cn' | 'cn_single' | 'en'
export type SessionStatus = 'pending' | 'processing' | 'paused' | 'completed' | 'failed'
export type RoundStatus = SessionStatus

export interface SessionSummary {
  id: string
  docId: string
  status: SessionStatus
  promptProfile: PromptProfile
}

export interface Round {
  id: string
  sessionId: string
  number: number
  prompt: string
  promptProfile: PromptProfile
  inputPath: string
  outputPath: string
  scoreTotal: number | null
  chunkLimit: number
  inputSegmentCount: number
  outputSegmentCount: number
  checkpointId: string
  providerUsed: string
  totalTokens: number
  status: RoundStatus
  startedAt: string | null
  completedAt: string | null
  createdAt: string
  recoveryJustification?: string
}

export interface Session {
  id: string
  documentId: string
  documentName: string
  docId: string
  originPath: string
  fileFormat: 'txt' | 'docx'
  fileSizeBytes: number
  promptProfile: PromptProfile
  status: SessionStatus
  createdAt: string
  updatedAt: string
  rounds: Round[]
}

export interface OutputResponse {
  text: string
  segmentCount?: number
  paragraphCount?: number
}

export interface CheckResult {
  type: string
  passed: boolean
  reason?: string
}

export type ChunkStatus = 'passed' | 'recovered' | 'failed'
export type ChunkReviewState = 'pending' | 'accepted' | 'rejected'

export interface ChunkComparison {
  id: string
  paragraphIndex: number
  chunkIndex: number
  input: string
  output: string
  status: ChunkStatus
  charDelta: number
  aiRate: number
  outputAiRate: number
  detector: string
  state: ChunkReviewState
  checks: CheckResult[]
}

export interface RoundDiffResponse {
  round: number
  chunks: ChunkComparison[]
}

export interface SessionStateResponse {
  session: Session
  preview: OutputResponse | null
  comparison: RoundDiffResponse | null
  progress: ProgressPayload | null
  timeline: TimelineEntry[]
}

export interface ProgressPayload {
  sessionId: string
  round: number
  phase: string
  completedChunks: number
  totalChunks: number
  percent: number
  chunkId: string
  paragraphIndex: number
  chunkIndex: number
  providerUsed: string
}

export interface QualityAlertPayload {
  chunkId: string
  checkType: string
  reason: string
  action: string
  recoveryStep: number
}

export interface RecoveryPayload {
  chunkId: string
  success: boolean
  method: string
  steps: number
  tokenCost: number
}

export interface CompletePayload {
  sessionId: string
  round: number
  scoreTotal: number
  chunkCount: number
  passedChunks: number
  recoveredChunks: number
  failedChunks: number
  totalTokens: number
  providerUsed: string
  downloadUrl: string
}

export interface PausedPayload {
  sessionId: string
  round: number
  completedChunks: number
  totalChunks: number
  checkpointId: string
  reason: string
}

export interface ErrorPayload {
  message: string
  recoverable: boolean
  code: string
  chunkId?: string
}

export interface TimelineEntry {
  id: string
  round?: number
  tone: 'neutral' | 'warning' | 'success' | 'error'
  title: string
  detail: string
  timestamp: number
}

export interface SessionListMetrics {
  completedRounds: number
  totalRounds: number
  totalTokens: number
  lastActivityAt: string
}

export interface SessionListItem {
  session: Session
  progress: ProgressPayload | null
  metrics: SessionListMetrics
}

export interface SessionListResponse {
  items: SessionListItem[]
  total: number
  page: number
  size: number
}

export interface RoundSummary {
  chunkCount: number
  passedChunks: number
  recoveredChunks: number
  failedChunks: number
  scoreTotal: number | null
  durationSeconds: number
}

export interface RoundHistoryEntry {
  round: Round
  summary: RoundSummary
}

export interface SessionHistoryResponse {
  session: Session
  progress: ProgressPayload | null
  metrics: SessionListMetrics
  rounds: RoundHistoryEntry[]
  timeline: TimelineEntry[]
}

export type AgentName = 'coordinator' | 'lexical_mutator' | 'syntax_rebuilder'
export type AgentProtocol = 'responses' | 'chat'

export interface AgentSetting {
  name: AgentName
  displayName: string
  protocol: AgentProtocol
  baseUrl: string
  apiKey?: string
  model: string
  updatedAt: string
}

export interface AgentSettingInput {
  protocol: AgentProtocol
  baseUrl: string
  apiKey: string
  model: string
}
