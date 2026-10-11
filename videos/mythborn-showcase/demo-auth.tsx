import type { ReactNode } from 'react'

export function AuthProvider({ children }: { children: ReactNode }) {
  return children
}

export function useAuth() {
  return {
    configured: true,
    loading: false,
    user: { id: 'local-demo', email: 'demo@mythborn.local' },
    session: { access_token: 'local-demo-token' },
  }
}
