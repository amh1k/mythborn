import type { ReactNode } from 'react'
import { ApiError } from '../lib/api'

export function ErrorNotice({ error }: { error: unknown }) {
  return <div className="notice notice-error" role="alert"><span className="error-heading"><svg width="16" height="16" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M10 2.5 18 17H2L10 2.5Z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /><path d="M10 7v4" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" /><circle cx="10" cy="14" r=".8" fill="currentColor" /></svg>Error</span><div>{error instanceof Error ? error.message : 'Something went wrong. Please try again.'}</div>{error instanceof ApiError && error.requestId && <small className="error-reference">Reference: {error.requestId}</small>}</div>
}
export function Loading({ label = 'Gathering the latest records', detail, variant = 'inline' }: {
  label?: string
  detail?: string
  variant?: 'inline' | 'page' | 'section'
}) {
  return <div className={`loading loading-${variant}`} role="status" aria-live="polite" aria-atomic="true">
    <svg className="atlas-spinner" viewBox="0 0 64 64" fill="none" aria-hidden="true">
      <circle className="spinner-track" cx="32" cy="32" r="26" />
      <path className="spinner-points" d="M32 1v5m31 26h-5M32 63v-5M1 32h5" />
      <g className="spinner-orbit"><path d="M32 6a26 26 0 0 1 26 26" /><circle cx="32" cy="6" r="2" /></g>
      <g className="spinner-orbit spinner-orbit-inner"><path d="M32 15a17 17 0 0 1 17 17" /></g>
      <path className="spinner-center" d="m32 24 8 8-8 8-8-8Z" />
      <circle className="spinner-core" cx="32" cy="32" r="2" />
    </svg>
    <div className="loading-copy"><p className="loading-title">{label}</p>{detail && <p className="loading-detail">{detail}</p>}</div>
  </div>
}
export function Empty({ children }: { children: ReactNode }) { return <div className="empty-state surface-subtle">{children}</div> }
export function Badge({ children, state = 'active' }: { children: ReactNode; state?: 'active' | 'complete' | 'attention' }) { return <span className="status-pill" data-state={state}>{children}</span> }
