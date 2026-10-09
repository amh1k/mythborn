import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import type { Session, User } from '@supabase/supabase-js'
import { supabase, supabaseConfigured } from './lib/supabase'

interface AuthValue { session: Session | null; user: User | null; loading: boolean; configured: boolean }
const AuthContext = createContext<AuthValue>({ session: null, user: null, loading: true, configured: supabaseConfigured })

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState(true)
  useEffect(() => {
    if (!supabase) { setLoading(false); return }
    void supabase.auth.getSession().then(({ data }) => { setSession(data.session); setLoading(false) })
    const { data } = supabase.auth.onAuthStateChange((_event, current) => { setSession(current); setLoading(false) })
    return () => data.subscription.unsubscribe()
  }, [])
  const value = useMemo(() => ({ session, user: session?.user ?? null, loading, configured: supabaseConfigured }), [session, loading])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
export const useAuth = () => useContext(AuthContext)
