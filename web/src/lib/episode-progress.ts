import type { EpisodeSummary } from '../types'

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
