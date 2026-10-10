import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

type Theme = 'light' | 'dark'
const storageKey = 'mythborn-theme'
const ThemeContext = createContext<{ theme: Theme; toggle: () => void } | null>(null)

function savedTheme(): Theme | null {
  try {
    const value = localStorage.getItem(storageKey)
    return value === 'light' || value === 'dark' ? value : null
  } catch { return null }
}

function systemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#101820' : '#f3f0e9')
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<Theme | null>(savedTheme)
  const [theme, setTheme] = useState<Theme>(() => savedTheme() ?? systemTheme())

  useEffect(() => { applyTheme(theme) }, [theme])

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const systemChanged = () => { if (preference === null) setTheme(systemTheme()) }
    const storageChanged = (event: StorageEvent) => {
      if (event.key !== storageKey && event.key !== null) return
      const saved = savedTheme()
      setPreference(saved)
      setTheme(saved ?? systemTheme())
    }
    media.addEventListener('change', systemChanged)
    window.addEventListener('storage', storageChanged)
    return () => {
      media.removeEventListener('change', systemChanged)
      window.removeEventListener('storage', storageChanged)
    }
  }, [preference])

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    setPreference(next)
    setTheme(next)
    try { localStorage.setItem(storageKey, next) } catch { /* The toggle still works if storage is unavailable. */ }
  }

  return <ThemeContext.Provider value={{ theme, toggle }}>{children}</ThemeContext.Provider>
}

export function ThemeToggle() {
  const value = useContext(ThemeContext)
  if (!value) throw new Error('ThemeToggle requires ThemeProvider')
  const { theme, toggle } = value
  const label = theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'
  return <button type="button" className="button button-secondary theme-toggle" onClick={toggle} aria-label={label} title={label}>
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {theme === 'dark' ? <><circle cx="12" cy="12" r="4" /><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M19 5l-1.5 1.5m-11 11L5 19" /></> : <path d="M20.5 14a8.5 8.5 0 0 1-10.5-10.5A8.5 8.5 0 1 0 20.5 14Z" />}
    </svg><span>{theme === 'dark' ? 'Light' : 'Dark'}</span>
  </button>
}
