import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link, Navigate, NavLink, Outlet, Route, Routes, useNavigate, useParams } from 'react-router-dom'
import { useAuth } from './auth'
import { api, ApiError, asArray } from './lib/api'
import { supabase } from './lib/supabase'
import { Sigil } from './components/sigil'
import { BrandMark } from './components/brand-mark'
import { ThemeToggle } from './theme'
import { AtlasHero } from './components/atlas-hero'
import { worldArtwork } from './lib/world-art'
import type { AdminAccount, Agent, AgentTemplate, Belief, ConversationSummary, Episode, Game, HistoryItem, InitialBeliefTemplate, PlayerRole, Tradition } from './types'

const roleNames: Record<PlayerRole, string> = { observer: 'Observer', god: 'God', messenger: 'Messenger' }
const agentNames: Record<string, string> = { priest: 'The Priest', scientist: 'The Scientist', soldier: 'The Soldier', historian: 'The Historian' }

function useToken() { return useAuth().session?.access_token ?? '' }
function ErrorNotice({ error }: { error: unknown }) {
  return <div className="notice notice-error" role="alert">{error instanceof Error ? error.message : 'Something went wrong. Please try again.'}</div>
}
function Loading({ label = 'Loading…' }: { label?: string }) { return <p className="loading" role="status">{label}</p> }
function Empty({ children }: { children: ReactNode }) { return <div className="empty-state surface-subtle">{children}</div> }
function Badge({ children, state = 'active' }: { children: ReactNode; state?: 'active' | 'complete' | 'attention' }) { return <span className="status-pill" data-state={state}>{children}</span> }

function App() {
  const { user, loading, configured } = useAuth()
  if (!configured) return <ConfigurationScreen />
  if (loading) return <main className="container app-loading"><Loading label="Opening Mythborn…" /></main>
  return <Routes>
    <Route path="/sign-in" element={user ? <Navigate to="/worlds" replace /> : <SignIn />} />
    <Route element={user ? <Shell /> : <Navigate to="/sign-in" replace />}>
      <Route path="/" element={<Navigate to="/worlds" replace />} />
      <Route path="/worlds" element={<WorldList />} />
      <Route path="/worlds/new" element={<CreateWorld />} />
      <Route path="/worlds/:worldId" element={<WorldPage />} />
      <Route path="/episodes/:episodeId" element={<EpisodePage />} />
      <Route path="/admin/*" element={<AdminPortal />} />
    </Route>
    <Route path="*" element={<Navigate to={user ? '/worlds' : '/sign-in'} replace />} />
  </Routes>
}

function ConfigurationScreen() {
  return <main className="config-screen container"><div className="config-topline"><BrandMark /><ThemeToggle /></div><p className="eyebrow">Mythborn setup</p><h1>Connect Mythborn</h1><p className="text-muted">Add the public Supabase and API settings to load the sign-in screen.</p><div className="surface-card config-card"><code>VITE_SUPABASE_URL</code><code>VITE_SUPABASE_PUBLISHABLE_KEY</code><code>VITE_API_BASE_URL</code><p>These are browser settings. Keep service keys and model credentials on the server.</p></div></main>
}

function Shell() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  async function signOut() { await supabase?.auth.signOut(); navigate('/sign-in') }
  return <><a className="skip-link" href="#main-content">Skip to content</a><header className="site-header"><div className="container header-inner">
    <Link className="brand" to="/worlds" aria-label="Mythborn home"><BrandMark /><span>Mythborn</span></Link>
    <div className="header-actions"><ThemeToggle /><button className="mobile-menu button button-secondary" aria-expanded={menuOpen} aria-controls="primary-navigation" onClick={() => setMenuOpen(v => !v)}>Menu</button>
    <nav id="primary-navigation" className={menuOpen ? 'primary-nav is-open' : 'primary-nav'} aria-label="Main navigation">
      <NavLink to="/worlds" onClick={() => setMenuOpen(false)}>My worlds</NavLink><NavLink to="/admin" onClick={() => setMenuOpen(false)}>Admin</NavLink>
      <span className="account-label">{user?.email ?? 'Your account'}</span><button className="button button-quiet signout" onClick={signOut}>Sign out</button>
    </nav></div>
  </div></header><main id="main-content" className="container app-main"><Outlet /></main><footer className="site-footer"><div className="container"><span>MYTHBORN</span><span>Civilizations, beliefs & the stories between.</span></div></footer></>
}

function SignIn() {
  const [mode, setMode] = useState<'sign-in' | 'sign-up' | 'recovery'>('sign-in')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState<unknown>(null)
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError(null); setMessage('')
    try {
      if (!supabase) throw new Error('Sign in is not configured yet.')
      if (mode === 'recovery') {
        const { error: authError } = await supabase.auth.resetPasswordForEmail(email)
        if (authError) throw authError
        setMessage('If an account uses that email, a recovery link is on its way.')
      } else if (mode === 'sign-up') {
        const { error: authError } = await supabase.auth.signUp({ email, password })
        if (authError) throw authError
        setMessage('Check your email to confirm your account, then return here to sign in.')
      } else {
        const { error: authError } = await supabase.auth.signInWithPassword({ email, password })
        if (authError) throw authError
      }
    } catch (caught) { setError(caught) } finally { setBusy(false) }
  }
  async function oauth(provider: 'google' | 'github') {
    setError(null)
    const { error: authError } = await supabase!.auth.signInWithOAuth({ provider, options: { redirectTo: window.location.origin } })
    if (authError) setError(authError)
  }
  const title = mode === 'sign-up' ? 'Begin a new world' : mode === 'recovery' ? 'Find your way back' : 'Return to your world'
  return <main className="auth-layout container"><section className="auth-story"><Link className="brand" to="/sign-in"><BrandMark /><span>Mythborn</span></Link><div className="auth-story-copy"><p className="eyebrow">A civilization shaped by you</p><h1>The world is real.<br /><em>The myth is yours.</em></h1><p>A photograph. Four competing perspectives. Watch a civilization turn your discoveries into beliefs, rituals, and history.</p></div><div className="story-stamp"><span>01 — THE BEGINNING</span><span>Every civilization starts<br />with something unexplained.</span></div></section>
    <section className="auth-card surface-card" aria-labelledby="auth-title"><div className="auth-card-tools"><ThemeToggle /></div><p className="eyebrow">Enter Mythborn</p><h2 id="auth-title">{title}</h2><p className="text-muted">{mode === 'sign-up' ? 'Create an account to start your first civilization.' : mode === 'recovery' ? 'We’ll send a secure recovery link if the account exists.' : 'Continue where your civilization left off.'}</p>
      <form onSubmit={submit} className="form-stack"><div><label htmlFor="email">Email address</label><input id="email" autoComplete="email" type="email" required value={email} onChange={e => setEmail(e.target.value)} /></div>{mode !== 'recovery' && <div><label htmlFor="password">Password</label><input id="password" autoComplete={mode === 'sign-up' ? 'new-password' : 'current-password'} type="password" minLength={8} required value={password} onChange={e => setPassword(e.target.value)} /></div>}
        {error !== null && <ErrorNotice error={error} />}{message && <div className="notice notice-success" role="status">{message}</div>}
        <button className="button full-width" disabled={busy}>{busy ? 'Please wait…' : mode === 'sign-up' ? 'Create account' : mode === 'recovery' ? 'Send recovery link' : 'Sign in'}</button>
      </form>
      {mode === 'sign-in' && <><div className="auth-divider"><span>or continue with</span></div><div className="oauth-row"><button className="button button-secondary" onClick={() => void oauth('google')}>Google</button><button className="button button-secondary" onClick={() => void oauth('github')}>GitHub</button></div></>}
      <div className="auth-links">{mode !== 'sign-in' && <button className="text-button" onClick={() => { setMode('sign-in'); setError(null); setMessage('') }}>Back to sign in</button>}{mode === 'sign-in' && <><button className="text-button" onClick={() => setMode('recovery')}>Forgot password?</button><span>New here? <button className="text-button" onClick={() => setMode('sign-up')}>Create account</button></span></>}</div>
    </section></main>
}

