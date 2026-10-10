import type { AdminAccount, Agent, AgentTemplate, ApiEnvelope, ApiErrorShape, Belief, Chronicle, ConversationSummary, DebateDetail, Episode, EpisodeSummary, Game, HistoryItem, InitialBeliefTemplate, Tradition } from '../types'

const baseUrl = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.replace(/\/$/, '') ?? ''

export class ApiError extends Error {
  constructor(message: string, public status: number, public code?: string, public requestId?: string) { super(message); this.name = 'ApiError' }
}

async function request<T>(path: string, token: string, init: RequestInit = {}, idempotencyKey?: string): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${token}`)
  if (idempotencyKey) headers.set('Idempotency-Key', idempotencyKey)
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const response = await fetch(`${baseUrl}/api/v1${path}`, { ...init, headers })
  const json = response.status === 204 ? undefined : await response.json().catch(() => undefined)
  if (!response.ok) {
    const error = json as ApiErrorShape | undefined
    const nested = typeof error?.error === 'object' ? error.error : undefined
    const message = nested?.message ?? error?.message ?? (typeof error?.error === 'string' ? error.error : undefined) ?? `The request failed (${response.status}).`
    throw new ApiError(message, response.status, nested?.code ?? error?.code, error?.request_id)
  }
  if (json && typeof json === 'object' && 'data' in json) return (json as ApiEnvelope<T>).data
  return json as T
}

const body = (value: unknown) => JSON.stringify(value)
export const api = {
  worlds: (token: string) => request<Game[] | { worlds: Game[] }>('/worlds', token),
  world: async (token: string, id: string) => { const result = await request<Game | { world: Game }>(`/worlds/${id}`, token); return 'world' in result ? result.world : result },
  createWorld: async (token: string, payload: { name: string; player_role: Game['player_role']; idempotency_key: string }) => (await request<{ world: Game }>('/worlds', token, { method: 'POST', body: body({ name: payload.name, player_role: payload.player_role }) }, payload.idempotency_key)).world,
  endWorld: (token: string, id: string) => request<{ command_id?: string }>(`/worlds/${id}/end`, token, { method: 'POST' }, crypto.randomUUID()),
  deleteWorld: (token: string, id: string) => request<void>(`/worlds/${id}`, token, { method: 'DELETE' }),
  deleteAccount: (token: string) => request<void>('/account', token, { method: 'DELETE' }, crypto.randomUUID()),
  settings: (token: string, id: string, review_photo_description: boolean) => request<Game | { world: Game }>(`/worlds/${id}/settings`, token, { method: 'PATCH', body: body({ review_photo_description }) }),
  agents: (token: string, id: string) => request<Agent[] | { agents: Agent[] }>(`/worlds/${id}/agents`, token),
  beliefs: (token: string, id: string, agentId: string) => request<Belief[] | { beliefs: Belief[] }>(`/worlds/${id}/agents/${agentId}/beliefs`, token),
  traditions: async (token: string, id: string) => {
    type TraditionRecord = { tradition: Tradition; supporter_ids: string[] }
    const result = await request<Tradition[] | { traditions: Array<Tradition | TraditionRecord> }>(`/worlds/${id}/traditions`, token)
    const records = Array.isArray(result) ? result : result.traditions
    return { traditions: records.map(record => 'tradition' in record ? { ...record.tradition, supporters: record.supporter_ids } : record) }
  },
  history: (token: string, id: string) => request<HistoryItem[] | { history: HistoryItem[] }>(`/worlds/${id}/history`, token),
  episodes: async (token: string, id: string, signal?: AbortSignal) => (await request<{ episodes: EpisodeSummary[] }>(`/worlds/${id}/episodes`, token, { signal, cache: 'no-store' })).episodes,
  episode: async (token: string, id: string, signal?: AbortSignal) => { const result = await request<{ episode: Episode; observation?: { id?: string; description?: string; description_status?: Episode['description_status']; photo_url?: string }; chronicle?: Episode['chronicle'] }>(`/episodes/${id}`, token, { signal, cache: 'no-store' }); return { ...result.episode, observation_id: result.observation?.id ?? result.episode.observation_id, description: result.observation?.description ?? result.episode.description, description_status: result.observation?.description_status ?? result.episode.description_status, photo_url: result.observation?.photo_url ?? result.episode.photo_url, chronicle: result.chronicle ?? result.episode.chronicle } },
  debate: (token: string, id: string, signal?: AbortSignal) => request<DebateDetail>(`/episodes/${id}/debate`, token, { signal, cache: 'no-store' }),
  photoUrl: async (token: string, observationId: string) => { const result = await request<{ url?: string; signed_url?: string; expires_at?: string }>(`/observations/${observationId}/photo-url`, token); return { url: result.url ?? result.signed_url ?? '', expires_at: result.expires_at } },
  submitObservation: async (token: string, gameId: string, form: FormData) => request<{ episode_id: string; command_id: string }>(`/worlds/${gameId}/observations`, token, { method: 'POST', body: form }, String(form.get('idempotency_key') ?? '')),
  descriptionDecision: (token: string, id: string, payload: { decision: 'accept' | 'correct' | 'choose_clarity'; player_correction?: string; clarity_choice?: 'continue_uncertain' | 'try_another_photo' }) => request<{ episode?: Episode; command_id?: string }>(`/episodes/${id}/description-decision`, token, { method: 'POST', body: body(payload) }, crypto.randomUUID()),
  retryEpisode: (token: string, id: string) => request<{ episode?: Episode; command_id?: string }>(`/episodes/${id}/retry`, token, { method: 'POST' }, crypto.randomUUID()),
  conversation: (token: string, episodeId: string, agentId: string) => request<{ summary: ConversationSummary | null; summary_version: number }>(`/episodes/${episodeId}/agents/${agentId}/messages`, token),
  sendMessage: (token: string, episodeId: string, agentId: string, message: string, expectedSummaryVersion = 0) => request<{ command_id: string }>(`/episodes/${episodeId}/agents/${agentId}/messages`, token, { method: 'POST', body: body({ message, expected_summary_version: expectedSummaryVersion }) }, crypto.randomUUID()),
  command: async (token: string, id: string) => { const result = await request<{ command: { status: string }; status?: string; reply?: unknown; summary?: ConversationSummary }>(`/commands/${id}`, token); return { status: result.status ?? result.command?.status ?? 'pending', reply: result.reply, summary: result.summary } },
  admin: (token: string, path: string, init: RequestInit = {}) => request<unknown>(`/admin${path}`, token, init),
  adminTemplates: async (token: string) => (await request<{ templates: AgentTemplate[] }>('/admin/templates', token)).templates,
  createAdminTemplate: async (token: string, input: Omit<AgentTemplate, 'id' | 'version' | 'beliefs'> & { beliefs: InitialBeliefTemplate[] }) => (await request<{ template: AgentTemplate }>('/admin/templates', token, { method: 'POST', body: body(input) })).template,
  activateAdminTemplate: (token: string, id: string) => request<void>(`/admin/templates/${id}/activate`, token, { method: 'POST' }),
  adminAccounts: async (token: string) => (await request<{ accounts: AdminAccount[] }>('/admin/accounts', token)).accounts,
  updateAdminAccountRole: (token: string, id: string, system_role: AdminAccount['system_role']) => request<{ system_role: AdminAccount['system_role'] }>(`/admin/accounts/${id}`, token, { method: 'PATCH', body: body({ system_role }) }),
  adminGames: async (token: string) => (await request<{ games: Game[] }>('/admin/games', token)).games,
}

export function asArray<T>(value: T[] | Record<string, T[]> | undefined, key: string): T[] {
  if (Array.isArray(value)) return value
  if (value && Array.isArray(value[key])) return value[key] as T[]
  return []
}

export function chronicleFrom(item: HistoryItem | Chronicle): Chronicle { return item }
