export type PlayerRole = 'observer' | 'god' | 'messenger'
export type GameStatus = 'starting' | 'active' | 'ending_requested' | 'ending' | 'archived' | 'deleting'
export type AgentType = 'priest' | 'scientist' | 'soldier' | 'historian'
export type EpisodeStatus = 'queued' | 'running' | 'needs_attention' | 'failed' | 'complete' | 'abandoned'
export type EpisodeStage = 'queued' | 'describing' | 'awaiting_review' | 'awaiting_clarity_choice' | 'reacting' | 'debating' | 'writing' | 'complete'

export interface Game {
  id: string
  name: string
  player_role: PlayerRole
  status: GameStatus
  review_photo_description?: boolean
  created_at: string
  archived_at?: string | null
  latest_episode?: EpisodeSummary | null
}
export interface Agent {
  id: string
  agent_type: AgentType
  display_name?: string
  role_summary?: string
  beliefs?: Belief[]
}
export interface Belief {
  id: string
  claim: string
  state: 'forming' | 'held' | 'questioned' | 'abandoned'
  version?: number
  revisions?: BeliefRevision[]
}
export interface BeliefRevision {
  id: string
  change_type: string
  previous_claim?: string
  new_claim?: string
  reason?: string
  created_at?: string
}
export interface Tradition {
  id: string
  type: 'myth' | 'ritual' | 'taboo'
  title: string
  description?: string
  state: 'adopted' | 'contested' | 'retired'
  supporters?: string[]
}
export interface ConversationSummary {
  id: string
  summary: string
  version: number
  updated_at?: string
}
export interface InitialBeliefTemplate {
  id?: string
  claim: string
  initial_state: Belief['state']
  sort_order?: number
}
export interface AgentTemplate {
  id: string
  agent_type: AgentType
  version: number
  display_name: string
  personality_prompt: string
  is_active: boolean
  beliefs: InitialBeliefTemplate[]
}
export interface AdminAccount {
  id: string
  system_role: 'user' | 'admin'
  status: string
  created_at: string
}
export interface Chronicle {
  id: string
  round_id: string
  outcome: 'consensus' | 'majority' | 'unresolved'
  verdict: string
  body: string
  created_at: string
  suggestion?: string | null
  observation_id?: string
}
export interface EpisodeSummary {
  id: string
  game_id: string
  sequence_number: number
  kind: 'discovery' | 'council' | 'closing'
  status: EpisodeStatus
  stage: EpisodeStage
  created_at?: string
  chronicle?: Chronicle | null
  observation_id?: string
  error_code?: string
  error_message?: string
}
export interface Episode extends EpisodeSummary {
  description?: string
  description_status?: 'pending' | 'awaiting_review' | 'uncertain' | 'accepted'
  photo_url?: string
  chronicle?: Chronicle | null
  command_id?: string
}
export interface DebateAgent {
  agent_id: string
  agent_type: AgentType
  display_name: string
}
export interface DebateEntry {
  id: string
  agent_id: string
  phase: 'reaction' | 'rebuttal'
  text: string
  reasoning?: string
}
export interface DebatePreview {
  game_id: string
  round_id: string
  attempt_id: string
  phase: 'describing' | 'reaction' | 'rebuttal' | 'writing'
  agents: DebateAgent[]
  entries: DebateEntry[]
  retries?: { agent_id?: string; phase: 'describing' | 'reaction' | 'rebuttal' | 'writing'; retry_at: string; attempt: number }[]
}
export interface DebateDetail {
  state: 'waiting' | 'live' | 'unavailable' | 'finished'
  player_role: PlayerRole
  preview: DebatePreview | null
}
export interface HistoryItem extends Chronicle {
  round_kind?: 'discovery' | 'council' | 'closing'
}
export interface SessionUser { id: string; email?: string }

export interface ApiEnvelope<T> { data: T; request_id?: string }
export interface ApiErrorShape { error?: string | { code?: string; message?: string }; code?: string; request_id?: string; message?: string }
