import type { EpisodeSummary } from '../types'
import { isFinished, isPaused, stageLabel } from '../lib/episode-progress'

export function RoundProgress({ episode, watchLink = false }: { episode: EpisodeSummary; watchLink?: boolean }) {
  const complete = episode.status === 'complete'
  const step = complete ? 4 : ({ queued: -1, describing: 0, awaiting_review: 0, awaiting_clarity_choice: 0, reacting: 1, debating: 2, writing: 3, complete: 4 })[episode.stage] ?? -1
  const stopped = isPaused(episode) || episode.status === 'abandoned'
  const needsReview = ['awaiting_review', 'awaiting_clarity_choice'].includes(episode.stage)
  return <section className="progress-card surface-card" aria-label="Round progress">
    <div className="progress-heading"><div><span className="eyebrow">{complete ? 'Round complete' : stopped ? 'Saved round progress' : 'Current round stage'}</span><h2>{stageLabel(episode)}</h2></div>{!stopped && !complete && <span className="live-dot" aria-label="Updating automatically" />}</div>
    <ol className="progress-steps">{['Shared evidence', 'First reactions', 'Rebuttals', 'Chronicle'].map((label, i) => <li className={i < step ? 'complete' : i === step ? 'current' : ''} aria-current={i === step ? 'step' : undefined} key={label}><span className="step-mark">{i < step ? '✓' : i + 1}</span><span>{label}</span></li>)}</ol>
    <p className="text-muted">{complete ? 'The historian’s chronicle is saved below.' : stopped ? 'This is the stage reached before the round stopped.' : needsReview ? 'The agents are waiting for your decision below.' : 'You can leave and return through this civilization’s Rounds tab. The current stage loads automatically.'}</p>
    {watchLink && !isFinished(episode) && !needsReview && <a className="button button-secondary" href="#agent-debate">Watch agents</a>}
  </section>
}
