import { createServer } from 'node:http'

const worldId = 'demo-valley-of-first-rain'
const episodeId = 'demo-first-discovery'
const liveEpisodeId = 'demo-live-debate'
const created = '2026-09-14T12:00:00.000Z'
const agents = [
  { id: 'priest', agent_type: 'priest', display_name: 'The Priest', role_summary: 'Reads signs, sacred patterns, and obligations.' },
  { id: 'scientist', agent_type: 'scientist', display_name: 'The Scientist', role_summary: 'Tests claims against evidence and uncertainty.' },
  { id: 'soldier', agent_type: 'soldier', display_name: 'The Soldier', role_summary: 'Sees threats, borders, and practical consequences.' },
  { id: 'historian', agent_type: 'historian', display_name: 'The Historian', role_summary: 'Keeps the record, including unresolved disagreement.' },
]
const chronicle = {
  id: 'demo-chronicle', round_id: episodeId, title: 'The Watchtower Above the River', outcome: 'majority',
  verdict: 'The tower watched the river, though no one agrees what warning it gave.',
  body: 'The council agreed the tower was built to watch the valley, but not what its keepers watched for. The Priest believed its light carried a promise to the river. The Scientist saw a place to measure floods. The Soldier argued it guarded the road into the valley. The Historian kept all three readings beside the shared fact: generations returned to the tower when the river changed.',
  created_at: created, suggestion: 'Look for a second tower where the river narrows upstream.', observation_id: 'demo-observation',
}
const world = {
  id: worldId, name: 'Valley of First Rain', player_role: 'messenger', status: 'active',
  review_photo_description: true, created_at: created,
  last_chronicle: { id: chronicle.id, round_id: episodeId, title: chronicle.title, verdict: chronicle.verdict, created_at: created },
  latest_episode: { id: episodeId, game_id: worldId, sequence_number: 1, kind: 'discovery', status: 'complete', stage: 'complete', created_at: created, chronicle },
}
const episode = {
  id: episodeId, game_id: worldId, sequence_number: 1, kind: 'discovery', status: 'complete', stage: 'complete', created_at: created,
  observation_id: 'demo-observation', description: 'A stone watchtower stands on a ridge above a river valley. A narrow path climbs toward its gate as birds circle over the valley.',
  description_status: 'accepted', photo_url: 'http://127.0.0.1:5173/art/world-watchtower.webp', chronicle,
  changes: [
    { id: 'change-belief', kind: 'belief', name: 'The tower warns the valley', before_state: 'forming', after_state: 'held', after: 'A light from the tower tells the village when the river is rising.', reason: 'The Priest and Historian agreed the tower connected the valley to a shared warning.' },
    { id: 'change-mystery', kind: 'mystery', name: 'Who first lit the tower?', before_state: 'forming', after_state: 'open', after: 'The first keeper and the tower’s earliest warning remain unknown.', reason: 'The agents found no evidence that explains how the watch began.' },
    { id: 'change-ritual', kind: 'culture_influence', name: 'The evening watch', before_state: 'forming', after_state: 'adopted', after: 'When clouds gather upriver, a watchkeeper climbs the tower at dusk and reports the waterline.', reason: 'The village kept the watch even while the tower’s original purpose remained disputed.' },
  ],
}
const liveEpisode = { ...episode, id: liveEpisodeId, status: 'running', stage: 'debating', chronicle: undefined, changes: undefined }
const debateEntries = [
  { id: 'react-priest', agent_id: 'priest', phase: 'reaction', text: 'The tower’s light was a promise that the river would not take the valley unseen.', reasoning: 'A watch kept above the water can become an offering as much as a warning.' },
  { id: 'react-scientist', agent_id: 'scientist', phase: 'reaction', text: 'Its height makes it a useful place to observe flood levels.', reasoning: 'The tower overlooks the river bend and its floodplain.' },
  { id: 'react-soldier', agent_id: 'soldier', phase: 'reaction', text: 'It guarded the path into the valley. A warning would protect the road as well.', reasoning: 'The tower faces the narrow approach between the hills.' },
  { id: 'react-historian', agent_id: 'historian', phase: 'reaction', text: 'The path is worn. People kept returning after the first watch was over.', reasoning: 'The tower became part of the valley’s shared memory.' },
  { id: 'rebuttal-priest', agent_id: 'priest', phase: 'rebuttal', text: 'A flood measure can become a promise when the whole village depends on it.', reasoning: 'The practical use and sacred meaning may have grown together.' },
  { id: 'rebuttal-scientist', agent_id: 'scientist', phase: 'rebuttal', text: 'Then record the waterline and leave the keeper’s purpose open.', reasoning: 'The tower shows where people watched; it cannot tell us what each keeper believed.' },
]
const mysteries = [
  { id: 'mystery-first-light', game_id: worldId, title: 'Who first lit the tower?', question: 'What warning brought the first keeper up the hill?', state: 'open', version: 1, origin_round_id: episodeId, latest_round_id: episodeId, evidence_summary: 'The tower looks over both the river bend and the road, leaving its first purpose uncertain.', next_observation: 'Look for another watchtower where the river narrows upstream.', explanations: [{ key: 'flood', claim: 'The first light warned of rising water.', agent_ids: ['scientist', 'historian'] }, { key: 'vow', claim: 'The first light answered a promise to the river.', agent_ids: ['priest'] }] },
  { id: 'mystery-upper-road', game_id: worldId, title: 'The road beyond the ridge', question: 'Who used the path that leaves the valley?', state: 'open', version: 1, origin_round_id: episodeId, latest_round_id: episodeId, evidence_summary: 'The path continues beyond the tower, but the valley has no record of where it leads.', next_observation: 'Follow the old road past the upper bend.', explanations: [{ key: 'trade', claim: 'Travelers brought news from another settlement.', agent_ids: ['historian'] }, { key: 'patrol', claim: 'The path was used to watch the border.', agent_ids: ['soldier'] }] },
]
const traditions = [{ id: 'tradition-evening-watch', version: 1, type: 'ritual', title: 'The evening watch', description: 'When clouds gather upriver, a watchkeeper climbs the tower at dusk and reports the waterline.', state: 'adopted', supporters: ['priest', 'historian'] }]
const beliefs = {
  priest: [{ id: 'belief-priest', claim: 'The tower’s light is a promise to the river.', state: 'held', version: 1, revisions: [{ id: 'revision-priest', change_type: 'adopted', reason: 'The watch returned each evening as the water changed.' }] }],
  scientist: [{ id: 'belief-scientist', claim: 'The tower helps the village read floodwater.', state: 'held', version: 1, revisions: [{ id: 'revision-scientist', change_type: 'adopted', reason: 'The tower overlooks the river bend and floodplain.' }] }],
  soldier: [{ id: 'belief-soldier', claim: 'The tower guards the road into the valley.', state: 'questioned', version: 1, revisions: [{ id: 'revision-soldier', change_type: 'questioned', reason: 'The path is open to travelers as well as patrols.' }] }],
  historian: [{ id: 'belief-historian', claim: 'A warning from the tower belongs to the whole valley.', state: 'held', version: 1, revisions: [{ id: 'revision-historian', change_type: 'adopted', reason: 'The watch became a practice shared across generations.' }] }],
}
const history = [{ ...chronicle, round_kind: 'discovery' }]

