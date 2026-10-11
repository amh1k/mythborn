import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useAuth } from '../auth'
import { api } from '../lib/api'
import { usePolling } from '../lib/use-polling'
import { debateStage, isFinished, isPaused, roundLabel, stageDescription, stageLabel } from '../lib/episode-progress'
import { LiveDebate } from '../components/live-debate'
import { RoundProgress } from '../components/round-progress'
import { Badge, Empty, ErrorNotice, Loading } from '../components/ui'
import type { DebateProgress, Episode } from '../types'
const formatDate = (value?: string) => value ? new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }) : 'Undated'

export function EpisodePage() {
  const [debateProgress, setDebateProgress] = useState<DebateProgress>()
  const { episodeId = '' } = useParams(); const token = useAuth().session?.access_token ?? ''; const [episode, setEpisode] = useState<Episode | null>(null); const [loading, setLoading] = useState(true); const [error, setError] = useState<unknown>(null); const [busy, setBusy] = useState(false); const [correction, setCorrection] = useState(''); const [photo, setPhoto] = useState(''); const [commandId, setCommandId] = useState(''); const [pollVersion, setPollVersion] = useState(0)
  const reload = () => api.episode(token, episodeId).then(next => { setEpisode(next); return next })
  useEffect(() => { setEpisode(null); setLoading(true); setPhoto(''); setCommandId(''); setDebateProgress(undefined) }, [episodeId])
  usePolling(`${episodeId}/${token}/${pollVersion}`, async signal => {
    try {
      const next = await api.episode(token, episodeId, signal)
      if (signal.aborted) return false
      setEpisode(next); setError(null)
      let pending = Boolean(commandId)
      if (commandId) {
        const command = await api.command(token, commandId)
        pending = !['processed', 'failed'].includes(command.status)
        if (!pending) setCommandId('')
        if (command.status === 'failed') setError(new Error('The command could not finish. Retry or set this round aside.'))
      }
      return pending || !(isFinished(next) || isPaused(next))
    } catch (e) { if (!signal.aborted) setError(e); throw e }
    finally { if (!signal.aborted) setLoading(false) }
  })
  useEffect(() => {
    if (!episode?.observation_id) return
    let alive = true
    let timer: number | undefined
    const refresh = async () => {
      try {
        const result = await api.photoUrl(token, episode.observation_id!)
        if (!alive) return
        setPhoto(result.url)
        const expires = result.expires_at ? Date.parse(result.expires_at) - Date.now() - 60000 : 240000
        timer = window.setTimeout(() => { if (!document.hidden) void refresh() }, Math.max(30000, expires))
      } catch { /* Evidence text and recovery controls remain usable. */ }
    }
    void refresh()
    const resume = () => { if (!document.hidden) { window.clearTimeout(timer); void refresh() } }
    document.addEventListener('visibilitychange', resume)
    return () => { alive = false; window.clearTimeout(timer); document.removeEventListener('visibilitychange', resume) }
  }, [episode?.observation_id, token])
  async function decision(payload: Parameters<typeof api.descriptionDecision>[2]) { setBusy(true); setError(null); try { const result = await api.descriptionDecision(token, episodeId, payload); if (result.command_id) setCommandId(result.command_id); await reload(); setPollVersion(v => v + 1) } catch (e) { setError(e) } finally { setBusy(false) } }
  async function retry() { setBusy(true); setError(null); try { const result = await api.retryEpisode(token, episodeId); if (result.command_id) setCommandId(result.command_id); await reload(); setPollVersion(v => v + 1) } catch (e) { setError(e) } finally { setBusy(false) } }
  async function abandon() {
    setBusy(true); setError(null)
    try { const result = await api.abandonEpisode(token, episodeId); if (result.command_id) setCommandId(result.command_id); setPollVersion(v => v + 1) }
    catch (e) { setError(e) } finally { setBusy(false) }
  }
  if (loading) return <Loading label="Reconnecting to this discovery…" />
  if (error && !episode) return <><Link className="back-link" to="/worlds">← Your worlds</Link><ErrorNotice error={error} /></>
  if (!episode) return <ErrorNotice error={new Error('This discovery could not be found.')} />
  const done = episode.status === 'complete'
  const displayedEpisode = { ...episode, stage: debateStage(episode, debateProgress?.unavailable ? null : debateProgress?.preview) }
  return <><Link className="back-link" to={episode.game_id ? `/worlds/${episode.game_id}` : "/worlds"}>← Back to civilization</Link><div className="episode-layout"><main className="episode-main"><div className="page-heading episode-heading"><div><p className="eyebrow">{roundLabel(episode)} · {formatDate(episode.created_at)}</p><h1>{done ? 'The chronicle' : 'A discovery is unfolding'}</h1></div><Badge state={done ? 'complete' : ['failed','needs_attention'].includes(episode.status) ? 'attention' : 'active'}>{stageLabel(displayedEpisode)}</Badge></div>
    <RoundProgress episode={episode} watchLink debate={debateProgress} />
    {photo && <img className="episode-photo" src={photo} alt="Your submitted observation" />}
    {episode.description && <section className="evidence-card surface-subtle"><span className="eyebrow">Shared photo description</span><p>{episode.description}</p>{episode.description_status === 'awaiting_review' && <span className="text-small text-muted">This description is waiting for your review.</span>}</section>}
    {episode.stage === 'awaiting_review' && <section className="decision-card surface-card"><h2>Does this match what you see?</h2><p className="text-muted">Correct visible details if needed. Meaning and interpretation are for you and your agents to explore.</p><label htmlFor="correction">Correct the description <span className="optional">(optional)</span></label><textarea id="correction" value={correction} maxLength={1800} onChange={e => setCorrection(e.target.value)} placeholder="For example: the object in the lower corner is a seed, not a stone." />{error !== null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-secondary" disabled={busy} onClick={() => void decision({ decision:'accept' })}>Accept description</button><button className="button" disabled={busy || !correction.trim()} onClick={() => void decision({ decision:'correct', player_correction: correction.trim() })}>Save correction</button></div></section>}
    {episode.stage === 'awaiting_clarity_choice' && <section className="decision-card surface-card"><h2>The photo is hard to read</h2><p className="text-muted">The description is uncertain. You can still continue, and agents will be asked not to treat uncertain details as fact.</p>{error !== null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-secondary" disabled={busy} onClick={() => void decision({ decision:'choose_clarity', clarity_choice:'try_another_photo' })}>Try another photo</button><button className="button" disabled={busy} onClick={() => void decision({ decision:'choose_clarity', clarity_choice:'continue_uncertain' })}>Continue with uncertainty</button></div></section>}
    {error !== null && episode && !['awaiting_review','awaiting_clarity_choice'].includes(episode.stage) && <ErrorNotice error={error} />}
    {commandId && <p className="text-small text-muted" role="status">Your request is accepted. Updating this round…</p>}
    {(['failed','needs_attention'].includes(episode.status)) && <section className="attention-card"><h2>This discovery needs another try</h2><p>{episode.error_message ?? stageDescription(episode)}</p>{error !== null && <ErrorNotice error={error} />}<button className="button" disabled={busy} onClick={() => void retry()}>{busy ? 'Working…' : 'Retry round'}</button></section>}
    {!isFinished(episode) && episode.kind !== 'closing' && <div className="round-control"><button className="button button-quiet" disabled={busy || Boolean(commandId)} onClick={() => void abandon()}>{busy ? 'Working…' : 'Set this round aside'}</button><span className="text-small text-muted">Stops this round without saving beliefs or traditions.</span></div>}
    <LiveDebate key={episode.id} episode={episode} token={token} onProgress={setDebateProgress} />
    {episode.chronicle && <ChronicleDetail chronicle={episode.chronicle} />}
    {done && <section className="saved-changes"><h2>What changed</h2>{episode.changes?.length ? episode.changes.map(change => <article key={change.id} className="saved-change"><p className="eyebrow">{change.kind} · {change.name}</p>{change.before && <p className="text-muted">Before ({change.before_state}): {change.before}</p>}<p>After ({change.after_state}): {change.after}</p><p className="text-small text-muted">{change.reason}</p></article>) : <p className="text-muted">No beliefs or traditions changed in this round.</p>}</section>}
    {episode.status === 'abandoned'  && <Empty>This photo was set aside. No beliefs or traditions changed.</Empty>}
    </main><aside className="episode-aside"><div className="surface-card aside-card"><p className="eyebrow">One story, four voices</p><h3>What happens next</h3><p className="text-muted">Each agent reacts to the same shared evidence, then they respond to one another. Their debate is temporary. Your historian saves the verdict and meaningful dissent.</p><div className="aside-rule" /><p className="text-small text-muted">A split or unresolved disagreement still completes the discovery.</p></div></aside></div></>
}
function ChronicleDetail({ chronicle }: { chronicle: NonNullable<Episode['chronicle']> }) {
  return <article className="chronicle-detail surface-card"><div className="chronicle-meta"><span className="eyebrow">Historian’s chronicle</span><Badge state={chronicle.outcome === 'unresolved' ? 'attention' : 'complete'}>{chronicle.outcome}</Badge></div><h2>{chronicle.verdict}</h2><div className="chronicle-body">{chronicle.body.split('\n').filter(Boolean).map((paragraph, i) => <p key={i}>{paragraph}</p>)}</div>{chronicle.suggestion && <div className="suggestion surface-subtle"><span className="eyebrow">If you feel curious</span><p>{chronicle.suggestion}</p></div>}</article>
}
