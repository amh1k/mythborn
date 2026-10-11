import { useEffect, useRef } from 'react'

// One in-flight request; hidden/offline tabs wait, outages back off to 30s.
// Returning false ends polling until its identity changes.
export function usePolling(key: string, task: (signal: AbortSignal) => Promise<boolean>, interval = 2000, enabled = true) {
  const taskRef = useRef(task)
  taskRef.current = task
  useEffect(() => {
    if (!enabled) return
    const controller = new AbortController()
    let timer: number | undefined, inFlight = false, stopped = false, failures = 0
    const visible = () => !document.hidden && navigator.onLine
    const clear = () => { window.clearTimeout(timer); timer = undefined }
    async function run() {
      clear()
      if (controller.signal.aborted || stopped || inFlight || !visible()) return
      inFlight = true
      try { stopped = !await taskRef.current(controller.signal); failures = 0 }
      catch { failures = Math.min(failures + 1, 4) }
      finally {
        inFlight = false
        if (!controller.signal.aborted && !stopped && visible()) timer = window.setTimeout(() => void run(), Math.min(30000, interval * 2 ** failures))
      }
    }
    const resume = () => { clear(); if (visible()) void run() }
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('online', resume)
    window.addEventListener('offline', resume)
    void run()
    return () => { controller.abort(); clear(); document.removeEventListener('visibilitychange', resume); window.removeEventListener('online', resume); window.removeEventListener('offline', resume) }
  }, [key, interval, enabled])
}