function payload(path) {
  if (path === '/worlds') return { worlds: [world] }
  if (path === `/worlds/${worldId}`) return { world }
  if (path === `/worlds/${worldId}/agents`) return { agents }
  if (new RegExp(`^/worlds/${worldId}/agents/[^/]+/relationships$`).test(path)) return { relationships: [] }
  if (path === `/worlds/${worldId}/history`) return { history }
  if (path === `/worlds/${worldId}/traditions`) return { traditions }
  if (path.startsWith(`/worlds/${worldId}/mysteries/`)) return { mystery: mysteries[0], revisions: [] }
  if (path === `/worlds/${worldId}/mysteries`) return { mysteries }
  if (path === `/worlds/${worldId}/episodes`) return { episodes: [world.latest_episode] }
  const beliefMatch = path.match(new RegExp(`^/worlds/${worldId}/agents/([^/]+)/beliefs$`))
  if (beliefMatch) return { beliefs: beliefs[beliefMatch[1]] ?? [] }
  if (path === `/episodes/${liveEpisodeId}`) return { episode: liveEpisode, observation: { id: 'demo-observation', description: episode.description, description_status: 'accepted', photo_url: episode.photo_url } }
  if (path === `/episodes/${liveEpisodeId}/debate`) return { state: 'live', player_role: 'messenger', preview: { game_id: worldId, round_id: liveEpisodeId, attempt_id: 'demo-attempt', phase: 'rebuttal', agents: agents.map(({ id, agent_type, display_name }) => ({ agent_id: id, agent_type, display_name })), entries: debateEntries, retries: [] } }
  if (path === `/episodes/${episodeId}`) return { episode, observation: { id: 'demo-observation', description: episode.description, description_status: 'accepted', photo_url: episode.photo_url }, chronicle, changes: episode.changes }
  if (path === `/episodes/${episodeId}/debate`) return { state: 'finished', player_role: 'messenger', preview: { game_id: worldId, round_id: episodeId, attempt_id: 'demo-attempt', phase: 'writing', agents: agents.map(({ id, agent_type, display_name }) => ({ agent_id: id, agent_type, display_name })), entries: debateEntries, retries: [] } }
  if (new RegExp(`^/episodes/${episodeId}/agents/[^/]+/messages$`).test(path)) return { summary: null, summary_version: 0 }
  if (path === '/observations/demo-observation/photo-url') return { url: episode.photo_url, expires_at: '2030-01-01T00:00:00.000Z' }
  return {}
}

createServer((req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Access-Control-Allow-Headers', 'authorization,content-type,idempotency-key')
  res.setHeader('Access-Control-Allow-Methods', 'GET,POST,PATCH,DELETE,OPTIONS')
  if (req.method === 'OPTIONS') { res.writeHead(204).end(); return }
  const path = new URL(req.url ?? '/', 'http://localhost').pathname.replace('/api/v1', '')
  res.writeHead(200, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify({ data: payload(path) }))
}).listen(6080, '127.0.0.1', () => console.log('Local demo API listening on http://127.0.0.1:6080'))
