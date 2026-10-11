import type { DebateProgress, EpisodeSummary } from '../types'
import { debateStage, isFinished, isPaused, rateLimitMessage, stageDescription, stageLabel } from '../lib/episode-progress'

export function RoundProgress({ episode, watchLink = false, debate }: { episode: EpisodeSummary; watchLink?: boolean; debate?: DebateProgress }) {
  const preview = debate?.preview?.round_id === episode.id ? debate.preview : null
  const stage = debateStage(episode, debate?.unavailable ? null : preview)
  const displayedEpisode = { ...episode, stage }
  const complete = episode.status === 'complete'
  const step = complete ? 4 : ({ queued: -1, describing: 0, awaiting_review: 0, awaiting_clarity_choice: 0, reacting: 1, debating: 2, writing: 3, complete: 4 })[stage] ?? -1
  const stopped = isPaused(episode) || episode.status === 'abandoned'
  const needsReview = ['awaiting_review', 'awaiting_clarity_choice'].includes(stage)
  const live = !stopped && !complete && !needsReview && stage !== 'queued'
  const nextRetry = live && !debate?.unavailable ? preview?.retries?.reduce<string | undefined>((earliest, retry) => !earliest || Date.parse(retry.retry_at) < Date.parse(earliest) ? retry.retry_at : earliest, undefined) : undefined
  const processing = live && !nextRetry && !debate?.unavailable
  const phase = stage === 'reacting' ? 'reaction' : stage === 'debating' ? 'rebuttal' : null
  const received = phase ? preview?.entries.filter(entry => entry.phase === phase).length : undefined
  const title = stage === 'debating' && live ? 'Rebuttals in progress' : stageLabel(displayedEpisode)
  return <section className="progress-card surface-card" aria-label="Round progress">
    <div className="progress-heading"><div><span className="eyebrow">{complete ? 'Round complete' : stopped ? 'Saved round progress' : nextRetry ? 'Waiting for automatic retry' : 'Current round stage'}</span><h2>{title}</h2></div>{processing && <span className="activity-indicator" aria-hidden="true" />}</div>
    <ol className="progress-steps">{['Shared evidence', 'First reactions', 'Rebuttals', 'Chronicle'].map((label, i) => <li className={i < step ? 'complete' : i === step ? 'current' : ''} aria-current={i === step ? 'step' : undefined} key={label}><span className="step-mark">{i < step ? '✓' : i + 1}</span><span>{label}</span></li>)}</ol>
    <p className="progress-explanation text-muted" role="status" aria-live="polite">{nextRetry ? `${rateLimitMessage(nextRetry, debate!.now)} Completed responses are kept; no reload or manual retry is needed.` : stageDescription(displayedEpisode)}{live && !debate?.unavailable && received !== undefined && ` ${received} of ${preview?.agents.length ?? 4} ${phase === 'reaction' ? 'reactions' : 'rebuttals'} received.`}{live && debate?.unavailable && ' Reconnecting to live agent progress; this is the latest saved stage.'}</p>
    {!complete && !stopped && <p className="text-small text-muted">Progress updates automatically. You can leave and return through this civilization’s Rounds tab.</p>}
    {watchLink && !isFinished(episode) && !needsReview && <a className="button button-secondary" href="#agent-debate">Watch agents</a>}
  </section>
}
