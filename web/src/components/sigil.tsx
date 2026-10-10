import type { PlayerRole } from '../types'

export type SigilKind = PlayerRole | 'priest' | 'scientist' | 'soldier' | 'historian' | 'world' | 'camera'

// Small interface marks are drawn in SVG; the landscape is original raster artwork.
export function Sigil({ kind = 'world', className = '' }: { kind?: SigilKind; className?: string }) {
  return <svg className={className} viewBox="0 0 32 32" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === 'world' && <><path d="M3 27h26M5 24l7-14 5 9 4-6 6 11M8 24h15" /><circle cx="23" cy="7" r="3" /><path d="M12 10l2 8-4-2" /></>}
    {kind === 'observer' && <><path d="M3 16s5-8 13-8 13 8 13 8-5 8-13 8S3 16 3 16Z" /><circle cx="16" cy="16" r="4" /><path d="M16 3v2m0 22v2" /></>}
    {(kind === 'god' || kind === 'priest') && <><circle cx="16" cy="12" r="5" /><path d="M16 2v3m0 14v3M6 12H3m26 0h-3M9 5l2 2m10 10 2 2M23 5l-2 2M9 19l2-2M9 29l7-7 7 7M16 22v7" /></>}
    {kind === 'messenger' && <><path d="M6 7h20v15H16l-7 5v-5H6Z" /><path d="M11 12h10m-10 5h7" /></>}
    {kind === 'scientist' && <><circle cx="16" cy="16" r="3" /><ellipse cx="16" cy="16" rx="13" ry="6" transform="rotate(-45 16 16)" /><ellipse cx="16" cy="16" rx="13" ry="6" transform="rotate(45 16 16)" /></>}
    {kind === 'soldier' && <><path d="M16 3l10 4v10c0 6-10 12-10 12S6 23 6 17V7Z" /><path d="M16 9v13m-5-9h10" /></>}
    {kind === 'historian' && <><path d="M16 9c-3-3-8-4-13-3v20c5-1 10 0 13 3 3-3 8-4 13-3V6c-5-1-10 0-13 3Zm0 0v20M7 12l5 1m-5 4 5 1m8-5 5-1m-5 6 5-1" /></>}
    {kind === 'camera' && <><path d="M3 10h7l2-4h8l2 4h7v17H3Z" /><circle cx="16" cy="18" r="5" /><path d="M25 14h1" /></>}
  </svg>
}
