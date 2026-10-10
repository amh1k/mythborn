import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

const scenes = [
  { image: '/art/hero-citadel.webp', title: 'The city wakes', caption: 'Markets, towers, and a thousand possible stories.', position: 'center' },
  { image: '/art/hero-harbor.webp', title: 'Beyond the harbor', caption: 'New discoveries arrive with every tide.', position: 'center' },
  { image: '/art/hero-sanctuary.webp', title: 'Among the old trees', caption: 'What one calls a mystery, another calls sacred.', position: 'center' },
]

export function AtlasHero() {
  const [active, setActive] = useState(0)
  const [paused, setPaused] = useState(false)
  const [hovered, setHovered] = useState(false)
  const [focused, setFocused] = useState(false)
  const [visible, setVisible] = useState(() => !document.hidden)
  const [reducedMotion, setReducedMotion] = useState(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches)
  const [ready, setReady] = useState<boolean[]>([false, false, false])
  const playing = !paused && !hovered && !focused && visible && !reducedMotion

  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)')
    const motionChanged = () => setReducedMotion(media.matches)
    const visibilityChanged = () => setVisible(!document.hidden)
    media.addEventListener('change', motionChanged)
    document.addEventListener('visibilitychange', visibilityChanged)
    return () => {
      media.removeEventListener('change', motionChanged)
      document.removeEventListener('visibilitychange', visibilityChanged)
    }
  }, [])

  useEffect(() => {
    // Wait until every scene is loaded; slow connections never fade to a blank frame.
    if (!playing || !ready.every(Boolean)) return
    const timer = window.setTimeout(() => setActive(index => (index + 1) % scenes.length), 7500)
    return () => window.clearTimeout(timer)
  }, [active, playing, ready])

  function choose(index: number) {
    setActive((index + scenes.length) % scenes.length)
    setPaused(true)
  }

  return <section className="atlas-hero" aria-label="Explore Mythborn" aria-roledescription="carousel"
    onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}
    onFocusCapture={() => setFocused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocused(false) }}>
    <div className="atlas-scenes" data-playing={playing} aria-hidden="true">
      {scenes.map((scene, index) => <div key={scene.image} className={index === active ? 'atlas-scene is-active' : 'atlas-scene'}>
        <img src={scene.image} alt="" width="1536" height="1024" fetchPriority={index === 0 ? 'high' : 'low'} decoding="async" style={{ objectPosition: scene.position }}
          onLoad={() => setReady(previous => previous[index] ? previous : previous.map((value, i) => i === index ? true : value))} />
      </div>)}
    </div>
    <div className="atlas-hero-copy"><p className="eyebrow">The civilization atlas</p><h1>Your worlds.<br /><em>Their histories.</em></h1><p>Bring a discovery.<br />See what a civilization makes of it.</p><Link className="button" to="/worlds/new">Start a civilization <span aria-hidden="true">↗</span></Link></div>
    <div className="atlas-scene-info" aria-live="off"><span className="scene-number">0{active + 1} / 0{scenes.length}</span><div><strong>{scenes[active].title}</strong><p>{scenes[active].caption}</p></div></div>
    <div className="atlas-controls" role="group" aria-label="Hero slideshow controls">
      <button type="button" className="scene-arrow" aria-label="Previous scene" onClick={() => choose(active - 1)}><span aria-hidden="true">←</span></button>
      <div className="scene-dots">{scenes.map((scene, index) => <button key={scene.image} type="button" className={index === active ? 'scene-dot is-active' : 'scene-dot'} aria-label={`Show scene ${index + 1}: ${scene.title}`} aria-current={index === active ? 'true' : undefined} onClick={() => choose(index)}><span /></button>)}</div>
      <button type="button" className="scene-arrow" aria-label="Next scene" onClick={() => choose(active + 1)}><span aria-hidden="true">→</span></button>
      {!reducedMotion && <button type="button" className="scene-play" aria-label={paused ? 'Play slideshow' : 'Pause slideshow'} onClick={() => setPaused(value => !value)}>
        <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">{paused ? <path d="m4 2 9 6-9 6Z" /> : <><path d="M3 2h3v12H3zM10 2h3v12h-3z" /></>}</svg><span>{paused ? 'Play' : 'Pause'}</span>
      </button>}
    </div>
  </section>
}