function WorldList() {
  const token = useToken(); const [games, setGames] = useState<Game[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState<unknown>(null)
  useEffect(() => { let alive = true; void api.worlds(token).then(v => { if (alive) setGames(asArray(v, 'worlds')) }).catch(e => alive && setError(e)).finally(() => alive && setLoading(false)); return () => { alive = false } }, [token])
  const active = games.filter(g => !['archived', 'deleting'].includes(g.status)); const archives = games.filter(g => g.status === 'archived')
  return <><AtlasHero />
    {error !== null && <ErrorNotice error={error} />}{loading ? <Loading label="Gathering your worlds…" /> : !error && games.length === 0 ? <section className="welcome-card"><div className="welcome-illustration"><Sigil /></div><div><p className="eyebrow">No civilizations yet</p><h2>The first chapter is unwritten.</h2><p className="text-muted">Name a civilization and choose your role. Its story begins with your first photograph.</p></div><Link className="button button-secondary" to="/worlds/new">Create your first world <span aria-hidden="true">→</span></Link></section> : !error && <>
      <section className="world-section"><div className="section-heading"><h2>In progress</h2><span>{active.length} {active.length === 1 ? 'world' : 'worlds'}</span></div>{active.length ? <div className="world-grid">{active.map(game => <WorldCard key={game.id} game={game} />)}</div> : <Empty>No civilizations in progress. Start a new one when you’re ready.</Empty>}</section>
      {archives.length > 0 && <section className="world-section"><div className="section-heading"><h2>Archives</h2><span>{archives.length}</span></div><div className="world-grid">{archives.map(game => <WorldCard key={game.id} game={game} />)}</div></section>}
    </>}</>
}
function WorldCard({ game }: { game: Game }) {
  const archived = game.status === 'archived'
  return <Link className="world-card surface-card" to={`/worlds/${game.id}`}><div className="world-cover"><img className="world-cover-art" src={worldArtwork(game.id)} width="1536" height="1024" alt="" loading="lazy" decoding="async" /><span className="world-symbol"><Sigil kind={game.player_role} /></span><span className="world-cover-label">{roleNames[game.player_role]}</span></div><div className="world-card-top"><Badge state={archived ? 'complete' : game.status === 'active' ? 'active' : 'attention'}>{archived ? 'Archived' : game.status.replaceAll('_', ' ')}</Badge></div><h3>{game.name}</h3><p className="text-muted">{roleNames[game.player_role]} · Began {formatDate(game.created_at)}</p>{game.latest_episode?.chronicle && <p className="world-excerpt">“{game.latest_episode.chronicle.verdict}”</p>}<span className="card-link">{archived ? 'Read the archive' : 'Open civilization'} <span aria-hidden="true">→</span></span></Link>
}
function CreateWorld() {
  const token = useToken(); const navigate = useNavigate(); const [name, setName] = useState(''); const [role, setRole] = useState<PlayerRole>('observer'); const [busy, setBusy] = useState(false); const [error, setError] = useState<unknown>(null)
  async function submit(e: FormEvent) { e.preventDefault(); setBusy(true); setError(null); try { const game = await api.createWorld(token, { name: name.trim(), player_role: role, idempotency_key: crypto.randomUUID() }); navigate(`/worlds/${game.id}`) } catch (e) { setError(e) } finally { setBusy(false) } }
  const roleInfo: Record<PlayerRole, string> = { observer: 'Offer no direct message. Let the agents interpret what the photo shows.', god: 'Share a divine proclamation with each photo. The agents may accept or question it.', messenger: 'Share testimony and speak directly with an agent about the discovery.' }
  return <div className="creation-layout"><aside className="creation-plate"><p className="eyebrow">The founding</p><h2>A name.<br />A perspective.<br /><em>A beginning.</em></h2><span className="plate-caption">THE FIRST STONES ARE LAID</span></aside><div className="narrow-page"><Link className="back-link" to="/worlds">← All worlds</Link><p className="eyebrow">A new beginning</p><h1>Name your civilization</h1><p className="text-muted">Your role is fixed for this world. You can choose a different one in another civilization.</p><form className="form-stack" onSubmit={submit}><div><label htmlFor="world-name">Civilization name</label><input id="world-name" maxLength={80} required value={name} onChange={e => setName(e.target.value)} placeholder="The Valley of First Rain" /></div><fieldset className="role-picker"><legend>Your role in this world</legend>{(['observer', 'god', 'messenger'] as const).map(value => <label className={role === value ? 'role-option selected' : 'role-option'} key={value}><input type="radio" name="role" value={value} checked={role === value} onChange={() => setRole(value)} /><span className="role-icon" aria-hidden="true"><Sigil kind={value} /></span><span><strong>{roleNames[value]}</strong><small>{roleInfo[value]}</small></span></label>)}</fieldset>{error !== null && <ErrorNotice error={error} />}<div className="privacy-note"><strong>Before the first photo</strong><p>Photos are sent to an external AI service to create a shared description. The service may use submitted content to improve its products. You can review this before starting.</p></div><div className="form-actions"><Link className="button button-secondary" to="/worlds">Cancel</Link><button className="button" disabled={busy || !name.trim()}>{busy ? 'Preparing your world…' : 'Create civilization'}</button></div></form></div></div>
}

function WorldPage() {
  const { worldId = '' } = useParams(); const token = useToken(); const [game, setGame] = useState<Game | null>(null); const [agents, setAgents] = useState<Agent[]>([]); const [history, setHistory] = useState<HistoryItem[]>([]); const [traditions, setTraditions] = useState<Tradition[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState<unknown>(null); const [resourceError, setResourceError] = useState<unknown>(null); const [tab, setTab] = useState<'home' | 'agents' | 'culture' | 'history' | 'settings'>('home')
  const refresh = async () => {
    const w = await api.world(token, worldId); setGame(w)
    const results = await Promise.allSettled([api.agents(token, worldId), api.history(token, worldId), api.traditions(token, worldId)])
    const failures: unknown[] = []
    if (results[0].status === 'fulfilled') setAgents(asArray(results[0].value, 'agents')); else failures.push(results[0].reason)
    if (results[1].status === 'fulfilled') setHistory(asArray(results[1].value, 'history')); else failures.push(results[1].reason)
    if (results[2].status === 'fulfilled') setTraditions(asArray(results[2].value, 'traditions')); else failures.push(results[2].reason)
    setResourceError(failures[0] ?? null)
  }
  useEffect(() => { let alive = true; void refresh().catch(e => alive && setError(e)).finally(() => alive && setLoading(false)); return () => { alive = false } }, [worldId, token])
  useEffect(() => {
    if (game?.status !== 'starting') return
    let alive = true; let timer: number | undefined
    const check = async () => { try { const latest = await api.world(token, worldId); if (!alive) return; setGame(latest); if (latest.status === 'starting') timer = window.setTimeout(() => void check(), 2000) } catch { if (alive) timer = window.setTimeout(() => void check(), 4000) } }
    timer = window.setTimeout(() => void check(), 2000)
    return () => { alive = false; if (timer) window.clearTimeout(timer) }
  }, [game?.status, worldId, token])
  if (loading) return <Loading label="Opening this civilization…" />
  if (error || !game) return <ErrorNotice error={error ?? new Error('This world could not be found.')} />
  return <><Link className="back-link" to="/worlds">← All worlds</Link><div className="world-banner" style={{ backgroundImage: `url(${worldArtwork(game.id)})` }}><div><p className="eyebrow">{roleNames[game.player_role]} civilization</p><h1>{game.name}</h1><span className="world-founded">Founded {formatDate(game.created_at)}</span></div><Badge state={game.status === 'active' ? 'active' : game.status === 'archived' ? 'complete' : 'attention'}>{game.status.replaceAll('_', ' ')}</Badge></div>{resourceError !== null && <ErrorNotice error={resourceError} />}
    <nav className="world-tabs" aria-label="Civilization sections">{(['home','agents','culture','history','settings'] as const).map(item => <button key={item} className={tab === item ? 'tab active' : 'tab'} aria-current={tab === item ? 'page' : undefined} onClick={() => setTab(item)}>{({home:'Overview',agents:'Agents',culture:'Culture',history:'History',settings:'Settings'})[item]}</button>)}</nav>
    {tab === 'home' && <Overview game={game} history={history} token={token} onRefresh={refresh} />}{tab === 'agents' && <AgentsPanel agents={agents} game={game} history={history} token={token} />}{tab === 'culture' && <CulturePanel traditions={traditions} />}{tab === 'history' && <HistoryPanel history={history} />}{tab === 'settings' && <SettingsPanel game={game} token={token} onRefresh={refresh} />}</>
}

function Overview({ game, history, token, onRefresh }: { game: Game; history: HistoryItem[]; token: string; onRefresh: () => Promise<void> }) {
  const [file, setFile] = useState<File | null>(null); const [statement, setStatement] = useState(''); const [busy, setBusy] = useState(false); const [error, setError] = useState<unknown>(null); const [reuse, setReuse] = useState(false); const [duplicateId, setDuplicateId] = useState('')
  const canObserve = game.status === 'active'
  async function submit(e: FormEvent) { e.preventDefault(); if (!file) return; setBusy(true); setError(null); setDuplicateId(''); try { const form = new FormData(); form.set('photo', file); if (game.player_role !== 'observer' && statement) form.set('player_statement', statement); form.set('allow_reuse', String(reuse)); form.set('idempotency_key', crypto.randomUUID()); const accepted = await api.submitObservation(token, game.id, form); window.location.href = `/episodes/${accepted.episode_id}` } catch (caught) { if (caught instanceof ApiError && caught.status === 409) setDuplicateId(caught.message); else setError(caught) } finally { setBusy(false) } }
  return <div className="overview-grid"><div className="overview-main"><section className="discovery-card surface-card"><div className="discovery-icon"><Sigil kind="camera" /></div><p className="eyebrow">The next discovery</p><h2>Bring the world a discovery.</h2><p className="text-muted">A photo begins a conversation among your four agents. Any photo is welcome, whether or not it follows a suggestion.</p>
      {history[0]?.suggestion && <div className="suggestion surface-subtle"><span className="eyebrow">A thought from the historian</span><p>“{history[0].suggestion}”</p></div>}
      {canObserve ? <form className="observation-form" onSubmit={submit}><label className="upload-zone"><input type="file" accept="image/jpeg,image/png,image/webp" capture="environment" onChange={e => setFile(e.target.files?.[0] ?? null)} /><span className="upload-symbol" aria-hidden="true">+</span><strong>{file ? file.name : 'Take a photo or choose one'}</strong><small>JPEG, PNG, or WebP · up to 10 MB</small></label>
       {game.player_role !== 'observer' && <div><label htmlFor="statement">{game.player_role === 'god' ? 'Your proclamation' : 'Your testimony'} <span className="optional">(optional)</span></label><textarea id="statement" maxLength={1200} value={statement} onChange={e => setStatement(e.target.value)} placeholder={game.player_role === 'god' ? 'Tell your agents what this sign means…' : 'Share what you noticed or what you know…'} /></div>}
      {error !== null && <ErrorNotice error={error} />}{duplicateId && <div className="notice notice-error" role="alert">{duplicateId}<div className="duplicate-actions"><label className="inline-check"><input type="checkbox" checked={reuse} onChange={e => setReuse(e.target.checked)} /> Submit this same image as a new discovery</label></div></div>}
       <button className="button" disabled={!file || busy}>{busy ? 'Starting discovery…' : 'Begin discovery'}</button><p className="text-small text-muted">Your photo is processed by an external AI service to create a neutral description. Location metadata is not used.</p>
      </form> : <Empty>This civilization is {game.status === 'archived' ? 'archived and read-only' : game.status.replaceAll('_', ' ')}. New observations are unavailable.</Empty>}</section>
      <section className="section-block"><div className="section-heading"><h2>Recent chronicles</h2><button className="text-button" onClick={() => void onRefresh()}>Refresh</button></div><HistoryPanel history={history.slice(0, 3)} compact /></section>
    </div><aside className="overview-aside"><div className="surface-card aside-card"><p className="eyebrow">Your companions</p><h3>Four ways of seeing</h3>{(['priest','scientist','soldier','historian'] as const).map(type => <div className="agent-mini" key={type}><span className={`agent-avatar ${type}`} aria-hidden="true"><Sigil kind={type} /></span><span><strong>{agentNames[type]}</strong><small>{{priest:'Meaning & ritual',scientist:'Evidence & patterns',soldier:'Safety & survival',historian:'Memory & continuity'}[type]}</small></span></div>)}<span className="aside-footnote">Each keeps a distinct memory of this world.</span></div>
      <div className="surface-subtle role-card"><p className="eyebrow">Your role</p><h3>{roleNames[game.player_role]}</h3><p>{game.player_role === 'observer' ? 'You provide the evidence. Your agents decide what it means.' : game.player_role === 'god' ? 'You can proclaim meaning, while your agents keep their own minds.' : 'You can share testimony and speak with an agent during a discovery.'}</p></div></aside></div>
}

function AgentsPanel({ agents, game, history, token }: { agents: Agent[]; game: Game; history: HistoryItem[]; token: string }) {
  const [selected, setSelected] = useState<Agent | null>(null); const [beliefs, setBeliefs] = useState<Belief[]>([]); const [loading, setLoading] = useState(false); const [error, setError] = useState<unknown>(null); const [message, setMessage] = useState(''); const [chatBusy, setChatBusy] = useState(false); const [reply, setReply] = useState(''); const [summary, setSummary] = useState<ConversationSummary | null>(null); const [summaryVersion, setSummaryVersion] = useState(0); const [chatError, setChatError] = useState<unknown>(null)
  const latestEpisode = history.find(x => x.round_kind === 'discovery')
  useEffect(() => { if (!selected) return; let alive = true; setLoading(true); setError(null); void api.beliefs(token, game.id, selected.id).then(v => alive && setBeliefs(asArray(v,'beliefs'))).catch(e => alive && setError(e)).finally(() => alive && setLoading(false)); return () => { alive = false } }, [selected?.id, game.id, token])
  useEffect(() => {
    if (game.player_role !== 'messenger' || !selected || !latestEpisode) { setSummary(null); setSummaryVersion(0); return }
    let alive = true; setChatError(null)
    void api.conversation(token, latestEpisode.round_id, selected.id).then(thread => { if (alive) { setSummary(thread.summary); setSummaryVersion(thread.summary_version) } }).catch(e => alive && setChatError(e))
    return () => { alive = false }
  }, [game.player_role, latestEpisode?.round_id, selected?.id, token])
  async function sendMessage(e: FormEvent) { e.preventDefault(); if (!selected || !latestEpisode || !message.trim()) return; setChatBusy(true); setChatError(null); try { const result = await api.sendMessage(token, latestEpisode.round_id, selected.id, message.trim(), summary?.version ?? summaryVersion); setMessage(''); setReply(''); const poll = async () => { try { const status = await api.command(token, result.command_id); if (status.reply) setReply(replyText(status.reply)); if (status.summary) { setSummary(status.summary); setSummaryVersion(status.summary.version) } if (['pending', 'dispatched', 'running', 'queued'].includes(status.status)) window.setTimeout(() => void poll(), 1500) } catch (e) { setChatError(e) } }; void poll() } catch (e) { setChatError(e) } finally { setChatBusy(false) } }
  const list = agents
  return <div className="content-columns"><section><div className="section-heading"><div><p className="eyebrow">Four perspectives</p><h2>Your agents</h2></div></div>{list.length ? <div className="agent-grid">{list.map(agent => <button key={agent.id} className={`agent-card surface-card ${selected?.id === agent.id ? 'chosen' : ''}`} onClick={() => { setSelected(agent); setReply('') }}><span className={`agent-avatar large ${agent.agent_type}`} aria-hidden="true"><Sigil kind={agent.agent_type} /></span><span className="eyebrow">{agent.agent_type}</span><strong>{agent.display_name ?? agentNames[agent.agent_type]}</strong><span className="text-small text-muted">{agent.role_summary ?? {priest:'Sacred meaning, prophecy, and ritual.',scientist:'Mechanisms, observations, and uncertainty.',soldier:'Threats, security, and survival.',historian:'Earlier events and competing accounts.'}[agent.agent_type]}</span></button>)}</div> : <Empty>Agent records are not available yet.</Empty>}</section>
    <section className="surface-card detail-panel">{!selected ? <Empty>Choose an agent to read their current beliefs and memory.</Empty> : <><p className="eyebrow">{agentNames[selected.agent_type]}</p><h2>{selected.display_name ?? agentNames[selected.agent_type]}</h2><div className="section-heading"><h3>Current beliefs</h3></div>{loading ? <Loading /> : error ? <ErrorNotice error={error} /> : beliefs.length ? <ul className="belief-list">{beliefs.map(b => <li key={b.id}><p>{b.claim}</p><Badge state={b.state === 'abandoned' ? 'attention' : b.state === 'held' ? 'complete' : 'active'}>{b.state}</Badge>{Boolean(b.revisions?.length) && <details><summary>Belief history ({b.revisions!.length})</summary><ul>{b.revisions!.map(r => <li key={r.id}><strong>{r.change_type}</strong>{r.reason && ` · ${r.reason}`}</li>)}</ul></details>}</li>)}</ul> : <Empty>No beliefs are available yet. They develop as this world gathers evidence.</Empty>}
      {game.player_role === 'messenger' && <div className="messenger-box"><h3>Speak with this agent</h3>{latestEpisode ? <><p className="text-small text-muted">Conversation is tied to the latest discovery. Mythborn keeps a short memory summary, not a transcript.</p>{summary && <div className="summary-box"><span className="eyebrow">Conversation memory</span><p>{summary.summary}</p></div>}{reply && <div className="agent-reply"><span className="eyebrow">{agentNames[selected.agent_type]} replies</span><p>{reply}</p></div>}{chatError !== null && <ErrorNotice error={chatError} />}<form onSubmit={sendMessage}><label htmlFor="agent-message">Message</label><textarea id="agent-message" maxLength={1000} value={message} onChange={e => setMessage(e.target.value)} placeholder="Ask what the agent made of the discovery…" /><button className="button" disabled={chatBusy || !message.trim()}>{chatBusy ? 'Sending…' : 'Send message'}</button></form></> : <Empty>A discovery is needed before a Messenger conversation can begin.</Empty>}</div>}
    </>}</section></div>
}

function CulturePanel({ traditions }: { traditions: Tradition[] }) {
  const groups = (['adopted','contested','retired'] as const).map(state => [state, traditions.filter(t => t.state === state)] as const)
  return <><div className="page-heading"><div><p className="eyebrow">Shared memory</p><h2>The culture of this world</h2><p className="text-muted">Traditions grow through the agents’ shared support and change as new evidence arrives.</p></div></div>{traditions.length === 0 ? <Empty>No myths, rituals, or taboos have taken root yet. Culture begins with what your agents decide matters.</Empty> : <div className="culture-columns">{groups.map(([state, items]) => <section className="culture-group" key={state}><div className="section-heading"><h3>{state}</h3><span>{items.length}</span></div>{items.length ? items.map(item => <article className="tradition-card surface-card" key={item.id}><span className="eyebrow">{item.type}</span><h3>{item.title}</h3>{item.description && <p>{item.description}</p>}<Badge state={state === 'adopted' ? 'complete' : state === 'retired' ? 'attention' : 'active'}>{state}</Badge>{item.supporters && <p className="text-small text-muted">Supported by {item.supporters.length} agents</p>}</article>) : <p className="text-small text-muted">Nothing here yet.</p>}</section>)}</div>}</>
}

function HistoryPanel({ history, compact = false }: { history: HistoryItem[]; compact?: boolean }) {
  if (!history.length) return <Empty>No chronicles yet. Your historian will write the first after a discovery.</Empty>
  return <div className={compact ? 'chronicle-list compact' : 'chronicle-list'}>{history.map((item, index) => <article className="chronicle-card surface-card" key={item.id}><div className="chronicle-meta"><span className="eyebrow">{item.round_kind ?? 'discovery'} · {formatDate(item.created_at)}</span><Badge state={item.outcome === 'unresolved' ? 'attention' : 'complete'}>{item.outcome}</Badge></div><h3>{item.verdict}</h3><p>{item.body}</p>{item.suggestion && !compact && <div className="suggestion surface-subtle"><span className="eyebrow">A thought for another day</span><p>{item.suggestion}</p></div>}{index > 0 && compact && null}</article>)}</div>
}

function SettingsPanel({ game, token, onRefresh }: { game: Game; token: string; onRefresh: () => Promise<void> }) {
  const navigate = useNavigate(); const [busy, setBusy] = useState(false); const [error, setError] = useState<unknown>(null); const [message, setMessage] = useState('')
  async function toggleReview(value: boolean) { setBusy(true); setError(null); setMessage(''); try { await api.settings(token, game.id, value); await onRefresh(); setMessage('Photo description setting saved.') } catch (e) { setError(e) } finally { setBusy(false) } }
  async function endWorld() { if (!window.confirm('End this civilization? The historian will write a closing chronicle and the archive will become read-only.')) return; setBusy(true); setError(null); try { await api.endWorld(token, game.id); await onRefresh(); setMessage('Ending requested. This page will update when the closing chronicle is saved.') } catch (e) { setError(e) } finally { setBusy(false) } }
  async function deleteWorld() { if (!window.confirm('Delete this civilization and its photos, beliefs, and chronicles? This cannot be undone.')) return; setBusy(true); setError(null); try { await api.deleteWorld(token, game.id); window.location.href = '/worlds' } catch (e) { setError(e); setBusy(false) } }
  async function deleteAccount() { if (!window.confirm('Delete your account and all its civilizations, photos, and history? This cannot be undone.')) return; setBusy(true); setError(null); try { await api.deleteAccount(token); await supabase?.auth.signOut(); navigate('/sign-in') } catch (e) { setError(e); setBusy(false) } }
  return <div className="narrow-page settings-page"><p className="eyebrow">Civilization settings</p><h2>Shape how your world is handled</h2>{error !== null && <ErrorNotice error={error} />}{message && <div className="notice notice-success" role="status">{message}</div>}
    <section className="settings-row"><div><h3>Review photo descriptions</h3><p className="text-muted">Pause after the shared visual description so you can correct facts before agents see them. Unclear photos always offer a choice.</p></div><label className="switch-label"><input type="checkbox" checked={Boolean(game.review_photo_description)} disabled={busy || game.status !== 'active'} onChange={e => void toggleReview(e.target.checked)} /><span>{game.review_photo_description ? 'On' : 'Off'}</span></label></section>
    <section className="settings-row"><div><h3>End civilization</h3><p className="text-muted">The historian writes a closing chronicle. You can still read the archive afterward.</p></div><button className="button button-secondary" disabled={busy || game.status !== 'active'} onClick={() => void endWorld()}>End civilization</button></section>
    <section className="settings-row danger-row"><div><h3>Delete civilization</h3><p className="text-muted">Remove this world and its stored photos and history.</p></div><button className="button button-danger" disabled={busy || game.status === 'deleting'} onClick={() => void deleteWorld()}>Delete</button></section>
    <section className="settings-row danger-row"><div><h3>Delete account</h3><p className="text-muted">Remove all of your civilizations and sign-in data.</p></div><button className="button button-danger" disabled={busy} onClick={() => void deleteAccount()}>Delete account</button></section>
  </div>
}

function EpisodePage() {
  const { episodeId = '' } = useParams(); const token = useToken(); const [episode, setEpisode] = useState<Episode | null>(null); const [loading, setLoading] = useState(true); const [error, setError] = useState<unknown>(null); const [busy, setBusy] = useState(false); const [correction, setCorrection] = useState(''); const [photo, setPhoto] = useState(''); const [commandId, setCommandId] = useState('')
  const reload = () => api.episode(token, episodeId).then(next => { setEpisode(next); return next })
  useEffect(() => { let alive = true; let timer: number | undefined; async function fetchEpisode() { try { const next = await api.episode(token, episodeId); if (!alive) return; setEpisode(next); setError(null); if (next.observation_id) { void api.photoUrl(token, next.observation_id).then(v => alive && setPhoto(v.url)).catch(() => undefined) } if (!['complete','failed','abandoned'].includes(next.status)) timer = window.setTimeout(() => void fetchEpisode(), 2000) } catch (e) { if (alive) setError(e) } finally { if (alive) setLoading(false) } } void fetchEpisode(); return () => { alive = false; if (timer) window.clearTimeout(timer) } }, [episodeId, token])
  async function decision(payload: Parameters<typeof api.descriptionDecision>[2]) { setBusy(true); setError(null); try { const result = await api.descriptionDecision(token, episodeId, payload); if (result.command_id) setCommandId(result.command_id); await reload() } catch (e) { setError(e) } finally { setBusy(false) } }
  async function retry() { setBusy(true); setError(null); try { const result = await api.retryEpisode(token, episodeId); if (result.command_id) setCommandId(result.command_id); await reload() } catch (e) { setError(e) } finally { setBusy(false) } }
  if (loading) return <Loading label="Reconnecting to this discovery…" />
  if (error && !episode) return <><Link className="back-link" to="/worlds">← Your worlds</Link><ErrorNotice error={error} /></>
  if (!episode) return <ErrorNotice error={new Error('This discovery could not be found.')} />
  const stageLabel: Record<string,string> = { queued:'Preparing discovery', describing:'Describing the photo', awaiting_review:'Review the description', awaiting_clarity_choice:'Choose how to continue', reacting:'Agents are reflecting', debating:'Agents are responding', writing:'Historian is writing', complete:'Chronicle complete', failed:'Discovery stopped', needs_attention:'Needs attention', abandoned:'Discovery set aside' }
  const stepOrder = ['describing','reacting','debating','writing','complete']; const step = episode.stage === 'awaiting_review' || episode.stage === 'awaiting_clarity_choice' ? 0 : Math.max(0, stepOrder.indexOf(episode.stage)); const done = episode.status === 'complete'
  return <><Link className="back-link" to="/worlds">← Your worlds</Link><div className="episode-layout"><main className="episode-main"><div className="page-heading episode-heading"><div><p className="eyebrow">Discovery · {formatDate(episode.created_at)}</p><h1>{done ? 'The chronicle' : 'A discovery is unfolding'}</h1></div><Badge state={done ? 'complete' : ['failed','needs_attention'].includes(episode.status) ? 'attention' : 'active'}>{stageLabel[episode.stage] ?? episode.status}</Badge></div>
    {photo && <img className="episode-photo" src={photo} alt="Your submitted observation" />}
    {!done && !['failed','needs_attention','abandoned'].includes(episode.status) && <section className="progress-card surface-card"><div className="progress-heading"><div><span className="eyebrow">Your world is thinking</span><h2>{stageLabel[episode.stage] ?? 'Discovery in progress'}</h2></div><span className="live-dot" aria-label="Updating automatically" /></div><ol className="progress-steps">{['Shared evidence','First reactions','Rebuttals','Chronicle'].map((label, i) => <li className={i < step ? 'complete' : i === step ? 'current' : ''} key={label}><span className="step-mark">{i < step ? '✓' : i + 1}</span><span>{label}</span></li>)}</ol><p className="text-muted">You can leave this page. Progress is saved and will be here when you return.</p></section>}
    {episode.description && <section className="evidence-card surface-subtle"><span className="eyebrow">Shared photo description</span><p>{episode.description}</p>{episode.description_status === 'awaiting_review' && <span className="text-small text-muted">This description is waiting for your review.</span>}</section>}
    {episode.stage === 'awaiting_review' && <section className="decision-card surface-card"><h2>Does this match what you see?</h2><p className="text-muted">Correct visible details if needed. Meaning and interpretation are for you and your agents to explore.</p><label htmlFor="correction">Correct the description <span className="optional">(optional)</span></label><textarea id="correction" value={correction} maxLength={1800} onChange={e => setCorrection(e.target.value)} placeholder="For example: the object in the lower corner is a seed, not a stone." />{error !== null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-secondary" disabled={busy} onClick={() => void decision({ decision:'accept' })}>Accept description</button><button className="button" disabled={busy || !correction.trim()} onClick={() => void decision({ decision:'correct', player_correction: correction.trim() })}>Save correction</button></div></section>}
    {episode.stage === 'awaiting_clarity_choice' && <section className="decision-card surface-card"><h2>The photo is hard to read</h2><p className="text-muted">The description is uncertain. You can still continue, and agents will be asked not to treat uncertain details as fact.</p>{error !== null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-secondary" disabled={busy} onClick={() => void decision({ decision:'choose_clarity', clarity_choice:'try_another_photo' })}>Try another photo</button><button className="button" disabled={busy} onClick={() => void decision({ decision:'choose_clarity', clarity_choice:'continue_uncertain' })}>Continue with uncertainty</button></div></section>}
    {error !== null && episode && !['awaiting_review','awaiting_clarity_choice'].includes(episode.stage) && <ErrorNotice error={error} />}
    {commandId && <p className="text-small text-muted" role="status">Command {commandId} accepted. Reconnecting to progress…</p>}
    {(['failed','needs_attention'].includes(episode.status)) && <section className="attention-card"><h2>This discovery needs another try</h2><p>{episode.error_message ?? 'Processing paused before the chronicle was saved. Your evidence remains safe.'}</p>{error !== null && <ErrorNotice error={error} />}<button className="button" disabled={busy} onClick={() => void retry()}>{busy ? 'Retrying…' : 'Retry discovery'}</button></section>}
    {episode.chronicle && <ChronicleDetail chronicle={episode.chronicle} />}
    {episode.status === 'abandoned' && <Empty>This photo was set aside. No beliefs or traditions changed.</Empty>}
    </main><aside className="episode-aside"><div className="surface-card aside-card"><p className="eyebrow">One story, four voices</p><h3>What happens next</h3><p className="text-muted">Each agent reacts to the same shared evidence, then they respond to one another. Their debate is temporary. Your historian saves the verdict and meaningful dissent.</p><div className="aside-rule" /><p className="text-small text-muted">A split or unresolved disagreement still completes the discovery.</p></div></aside></div></>
}
function ChronicleDetail({ chronicle }: { chronicle: NonNullable<Episode['chronicle']> }) {
  return <article className="chronicle-detail surface-card"><div className="chronicle-meta"><span className="eyebrow">Historian’s chronicle</span><Badge state={chronicle.outcome === 'unresolved' ? 'attention' : 'complete'}>{chronicle.outcome}</Badge></div><h2>{chronicle.verdict}</h2><div className="chronicle-body">{chronicle.body.split('\n').filter(Boolean).map((paragraph, i) => <p key={i}>{paragraph}</p>)}</div>{chronicle.suggestion && <div className="suggestion surface-subtle"><span className="eyebrow">If you feel curious</span><p>{chronicle.suggestion}</p></div>}</article>
}

function AdminPortal() {
  const [section, setSection] = useState<'templates'|'accounts'|'games'>('templates')
  return <><div className="page-heading"><div><p className="eyebrow">Application administration</p><h1>Admin portal</h1><p className="text-muted">Define the agents and initial beliefs copied into each new civilization, and manage account permissions.</p></div><Badge>Admin access required</Badge></div><div className="admin-tabs" role="tablist" aria-label="Admin sections">{(['templates','accounts','games'] as const).map(item => <button role="tab" aria-selected={section === item} key={item} className={section === item ? 'tab active' : 'tab'} onClick={() => setSection(item)}>{({templates:'Agent templates',accounts:'Accounts',games:'Games'})[item]}</button>)}</div>
    {section === 'templates' ? <AdminTemplates /> : section === 'accounts' ? <AdminAccounts /> : <AdminGames />}
  </>
}

function AdminTemplates() {
  const token = useToken(); const [templates, setTemplates] = useState<AgentTemplate[]>([]); const [loading, setLoading] = useState(true); const [busy, setBusy] = useState(false); const [error, setError] = useState<unknown>(null); const [notice, setNotice] = useState('')
  const [agentType, setAgentType] = useState<Agent['agent_type']>('priest'); const [displayName, setDisplayName] = useState(''); const [personality, setPersonality] = useState(''); const [activateOnCreate, setActivateOnCreate] = useState(false); const [beliefs, setBeliefs] = useState<InitialBeliefTemplate[]>([{ claim: '', initial_state: 'held' }])
  async function load() { setLoading(true); setError(null); try { setTemplates(await api.adminTemplates(token)) } catch (e) { setError(e) } finally { setLoading(false) } }
  useEffect(() => { void load() }, [token])
  function updateBelief(index: number, update: Partial<InitialBeliefTemplate>) { setBeliefs(previous => previous.map((belief, i) => i === index ? { ...belief, ...update } : belief)) }
  async function create(event: FormEvent) { event.preventDefault(); setBusy(true); setError(null); setNotice(''); try { const created = await api.createAdminTemplate(token, { agent_type: agentType, display_name: displayName.trim(), personality_prompt: personality.trim(), is_active: activateOnCreate, beliefs: beliefs.filter(b => b.claim.trim()).map(b => ({ claim: b.claim.trim(), initial_state: b.initial_state })) }); setDisplayName(''); setPersonality(''); setBeliefs([{ claim: '', initial_state: 'held' }]); setActivateOnCreate(false); setNotice(`${created.display_name} version ${created.version} was created.`); await load() } catch (e) { setError(e) } finally { setBusy(false) } }
  async function activate(template: AgentTemplate) { setBusy(true); setError(null); setNotice(''); try { await api.activateAdminTemplate(token, template.id); setNotice(`${template.display_name} version ${template.version} is now active for new civilizations.`); await load() } catch (e) { setError(e) } finally { setBusy(false) } }
  const grouped = (['priest','scientist','soldier','historian'] as const).map(type => [type, templates.filter(t => t.agent_type === type).sort((a,b) => b.version-a.version)] as const)
  return <div className="admin-workspace"><section className="admin-panel surface-card"><div className="section-heading"><div><span className="eyebrow">Versioned configuration</span><h2>Agent templates</h2></div><button className="button button-secondary" onClick={() => void load()} disabled={loading || busy}>{loading ? 'Loading…' : 'Refresh'}</button></div><p className="text-muted">A game copies the active agent template and its initial beliefs when it starts. Creating a version does not change existing civilizations.</p>{error !== null && <ErrorNotice error={error} />}{notice && <div className="notice notice-success" role="status">{notice}</div>}
    {loading ? <Loading label="Loading agent templates…" /> : templates.length === 0 ? <Empty>No templates yet. Create and activate a template for each agent before starting a world.</Empty> : <div className="template-groups">{grouped.map(([type, versions]) => <section className="template-group" key={type}><div className="section-heading"><h3>{agentNames[type]}</h3><span>{versions.length} {versions.length === 1 ? 'version' : 'versions'}</span></div>{versions.length ? versions.map(template => <article className="template-card surface-subtle" key={template.id}><div className="template-card-heading"><strong>{template.display_name}</strong><div><Badge state={template.is_active ? 'complete' : 'active'}>{template.is_active ? 'Active' : `Version ${template.version}`}</Badge>{!template.is_active && <button className="button button-quiet" disabled={busy} onClick={() => void activate(template)}>Activate</button>}</div></div><p className="template-prompt">{template.personality_prompt}</p><div className="template-beliefs"><strong>Initial beliefs ({template.beliefs.length})</strong>{template.beliefs.length ? <ul>{template.beliefs.map(belief => <li key={belief.id ?? `${template.id}-${belief.sort_order}`}>{belief.claim} <Badge state={belief.initial_state === 'held' ? 'complete' : 'active'}>{belief.initial_state}</Badge></li>)}</ul> : <p className="text-small text-muted">Starts without predefined beliefs.</p>}</div></article>) : <Empty>No template versions for this agent yet.</Empty>}</section>)}</div>}
    </section>
    <section className="admin-panel surface-card"><p className="eyebrow">Create immutable version</p><h2>New agent template</h2><p className="text-muted">This adds a version. Active versions are copied into new games; existing agents keep their current configuration.</p><form className="form-stack admin-form" onSubmit={create}><div className="admin-form-grid"><div><label htmlFor="template-agent">Agent</label><select id="template-agent" value={agentType} onChange={e => setAgentType(e.target.value as Agent['agent_type'])}><option value="priest">Priest</option><option value="scientist">Scientist</option><option value="soldier">Soldier</option><option value="historian">Historian</option></select></div><div><label htmlFor="template-name">Display name</label><input id="template-name" value={displayName} maxLength={120} required onChange={e => setDisplayName(e.target.value)} placeholder="The Keeper of Embers" /></div></div>
      <div><label htmlFor="personality-prompt">Personality and perspective</label><textarea id="personality-prompt" value={personality} maxLength={12000} required onChange={e => setPersonality(e.target.value)} placeholder="Describe how this agent observes, reasons, and speaks…" /></div>
      <fieldset className="belief-editor"><legend>Initial beliefs</legend><p className="text-small text-muted">These are copied to the agent when a new civilization starts. Claims can change as the game progresses.</p>{beliefs.map((belief, index) => <div className="belief-editor-row" key={index}><div><label htmlFor={`belief-claim-${index}`}>Belief {index + 1}</label><textarea id={`belief-claim-${index}`} maxLength={2000} value={belief.claim} onChange={e => updateBelief(index, { claim: e.target.value })} placeholder="What does this agent begin believing?" /></div><div><label htmlFor={`belief-state-${index}`}>Starting state</label><select id={`belief-state-${index}`} value={belief.initial_state} onChange={e => updateBelief(index, { initial_state: e.target.value as InitialBeliefTemplate['initial_state'] })}><option value="forming">Forming</option><option value="held">Held</option><option value="questioned">Questioned</option><option value="abandoned">Abandoned</option></select></div><button className="button button-quiet remove-belief" type="button" aria-label={`Remove belief ${index + 1}`} disabled={beliefs.length === 1} onClick={() => setBeliefs(previous => previous.filter((_, i) => i !== index))}>Remove</button></div>)}<button className="button button-secondary add-belief" type="button" disabled={beliefs.length >= 100} onClick={() => setBeliefs(previous => [...previous, { claim: '', initial_state: 'held' }])}>+ Add starting belief</button></fieldset>
      <label className="inline-check"><input type="checkbox" checked={activateOnCreate} onChange={e => setActivateOnCreate(e.target.checked)} /> Make this version active immediately</label><p className="text-small text-muted">If left unchecked, you can review this version and activate it from the list.</p>{error !== null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button" disabled={busy || !displayName.trim() || !personality.trim()}>{busy ? 'Creating version…' : 'Create template version'}</button></div>
    </form></section><p className="text-small text-muted">Admin operations require server-verified account permissions. Changes affect future games only.</p></div>
}

function AdminAccounts() {
  const token = useToken(); const [accounts, setAccounts] = useState<AdminAccount[]>([]); const [roles, setRoles] = useState<Record<string, AdminAccount['system_role']>>({}); const [loading, setLoading] = useState(true); const [saving, setSaving] = useState(''); const [error, setError] = useState<unknown>(null); const [notice, setNotice] = useState('')
  async function load() { setLoading(true); setError(null); try { const rows = await api.adminAccounts(token); setAccounts(rows); setRoles(Object.fromEntries(rows.map(account => [account.id, account.system_role]))) } catch (e) { setError(e) } finally { setLoading(false) } }
  useEffect(() => { void load() }, [token])
  async function save(account: AdminAccount) { setSaving(account.id); setError(null); setNotice(''); try { await api.updateAdminAccountRole(token, account.id, roles[account.id]); setNotice(`Account permission updated for ${account.id}.`); await load() } catch (e) { setError(e) } finally { setSaving('') } }
  return <section className="admin-panel surface-card"><div className="section-heading"><div><span className="eyebrow">Application permissions</span><h2>Accounts</h2></div><button className="button button-secondary" onClick={() => void load()} disabled={loading || Boolean(saving)}>{loading ? 'Loading…' : 'Refresh'}</button></div><p className="text-muted">Admin is an account permission and is separate from each game's fixed player role.</p>{error !== null && <ErrorNotice error={error} />}{notice && <div className="notice notice-success" role="status">{notice}</div>}{loading ? <Loading label="Loading accounts…" /> : accounts.length ? <div className="table-scroll"><table className="admin-table"><caption className="sr-only">Account roles and status</caption><thead><tr><th scope="col">Account</th><th scope="col">Created</th><th scope="col">Status</th><th scope="col">Permission</th><th scope="col"><span className="sr-only">Save changes</span></th></tr></thead><tbody>{accounts.map(account => <tr key={account.id}><th scope="row"><code title={account.id}>{account.id}</code></th><td>{formatDate(account.created_at)}</td><td><Badge state={account.status === 'active' ? 'complete' : 'attention'}>{account.status}</Badge></td><td><label className="sr-only" htmlFor={`account-role-${account.id}`}>Permission for account {account.id}</label><select id={`account-role-${account.id}`} value={roles[account.id] ?? account.system_role} disabled={account.status !== 'active' || saving === account.id} onChange={e => setRoles(previous => ({ ...previous, [account.id]: e.target.value as AdminAccount['system_role'] }))}><option value="user">User</option><option value="admin">Admin</option></select></td><td><button className="button button-secondary table-action" disabled={saving === account.id || roles[account.id] === account.system_role} onClick={() => void save(account)}>{saving === account.id ? 'Saving…' : 'Save'}</button></td></tr>)}</tbody></table></div> : <Empty>No accounts are available.</Empty>}<p className="text-small text-muted">Only active accounts can be changed. The server checks permission on every administrative request.</p></section>
}

function AdminGames() {
  const token = useToken(); const [games, setGames] = useState<Game[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState<unknown>(null)
  async function load() { setLoading(true); setError(null); try { setGames(await api.adminGames(token)) } catch (e) { setError(e) } finally { setLoading(false) } }
  useEffect(() => { void load() }, [token])
  return <section className="admin-panel surface-card"><div className="section-heading"><div><span className="eyebrow">Civilizations</span><h2>All games</h2></div><button className="button button-secondary" onClick={() => void load()} disabled={loading}>{loading ? 'Loading…' : 'Refresh'}</button></div><p className="text-muted">Review game role and lifecycle state. A game's player role remains fixed after creation.</p>{error !== null && <ErrorNotice error={error} />}{loading ? <Loading label="Loading games…" /> : games.length ? <div className="table-scroll"><table className="admin-table"><caption className="sr-only">All games</caption><thead><tr><th scope="col">Civilization</th><th scope="col">Player role</th><th scope="col">Status</th><th scope="col">Created</th></tr></thead><tbody>{games.map(game => <tr key={game.id}><th scope="row"><Link to={`/worlds/${game.id}`}>{game.name}</Link></th><td>{roleNames[game.player_role]}</td><td><Badge state={game.status === 'active' ? 'active' : game.status === 'archived' ? 'complete' : 'attention'}>{game.status.replaceAll('_', ' ')}</Badge></td><td>{formatDate(game.created_at)}</td></tr>)}</tbody></table></div> : <Empty>No games are available.</Empty>}<p className="text-small text-muted">Game editing is not exposed here; administrative access does not silently change a world's chosen role.</p></section>
}

function formatDate(value?: string) { if (!value) return 'recently'; const date = new Date(value); return Number.isNaN(date.getTime()) ? 'recently' : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(date) }
function replyText(value: unknown): string {
  if (typeof value === 'string') return value
  if (!value || typeof value !== 'object') return ''
  const record = value as Record<string, unknown>
  for (const key of ['reply', 'text', 'message', 'body', 'response']) {
    const result = replyText(record[key])
    if (result) return result
  }
  return ''
}

export default App
