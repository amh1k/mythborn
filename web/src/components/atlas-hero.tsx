import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

const scenes = [
  { image: '/art/hero-citadel.webp', title: 'The city wakes', caption: 'Markets, towers, and a thousand possible stories.', position: 'center' },
  { image: '/art/hero-harbor.webp', title: 'Beyond the harbor', caption: 'New discoveries arrive with every tide.', position: 'center' },
  { image: '/art/hero-sanctuary.webp', title: 'Among the old trees', caption: 'What one calls a mystery, another calls sacred.', position: 'center' },
]

export function AtlasHero() {
  const [active, setActive] = useState(0)
  const [hovered, setHovered] = useState(false)
  const [focused, setFocused] = useState(false)
  const [visible, setVisible] = useState(() => !document.hidden)
  const [reducedMotion, setReducedMotion] = useState(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches)
  const [ready, setReady] = useState<boolean[]>([false, false, false])
  const playing = !hovered && !focused && visible && !reducedMotion

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

  return <section className="atlas-hero" aria-label="Explore Mythborn" data-depth
    onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}
    onFocusCapture={() => setFocused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocused(false) }}>
    <div className="atlas-scenes" data-playing={playing} aria-hidden="true">
      {scenes.map((scene, index) => <div key={scene.image} className={index === active ? 'atlas-scene is-active' : 'atlas-scene'}>
        <img src={scene.image} alt="" width="1536" height="1024" fetchPriority={index === 0 ? 'high' : 'low'} decoding="async" style={{ objectPosition: scene.position }}
          onLoad={() => setReady(previous => previous[index] ? previous : previous.map((value, i) => i === index ? true : value))} />
      </div>)}
    </div>
    <div className="atlas-hero-copy"><p className="eyebrow">The civilization atlas</p><h1>Your worlds.<br /><em>Their histories.</em></h1><p>Bring a discovery.<br />See what a civilization makes of it.</p><Link className="button" to="/worlds/new">Start a civilization <span aria-hidden="true">↗</span></Link></div>
    <div className="atlas-scene-info" aria-live="off"><span className="scene-number">0{active + 1} / 0{scenes.length}</span><div key={active} className="scene-caption"><strong>{scenes[active].title}</strong><p>{scenes[active].caption}</p></div></div>
  </section>
}
