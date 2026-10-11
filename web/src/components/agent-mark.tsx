import type { Agent } from '../types'

const signatures: Record<Agent['agent_type'], string> = {
  priest: 'P', scientist: 'Sc', soldier: 'S', historian: 'H',
}

// Printed initials identify the speakers; their names carry the meaning.
export function AgentMark({ type, large = false }: { type: Agent['agent_type']; large?: boolean }) {
  return <span className={`agent-avatar agent-letter ${type}${large ? ' large' : ''}`} aria-hidden="true">{signatures[type]}</span>
}
