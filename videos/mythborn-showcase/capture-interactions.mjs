import { mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { spawn } from 'node:child_process'

const out = path.resolve(process.env.MYTHBORN_CAPTURE_DIR ?? 'capture-interactions')
const complete = process.env.MYTHBORN_CAPTURE_COMPLETE === '1'
const onlyAtlas = process.env.MYTHBORN_CAPTURE_ONLY === 'atlas'
const captures = onlyAtlas ? JSON.parse(await readFile(path.join(out, 'measurements.json'), 'utf8')) : {}
const chromePath = '/home/amh1k/.cache/puppeteer/chrome-headless-shell/linux-131.0.6778.204/chrome-headless-shell-linux64/chrome-headless-shell'
const chrome = spawn(chromePath, [
  '--no-sandbox', '--disable-dev-shm-usage', '--remote-debugging-port=9338',
  '--user-data-dir=/tmp/mythborn-video-capture-chrome', 'about:blank',
], { stdio: 'ignore' })
let ws
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms))

async function waitFor(url) {
  for (let i = 0; i < 120; i++) {
    try { const response = await fetch(url); if (response.ok) return response }
    catch {}
    await sleep(100)
  }
  throw new Error(`Local service did not become ready: ${url}`)
}

try {
  await mkdir(out, { recursive: true })
  await waitFor('http://127.0.0.1:9338/json/version')
  const target = await (await fetch('http://127.0.0.1:9338/json/new?about:blank', { method: 'PUT' })).json()
  ws = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise(resolve => ws.addEventListener('open', resolve, { once: true }))
  let nextId = 0
  const pending = new Map()
  ws.addEventListener('message', event => {
    const packet = JSON.parse(event.data)
    if (!packet.id) return
    const job = pending.get(packet.id)
    pending.delete(packet.id)
    if (packet.error) job?.reject(new Error(JSON.stringify(packet.error)))
    else job?.resolve(packet.result)
  })
  const rpc = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++nextId
    pending.set(id, { resolve, reject })
    ws.send(JSON.stringify({ id, method, params }))
  })
  const evaluate = async expression => {
    const result = await rpc('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description ?? result.exceptionDetails.text)
    return result.result.value
  }
  const navigate = async (url, selector) => {
    await rpc('Page.navigate', { url })
    for (let i = 0; i < 120; i++) {
      await sleep(100)
      if (await evaluate(`Boolean(document.querySelector(${JSON.stringify(selector)}))`)) {
        await evaluate('window.scrollTo(0,0)')
        await sleep(500)
        return
      }
    }
    throw new Error(`Page did not render ${selector} at ${url}`)
  }
  const capture = async name => {
    await evaluate(`(async()=>{
      await document.fonts.ready;
      document.querySelectorAll('img[loading="lazy"]').forEach(image=>image.loading='eager');
      await new Promise(resolve=>requestAnimationFrame(resolve));
      await Promise.all([...document.images].map(image => image.decode().catch(()=>{})));
      await Promise.all(document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(()=>{})));
      await new Promise(resolve => requestAnimationFrame(()=>requestAnimationFrame(resolve)));
    })()`)
    await sleep(150)
    console.log(JSON.stringify({ name, url: await evaluate('location.href'), scrollY: await evaluate('scrollY'), tabCount: await evaluate("document.querySelectorAll('.world-tabs .tab').length"), body: (await evaluate('document.body.innerText')).slice(0, 180) }))
    const metrics = await rpc('Page.getLayoutMetrics')
    const scrollY = await evaluate('window.scrollY')
    const height = complete ? Math.max(1080, Math.ceil(metrics.cssContentSize.height - scrollY)) : 1080
    captures[name] = await evaluate(`(()=>{
      const rect = element => { if(!element) return null; const r=element.getBoundingClientRect(); return {x:r.x,y:r.y,width:r.width,height:r.height,cx:r.x+r.width/2,cy:r.y+r.height/2}; };
      const button = [...document.querySelectorAll('button')].find(element => element.textContent.trim()==='Begin discovery');
      return {scrollY, controls:{messenger:rect(document.querySelector('input[value="messenger"]')),photo:rect(document.querySelector('.observation-preview')),begin:rect(button)}, regions:[...document.querySelectorAll('h1,h2,h3,.debate-contribution,.chronicle-detail,.chronicle-card,.tradition-card,.belief-list')].map(element=>({text:element.textContent.trim().slice(0,110),...rect(element)})), images:[...document.images].map(image=>({src:image.getAttribute('src'),complete:image.complete,naturalWidth:image.naturalWidth,naturalHeight:image.naturalHeight}))};
    })()`)
    captures[name].width = 1920
    captures[name].height = height
    if(captures[name].images.some(image => !image.complete || !image.naturalWidth)) throw new Error(`An image did not load in ${name}`)
    const shot = await rpc('Page.captureScreenshot', { format: 'png', fromSurface: true, ...(complete ? { captureBeyondViewport: true, clip: { x: 0, y: scrollY, width: 1920, height, scale: 1 } } : {}) })
    await writeFile(path.join(out, `${name}.png`), Buffer.from(shot.data, 'base64'))
    console.log(`${name}.png`)
  }
  const clickTab = async index => {
    const clicked = await evaluate(`(()=>{const tabs=[...document.querySelectorAll('.world-tabs .tab')];if(!tabs[${index}])return false;tabs[${index}].click();return true})()`)
    if (!clicked) throw new Error(`Could not click civilization tab ${index}`)
    await sleep(700)
  }
  const scrollTo = async (selector, block = 'center') => {
    await evaluate(`document.querySelector(${JSON.stringify(selector)})?.scrollIntoView({block:${JSON.stringify(block)},behavior:'instant'})`)
    await sleep(350)
  }

  await rpc('Page.enable')
  await rpc('Runtime.enable')
  await rpc('Emulation.setDeviceMetricsOverride', { width: 1920, height: 1080, deviceScaleFactor: 1, mobile: false })

  if (complete) {
    await navigate('http://127.0.0.1:5187/worlds', '.world-card')
    // Preserve the approved harbor shot through the app's own carousel state.
    for (let i = 0; i < 120; i++) {
      if (await evaluate(`document.querySelector('.atlas-scene.is-active img')?.getAttribute('src') === '/art/hero-harbor.webp'`)) break
      await sleep(100)
    }
    if (!(await evaluate(`document.querySelector('.atlas-scene.is-active img')?.getAttribute('src') === '/art/hero-harbor.webp'`))) throw new Error('The harbor scene did not become ready')
    await rpc('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 1000, y: 300 })
    await capture('01-atlas')
    if (!onlyAtlas) {
      await navigate('http://127.0.0.1:5187/worlds/demo-valley-of-first-rain', '.world-tabs')
      await capture('03-world')
    }
  }

  if (!onlyAtlas) {
  await navigate('http://127.0.0.1:5187/worlds/new', '.role-picker')
  await evaluate(`(()=>{const name=document.querySelector('#world-name');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(name,'Valley of First Rain');name.dispatchEvent(new Event('input',{bubbles:true}));const role=document.querySelector('input[value="messenger"]');role.click()})()`)
  await capture('02-founding')

  await navigate('http://127.0.0.1:5187/worlds/demo-valley-of-first-rain', '.world-tabs')
  await evaluate(`(async()=>{const blob=await(await fetch('/art/world-watchtower.webp')).blob();const input=document.querySelector('input[type=file]');const transfer=new DataTransfer();transfer.items.add(new File([blob],'world-watchtower.webp',{type:'image/webp'}));input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}))})()`)
  await sleep(700)
  await scrollTo('.observation-preview')
  await capture('03-discovery-photo')

  await clickTab(2)
  await evaluate(`document.querySelector('.agent-card')?.click()`)
  for (let i = 0; i < 40 && !(await evaluate(`Boolean(document.querySelector('.belief-list'))`)); i++) await sleep(100)
  await capture('06-beliefs')

  await navigate('http://127.0.0.1:5187/worlds/demo-valley-of-first-rain', '.world-tabs')
  await clickTab(3)
  await sleep(300)
  await scrollTo('.tradition-card')
  await capture('08-culture')

  await navigate('http://127.0.0.1:5187/worlds/demo-valley-of-first-rain', '.world-tabs')
  await clickTab(4)
  await sleep(300)
  await capture('09-history')

  await navigate('http://127.0.0.1:5187/worlds/demo-valley-of-first-rain', '.world-tabs')
  await scrollTo('.continuity-panel')
  await capture('07-mysteries')

  await navigate('http://127.0.0.1:5187/episodes/demo-live-debate', '.live-debate')
  await scrollTo('.live-debate', 'start')
  await capture('05-agent-debate')

  await navigate('http://127.0.0.1:5187/episodes/demo-first-discovery', '.chronicle-detail')
  await scrollTo('.chronicle-detail', 'start')
  await capture('04-chronicle')
  }
  await writeFile(path.join(out, 'measurements.json'), JSON.stringify(captures, null, 2) + '\n')
  console.log('Captured real Mythborn UI views with the local demo API.')
} finally {
  ws?.close()
  try { process.kill(chrome.pid, 'SIGTERM') } catch {}
}
