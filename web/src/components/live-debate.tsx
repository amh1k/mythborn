import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { DebateDetail, DebatePreview, Episode } from '../types'
import { Sigil } from './sigil'

const agentNames = { priest: 'The Priest', scientist: 'The Scientist', soldier: 'The Soldier', historian: 'The Historian' }

export function LiveDebate({ episode, token }: { episode: Episode; token: string }) {
  const [detail, setDetail] = useState<DebateDetail | null>(null)
  const [preview, setPreview] = useState<DebatePreview | null>(null)
  const [unavailable, setUnavailable] = useState(false)
  const [now, setNow] = useState(Date.now)
  const terminal = ['complete', 'failed', 'abandoned', 'needs_attention'].includes(episode.status)
  const waiting = !terminal && !!preview?.retries?.length

  useEffect(() => {
    if (!waiting) return
    setNow(Date.now())
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [waiting])

  useEffect(() => { setPreview(null); setDetail(null); setUnavailable(false) }, [episode.id, token])
  useEffect(() => {
    if (['complete', 'abandoned'].includes(episode.status)) return
    const controller = new AbortController()
    let timer: number | undefined
    async function poll() {
      try {
        const next = await api.debate(token, episode.id, controller.signal)
        if (controller.signal.aborted) return
        setDetail(next)
        setUnavailable(next.state === 'unavailable')
        if (next.preview) setPreview(next.preview)
        if (next.state === 'finished') return
      } catch {
        if (controller.signal.aborted) return
        setUnavailable(true)
      }
      if (!terminal) timer = window.setTimeout(() => void poll(), 2000)
    }
    void poll()
    return () => { controller.abort(); if (timer) window.clearTimeout(timer) }
  }, [episode.id, token, terminal])

  const current = preview?.round_id === episode.id ? preview : null
  const entries = current?.entries ?? []
  if (['complete', 'abandoned'].includes(episode.status) && !entries.length) return null
  const phase = current?.phase ?? (episode.stage === 'describing' ? 'describing' : episode.stage === 'debating' ? 'rebuttal' : episode.stage === 'writing' ? 'writing' : 'reaction')
  const showDebate = ['queued', 'describing', 'reacting', 'debating', 'writing'].includes(episode.stage) || entries.length > 0 || waiting
  if (!showDebate) return null
  const reactions = entries.filter(entry => entry.phase === 'reaction')
  const rebuttals = entries.filter(entry => entry.phase === 'rebuttal')
  const role = detail?.player_role
  const phaseEntries = phase === 'reaction' ? reactions : rebuttals
  const paused = ['failed', 'needs_attention'].includes(episode.status)
  const retryMessage = (retryAt: string) => {
    const minutes = Math.ceil((Date.parse(retryAt) - now) / 60000)
    return minutes <= 0 ? 'Rate limit reached. Retrying now…' : `Rate limit reached. Retrying in ${minutes} ${minutes === 1 ? 'minute' : 'minutes'}…`
  }
  const nextRetry = current?.retries?.reduce<string | undefined>((earliest, retry) => !earliest || Date.parse(retry.retry_at) < Date.parse(earliest) ? retry.retry_at : earliest, undefined)
  const notice = terminal
    ? episode.status === 'complete' ? 'The debate has ended. These are the responses you saw during this visit; the historian’s chronicle is the saved record.' : 'The debate has paused. Responses already received remain visible here.'
    : unavailable ? 'The live view is reconnecting. The saved round stage is shown above; responses already received stay visible.'
    : nextRetry ? `${retryMessage(nextRetry)} Responses already received stay visible; the discovery will continue automatically.`
    : episode.stage === 'queued' ? 'This round is waiting to start. Its stage will update automatically.'
    : phase === 'describing' ? 'Preparing the shared photo description before the agents begin.'
    : phase === 'writing' ? 'The historian is turning the debate into a chronicle.'
    : phase === 'rebuttal' ? 'Each agent is responding to the four first reactions.' : 'Each agent is considering the same shared evidence.'

  const content = <section id="agent-debate" className="live-debate" aria-labelledby="debate-heading">
    <div className="section-heading"><div><p className="eyebrow">The agents’ debate</p><h2 id="debate-heading">Four voices, one discovery</h2></div><span className="debate-state">{terminal ? paused ? 'Paused' : 'Ended' : unavailable ? 'Reconnecting' : waiting ? 'Waiting to retry' : episode.stage === 'queued' ? 'Queued' : 'Live'}</span></div>
    <p className="text-muted">{role === 'observer' ? 'You are watching as an Observer. Your agents speak to one another; you cannot join their conversation.' : role === 'god' ? 'Watch your agents interpret the evidence and your proclamation. They reach their own conclusions.' : role === 'messenger' ? 'Watch the shared debate. Direct conversations belong in an agent’s discovery thread after the chronicle is complete.' : 'Watch your agents share their interpretations and respond to one another.'}</p>
    <p className="debate-update text-small" role="status" aria-live="polite">{notice}{!terminal && !unavailable && episode.stage !== 'queued' && ['reaction', 'rebuttal'].includes(phase) && ` ${phaseEntries.length} of ${current?.agents.length ?? 4} ${phase === 'reaction' ? 'reactions' : 'rebuttals'} received.`}</p>
    {current && episode.status !== 'complete' && <ul className="debate-agents" aria-label="Agent progress">{current.agents.map(agent => {
      const received = phaseEntries.some(entry => entry.agent_id === agent.agent_id)
      const retry = current.retries?.find(item => item.agent_id === agent.agent_id && item.phase === phase)
      return <li key={agent.agent_id}><span className={`agent-avatar ${agent.agent_type}`}><Sigil kind={agent.agent_type} /></span><div><strong>{agent.display_name || agentNames[agent.agent_type]}</strong><small>{paused ? received ? 'Response received' : 'Round paused' : retry ? retryMessage(retry.retry_at) : episode.stage === 'queued' ? 'Waiting to start' : phase === 'describing' ? 'Waiting for the photo description' : phase === 'writing' ? agent.agent_type === 'historian' ? 'Writing the chronicle' : 'Debate complete' : received ? 'Response received' : unavailable ? 'Waiting to reconnect' : phase === 'reaction' ? 'Reflecting on the evidence' : 'Considering the other voices'}</small></div></li>
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
