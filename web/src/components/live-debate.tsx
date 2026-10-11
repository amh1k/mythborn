import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { usePolling } from '../lib/use-polling'
import type { DebateDetail, DebatePreview, DebateProgress, Episode } from '../types'
import { debateStage, rateLimitMessage } from '../lib/episode-progress'
import { Sigil } from './sigil'

const agentNames = { priest: 'The Priest', scientist: 'The Scientist', soldier: 'The Soldier', historian: 'The Historian' }

export function LiveDebate({ episode, token, onProgress }: { episode: Episode; token: string; onProgress?: (progress: DebateProgress) => void }) {
  const [detail, setDetail] = useState<DebateDetail | null>(null)
  const [preview, setPreview] = useState<DebatePreview | null>(null)
  const [unavailable, setUnavailable] = useState(false)
  const [now, setNow] = useState(Date.now)
  const terminal = ['complete', 'failed', 'abandoned', 'needs_attention'].includes(episode.status)
  const current = preview?.round_id === episode.id ? preview : null
  const waiting = !terminal && !!current?.retries?.length

  useEffect(() => {
    if (!waiting) return
    setNow(Date.now())
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [waiting])

  useEffect(() => { setPreview(null); setDetail(null); setUnavailable(false) }, [episode.id, token])
  usePolling(`${episode.id}/${token}/${terminal}`, async signal => {
    try {
      const next = await api.debate(token, episode.id, signal)
      if (signal.aborted) return false
      setDetail(next); setUnavailable(next.state === 'unavailable')
      if (next.preview) setPreview(next.preview)
      if (next.state === 'unavailable') throw new Error('Live preview unavailable')
      return !terminal && next.state !== 'finished'
    } catch (e) { if (!signal.aborted) setUnavailable(true); throw e }
  }, 2000, !['complete', 'abandoned'].includes(episode.status))

  useEffect(() => {
    onProgress?.({ preview: current, unavailable, now })
  }, [current, unavailable, now, onProgress])

  const entries = current?.entries ?? []
  if (['complete', 'abandoned'].includes(episode.status) && !entries.length) return null
  const stage = debateStage(episode, unavailable ? null : current)
  const phase = stage === 'queued' || stage === 'describing' ? 'describing' : stage === 'debating' ? 'rebuttal' : stage === 'writing' ? 'writing' : 'reaction'
  const showDebate = ['queued', 'describing', 'reacting', 'debating', 'writing'].includes(episode.stage) || entries.length > 0 || waiting
  if (!showDebate) return null
  const reactions = entries.filter(entry => entry.phase === 'reaction')
  const rebuttals = entries.filter(entry => entry.phase === 'rebuttal')
  const role = detail?.player_role
  const phaseEntries = phase === 'reaction' ? reactions : rebuttals
  const paused = ['failed', 'needs_attention'].includes(episode.status)
  const retryMessage = (retryAt: string) => rateLimitMessage(retryAt, now)
  const nextRetry = current?.retries?.reduce<string | undefined>((earliest, retry) => !earliest || Date.parse(retry.retry_at) < Date.parse(earliest) ? retry.retry_at : earliest, undefined)
  const notice = terminal
    ? episode.status === 'complete' ? 'The debate has ended. These are the responses you saw during this visit; the historian’s chronicle is the saved record.' : 'The debate has paused. Responses already received remain visible here.'
    : unavailable ? 'The live view is reconnecting. The saved round stage is shown above; responses already received stay visible.'
    : nextRetry ? `${retryMessage(nextRetry)} Responses already received stay visible; the discovery will continue automatically. No reload or manual retry is needed.`
    : episode.stage === 'queued' ? 'This round is waiting to start. Its stage will update automatically.'
    : phase === 'describing' ? 'Preparing the shared photo description before the agents begin.'
    : phase === 'writing' ? 'The historian is turning the debate into a chronicle.'
    : phase === 'rebuttal' ? 'Rebuttals are in progress. Each agent is responding to the four first reactions; their rebuttals appear below as they arrive.' : 'First reactions are in progress. Each agent is considering the same shared evidence; responses appear below as they arrive.'
  const heading = terminal ? paused ? 'Debate paused' : 'Completed debate' : episode.stage === 'queued' ? 'Waiting to start' : ({ describing: 'Preparing shared evidence', reaction: 'First reactions', rebuttal: 'Rebuttals in progress', writing: 'Writing the chronicle' })[phase]
  const processing = !terminal && !unavailable && !waiting && episode.stage !== 'queued'

  const content = <section id="agent-debate" className="live-debate" aria-labelledby="debate-heading">
    <div className="section-heading"><div><p className="eyebrow">The agents’ debate</p><h2 id="debate-heading">{heading}</h2></div><span className="debate-state">{processing && <span className="activity-indicator" aria-hidden="true" />}{terminal ? paused ? 'Paused' : 'Ended' : unavailable ? 'Reconnecting' : waiting ? 'Waiting to retry' : episode.stage === 'queued' ? 'Queued' : 'Processing'}</span></div>
    <p className="text-small text-muted">{role === 'observer' ? 'You are watching as an Observer. Your agents speak to one another; you cannot join their conversation.' : role === 'god' ? 'Watch your agents interpret the evidence and your proclamation. They reach their own conclusions.' : role === 'messenger' ? 'Watch the shared debate. Direct conversations belong in an agent’s discovery thread after the chronicle is complete.' : 'Watch your agents share their interpretations and respond to one another.'}</p>
    <p className="debate-update text-small" role="status" aria-live="polite">{notice}{current && !terminal && !unavailable && episode.stage !== 'queued' && ['reaction', 'rebuttal'].includes(phase) && ` ${phaseEntries.length} of ${current.agents.length} ${phase === 'reaction' ? 'reactions' : 'rebuttals'} received.`}</p>
    {current && episode.status !== 'complete' && !entries.length && <ul className="debate-agents" aria-label="Agent progress">{current.agents.map(agent => {
      const received = phaseEntries.some(entry => entry.agent_id === agent.agent_id)
      const retry = current.retries?.find(item => item.agent_id === agent.agent_id && item.phase === phase)
      const working = !terminal && !unavailable && !retry && episode.stage !== 'queued' && (phase === 'writing' ? agent.agent_type === 'historian' : ['reaction', 'rebuttal'].includes(phase) && !received)
      const status = paused ? phase === 'writing' ? agent.agent_type === 'historian' ? 'Chronicle paused' : 'Debate complete' : received ? 'Response received' : 'Round paused' : unavailable ? received && phase !== 'writing' ? 'Response received' : 'Waiting to reconnect' : retry ? retryMessage(retry.retry_at) : episode.stage === 'queued' ? 'Waiting to start' : phase === 'describing' ? 'Waiting for the photo description' : phase === 'writing' ? agent.agent_type === 'historian' ? 'Writing the chronicle…' : 'Debate complete' : received ? phase === 'rebuttal' ? 'Rebuttal received' : 'Reaction received' : phase === 'reaction' ? 'Preparing first reaction…' : 'Preparing rebuttal…'
      return <li key={agent.agent_id}><span className={`agent-avatar ${agent.agent_type}`}><Sigil kind={agent.agent_type} /></span><div><strong>{agent.display_name || agentNames[agent.agent_type]}</strong><small className="agent-progress-status">{working && <span className="activity-indicator" aria-hidden="true" />}{status}</small></div></li>
    })}</ul>}
    {(['reaction', 'rebuttal'] as const).map(item => {
      const contributions = item === 'reaction' ? reactions : rebuttals
      if (!contributions.length && (item === 'reaction' || phase === 'describing' || phase === 'reaction' || terminal)) return null
      return <div className="debate-phase" key={item}><div className="debate-phase-heading"><h3>{item === 'reaction' ? 'First reactions' : 'The rebuttal round'}</h3><span className="text-small text-muted">{contributions.length} / {current?.agents.length ?? 4}</span></div>
        <ol className="debate-contributions">{contributions.map(entry => {
          const agent = current?.agents.find(value => value.agent_id === entry.agent_id)
          return <li key={entry.id}><article className="debate-contribution"><div className="debate-speaker">{agent && <span className={`agent-avatar ${agent.agent_type}`}><Sigil kind={agent.agent_type} /></span>}<div><strong>{agent?.display_name || (agent ? agentNames[agent.agent_type] : 'Agent')}</strong><span className="eyebrow">{item === 'reaction' ? 'First interpretation' : 'Response to the other agents'}</span></div></div><p>{entry.text}</p>{entry.reasoning && <details><summary>Their reasoning</summary><p>{entry.reasoning}</p></details>}</article></li>
        })}</ol>{!contributions.length && <p className="text-muted">The agents have heard the first reactions. Their responses will appear here as they arrive.</p>}
      </div>
    })}
    {!entries.length && <p className="debate-empty">{paused ? 'Open the retry action above to continue this round.' : unavailable ? 'Waiting for the live view to reconnect…' : episode.stage === 'queued' ? 'This round is queued. Agent responses will appear here once processing begins.' : phase === 'describing' ? 'The agents will begin once the photo description is ready.' : 'The first voice will appear here as soon as an agent finishes.'}</p>}
  </section>
  return episode.status === 'complete' ? <details className="watched-debate"><summary>Revisit the debate you watched</summary>{content}</details> : content
}
