import { useEffect } from 'react'

const entrances = [
  '.page-heading', '.world-card', '.world-banner', '.civilization-header', '.creation-plate', '.narrow-page',
  '.welcome-card', '.auth-story-copy', '.auth-card', '.round-card', '.discovery-card',
  '.overview-aside > *', '.agent-card', '.detail-panel', '.tradition-card',
  '.chronicle-card', '.chronicle-detail', '.progress-card', '.decision-card',
  '.attention-card', '.evidence-card', '.episode-photo', '.debate-contribution',
  '.notice', '.agent-reply', '.summary-box', '.admin-panel', '.template-card',
  '.belief-editor-row', '.belief-list > li', '.settings-row', '.empty-state',
  '.continuity-card', '.continuity-entry', '.continuity-history > li', '.config-card',
].join(',')

// Progressive enhancement: content remains visible if motion is unsupported.
// Observe new content once; polling and countdown ticks never replay entrances.
export function useAppMotion() {
  useEffect(() => {
    const root = document.getElementById('root')
    if (!root || !('IntersectionObserver' in window)) return
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
    const finePointer = window.matchMedia('(hover: hover) and (pointer: fine)')
    const seen = new WeakSet<Element>()
    const watched = new WeakSet<Element>()
    const animations = new Set<Animation>()
    const ease = 'cubic-bezier(.22, 1, .36, 1)'

    function enter(element: Element, delay = 0, distance = 16) {
      if (reduced.matches || document.hidden || !(element instanceof HTMLElement) || !element.animate) return
      // Nested animated panels should not multiply their movement.
      if ([...animations].some(animation => {
        const target = (animation.effect as KeyframeEffect | null)?.target
        return target instanceof Element && target !== element && target.contains(element)
      })) return
      const feedback = element.matches('.notice, .agent-reply, .summary-box')
      const animation = element.animate([
        { opacity: 0, translate: `0 ${feedback ? 6 : distance}px` },
        { opacity: 1, translate: '0 0' },
      ], { duration: feedback ? 280 : 620, delay: feedback ? 0 : delay, easing: ease, fill: 'backwards' })
      animations.add(animation)
      const remove = () => animations.delete(animation)
      animation.addEventListener('finish', remove, { once: true })
      animation.addEventListener('cancel', remove, { once: true })
    }

    const observer = new IntersectionObserver(entries => {
      entries.forEach(entry => {
        if (!entry.isIntersecting) return
        observer.unobserve(entry.target)
        if (seen.has(entry.target)) return
        seen.add(entry.target)
        const siblings = entry.target.parentElement?.children
        const index = siblings ? [...siblings].filter(child => child.matches(entrances)).indexOf(entry.target) : 0
        enter(entry.target, Math.min(Math.max(index, 0) * 65, 240))
      })
    }, { threshold: 0, rootMargin: '0px 0px -24px 0px' })

    function scan(element: Element) {
      const candidates = [...element.querySelectorAll(entrances)]
      if (element.matches(entrances)) candidates.unshift(element)
      candidates.forEach(candidate => {
        if (watched.has(candidate)) return
        watched.add(candidate)
        observer.observe(candidate)
      })
    }
    scan(root)
    const changes = new MutationObserver(records => {
      records.forEach(record => {
        record.removedNodes.forEach(node => {
          if (!(node instanceof Element)) return
          observer.unobserve(node)
          node.querySelectorAll(entrances).forEach(child => observer.unobserve(child))
        })
        record.addedNodes.forEach(node => { if (node instanceof Element) scan(node) })
        if (record.type === 'attributes' && record.target instanceof Element && record.target.getAttribute('data-agent-state') === 'received') {
          enter(record.target, 0, 4)
        }
      })
    })
    changes.observe(root, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-agent-state'] })

    let depthTarget: HTMLElement | null = null
    let frame = 0
    let x = 0
    let y = 0
    function resetDepth() {
      if (frame) cancelAnimationFrame(frame)
      frame = 0
      depthTarget?.style.removeProperty('--pointer-x')
      depthTarget?.style.removeProperty('--pointer-y')
      depthTarget = null
    }
    function pointerMove(event: PointerEvent) {
      if (reduced.matches || !finePointer.matches || event.pointerType === 'touch') return
      const target = event.target instanceof Element ? event.target.closest<HTMLElement>('[data-depth]') : null
      if (!target) { resetDepth(); return }
      if (target !== depthTarget) { resetDepth(); depthTarget = target }
      const rect = target.getBoundingClientRect()
      x = ((event.clientX - rect.left) / rect.width - .5) * 10
      y = ((event.clientY - rect.top) / rect.height - .5) * 8
      if (!frame) frame = requestAnimationFrame(() => {
        frame = 0
        depthTarget?.style.setProperty('--pointer-x', `${x.toFixed(2)}px`)
        depthTarget?.style.setProperty('--pointer-y', `${y.toFixed(2)}px`)
      })
    }
    function pointerOut(event: PointerEvent) {
      if (depthTarget && !(event.relatedTarget instanceof Node && depthTarget.contains(event.relatedTarget))) resetDepth()
    }
    function motionChanged() {
      resetDepth()
      if (reduced.matches) animations.forEach(animation => animation.cancel())
    }
    function focusChanged(event: FocusEvent) {
      // Keyboard navigation never waits for an entrance to finish.
      animations.forEach(animation => {
        const target = (animation.effect as KeyframeEffect | null)?.target
        if (target instanceof Element && event.target instanceof Node && target.contains(event.target)) animation.cancel()
      })
    }
    function disclosure(event: Event) {
      if (!(event.target instanceof HTMLDetailsElement) || !event.target.open) return
      [...event.target.children].filter(child => child.tagName !== 'SUMMARY').forEach(child => enter(child, 0, 6))
    }
    root.addEventListener('pointermove', pointerMove, { passive: true })
    root.addEventListener('pointerout', pointerOut, { passive: true })
    root.addEventListener('focusin', focusChanged)
    root.addEventListener('toggle', disclosure, true)
    reduced.addEventListener('change', motionChanged)
    finePointer.addEventListener('change', resetDepth)
    return () => {
      observer.disconnect()
      changes.disconnect()
      animations.forEach(animation => animation.cancel())
      resetDepth()
      root.removeEventListener('pointermove', pointerMove)
      root.removeEventListener('pointerout', pointerOut)
      root.removeEventListener('focusin', focusChanged)
      root.removeEventListener('toggle', disclosure, true)
      reduced.removeEventListener('change', motionChanged)
      finePointer.removeEventListener('change', resetDepth)
    }
  }, [])
}
