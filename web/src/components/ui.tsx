import type { ReactNode } from 'react'
import { ApiError } from '../lib/api'

export function ErrorNotice({ error }: { error: unknown }) {
  return <div className="notice notice-error" role="alert">{error instanceof Error ? error.message : 'Something went wrong. Please try again.'}{error instanceof ApiError && error.requestId && <small className="error-reference">Reference: {error.requestId}</small>}</div>
}
export function Loading({ label = 'Loading…' }: { label?: string }) { return <p className="loading" role="status">{label}</p> }
export function Empty({ children }: { children: ReactNode }) { return <div className="empty-state surface-subtle">{children}</div> }
export function Badge({ children, state = 'active' }: { children: ReactNode; state?: 'active' | 'complete' | 'attention' }) { return <span className="status-pill" data-state={state}>{children}</span> }
