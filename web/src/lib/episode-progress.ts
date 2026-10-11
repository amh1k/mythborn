import type { DebatePreview, EpisodeSummary } from '../types'

export const isFinished = (episode: EpisodeSummary) => ['complete', 'abandoned'].includes(episode.status)
export const isPaused = (episode: EpisodeSummary) => ['failed', 'needs_attention'].includes(episode.status)

export function stageLabel(episode: EpisodeSummary) {
  const labels = { queued: 'Waiting to start', describing: 'Describing the photo', awaiting_review: 'Waiting for your review', awaiting_clarity_choice: 'Waiting for your choice', reacting: 'First reactions', debating: 'Rebuttal round', writing: 'Historian is writing', complete: 'Chronicle complete' }
  if (episode.status === 'abandoned') return 'Discovery set aside'
  const label = labels[episode.stage] ?? 'Discovery in progress'
  return isPaused(episode) ? `Paused · ${label}` : label
}

export function roundLabel(episode: EpisodeSummary) {
  const name = { discovery: 'Discovery', council: 'Council', closing: 'Closing chronicle' }[episode.kind] ?? 'Discovery'
  return episode.sequence_number ? `Round ${episode.sequence_number} · ${name}` : name
}

export function stageDescription(episode: EpisodeSummary) {
  if (episode.status === 'abandoned') return 'This discovery was set aside. No beliefs or traditions changed.'
  if (isPaused(episode) && episode.error_code === 'model_quota_exhausted') return 'The provider quota is still exhausted after automatic retries. This round is paused with completed responses kept. Retry later when quota is available.'
  if (isPaused(episode)) return 'Processing paused at this stage. Use Retry to continue the round.'
  if (episode.status === 'complete') return 'The historian’s chronicle is saved below.'
  const descriptions = {
    queued: 'This round is waiting to start. The page checks for updates automatically.',
    describing: 'Preparing the shared photo description before the agents begin.',
    awaiting_review: 'The agents are waiting for you to accept or correct the photo description.',
    awaiting_clarity_choice: 'Choose whether to continue with this uncertain description or try another photo.',
    reacting: 'The agents are forming their first interpretations of the shared evidence. Responses appear below as they arrive.',
    debating: 'The agents are responding to the first reactions. Each agent prepares one rebuttal; received responses appear below.',
    writing: 'The Historian is writing the chronicle. The debate is complete; the saved record will appear when it is ready.',
    complete: 'The historian’s chronicle is saved below.',
  }
  return descriptions[episode.stage] ?? 'The current stage will update automatically.'
}

export function debateStage(episode: EpisodeSummary, preview?: DebatePreview | null): EpisodeSummary['stage'] {
  if (!preview || preview.round_id !== episode.id || !['describing', 'reacting', 'debating', 'writing'].includes(episode.stage) || isFinished(episode) || isPaused(episode)) return episode.stage
  return ({ describing: 'describing', reaction: 'reacting', rebuttal: 'debating', writing: 'writing' } as const)[preview.phase]
}

export function rateLimitMessage(retryAt: string, now: number) {
  const seconds = Math.ceil((Date.parse(retryAt) - now) / 1000)
  if (seconds <= 0) return 'Rate limit reached. Retrying now…'
  const minutes = Math.floor(seconds / 60)
  const remaining = seconds % 60
  const wait = minutes ? `${minutes} ${minutes === 1 ? 'minute' : 'minutes'}${remaining ? ` ${remaining} ${remaining === 1 ? 'second' : 'seconds'}` : ''}` : `${seconds} ${seconds === 1 ? 'second' : 'seconds'}`
  return `Rate limit reached. Retrying in ${wait}…`
}
