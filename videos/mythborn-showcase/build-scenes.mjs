import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'

const root = path.dirname(new URL(import.meta.url).pathname)
const framesDir = path.join(root, 'compositions/frames')
const fonts = path.resolve(root, '../../web/public/fonts')
const measurements = JSON.parse(await readFile(path.join(root, 'capture-polish/measurements.json'), 'utf8'))
const control = (capture, key, at, press = true) => {
  const rect = measurements[capture]?.controls[key]
  if (!rect?.width || !rect?.height) throw new Error(`Missing measured control: ${capture}/${key}`)
  return { x: rect.cx, y: rect.cy, at, press, control: key, bounds: rect }
}
await mkdir(path.join(root, 'assets/fonts'), { recursive: true })
await mkdir(framesDir, { recursive: true })
for (const name of ['almendra-regular.ttf', 'almendra-bold.ttf', 'uncial-antiqua.ttf', 'dm-sans-regular.ttf', 'dm-sans-semibold.ttf', 'Almendra-LICENSE.txt', 'Uncial-Antiqua-LICENSE.txt', 'DM-Sans-LICENSE.txt']) {
  await copyFile(path.join(fonts, name), path.join(root, 'assets/fonts', name))
}
await copyFile(path.resolve(root, '../../web/public/art/world-watchtower.webp'), path.join(root, 'assets/watchtower.webp'))

// Times below are in the film clock. Native WAAPI is document-global, including
// animations created by a sub-composition. Each animation receives scene.start
// exactly once; never schedule later scenes at zero.
const scenes = [
  {
    id: 'f01-atlas', duration: 8, handoff: 2.5, name: 'Go outside. Bring back a story.',
    kicker: 'MYTHBORN', lines: ['Go outside.', 'Bring back a story.'],
    body: 'Turn a short walk into your civilization’s next discovery.',
    image: 'capture-polish/01-atlas.png', alt: 'The real Mythborn civilization atlas',
    poses: [[0, 2.4, -2160, -372], [2.5, 2.4, -2160, -372], [4.2, 1.5, -480, -180], [6, 1.35, -270, -660], [8, 1.4, -285, -845]],
  },
  {
    id: 'f02-founding', duration: 9, handoff: 2.5, name: 'Choose a name and a perspective',
    kicker: 'THE FOUNDING', lines: ['A name.', 'A point of view.'],
    body: 'Choose who speaks. Enter as the Messenger.',
    image: 'capture-polish/02-founding.png', alt: 'Real founding form with the local demo name and Messenger role',
    poses: [[0, 1.16, -170, -70], [2.5, 1.16, -170, -70], [4.2, 1.6, -1430, -250], [6.6, 1.45, -1270, -500], [9, 1.47, -1280, -520]],
    cursor: { from: [1220, 560], clicks: [control('02-founding', 'messenger', 6.8)] },
  },
  {
    id: 'f03-world', duration: 9, handoff: 2.4, name: 'A civilization takes shape',
    kicker: 'VALLEY OF FIRST RAIN', lines: ['A civilization', 'takes shape.'],
    body: 'A place for discoveries, beliefs, and a shared history.',
    image: 'capture-polish/03-world.png', alt: 'Real Valley of First Rain overview with banner, title and council',
    poses: [[0, 2.25, -1050, -490], [2.4, 2.25, -1050, -490], [4.2, 1.18, -220, -175], [6.8, 1.2, -240, -540], [9, 1.22, -250, -555]],
  },
  {
    id: 'f04-photo', duration: 10, handoff: 2.5, photo: true, name: 'Take a walk. Take a photo.',
    kicker: 'YOUR NEXT WALK', lines: ['Take a walk.', 'Take a photo.'],
    body: 'Notice something. Give your agents a discovery.',
    image: 'capture-polish/03-discovery-photo.png', alt: 'Real discovery form with the bundled watchtower photograph selected',
    poses: [[0, 2.2, -965, -630], [2.5, 2.2, -965, -630], [4.1, 1.1, -220, -50], [5.4, 1.42, -380, -310], [7.4, 1.2, -270, -200], [10, 1.22, -275, -215]],
    cursor: { from: [940, 460], clicks: [control('03-discovery-photo', 'photo', 5, false), control('03-discovery-photo', 'begin', 8.2)] },
  },
  {
    id: 'f05-readings', duration: 10, handoff: 2.15, split: true, roles: true, name: 'One tower, four readings',
    kicker: 'THE COUNCIL', lines: ['One tower.', 'Four readings.'],
    body: 'The same evidence. Different meanings.',
    image: 'capture-interactions/05-agent-debate.png', alt: 'Actual first-reaction UI populated by the local demo fixture',
    poses: [[0, 1.15, -320, -300], [2.15, 1.15, -320, -300], [4.3, 1.18, -325, -355], [6.7, 1.2, -335, -390], [10, 1.22, -340, -435]],
    annotations: [
      { at: 2.85, duration: 1.65, kicker: 'THE PRIEST', lines: ['A promise.'] },
      { at: 4.55, duration: 1.65, kicker: 'THE SCIENTIST', lines: ['A flood', 'measure.'] },
      { at: 6.25, duration: 1.65, kicker: 'THE SOLDIER', lines: ['A guarded', 'road.'] },
      { at: 7.95, duration: 2.05, kicker: 'THE HISTORIAN', lines: ['A remembered', 'path.'] },
    ],
  },
  {
    id: 'f06-rebuttals', duration: 12, handoff: 2.5, split: true, surfaceWidth: 1220, name: 'They challenge each other',
    kicker: 'THE REBUTTALS', lines: ['Then they', 'challenge each other.'],
    body: 'Each agent responds to the first interpretations.', smallTitle: true,
    image: 'capture-interactions/05-agent-debate.png', alt: 'Captured real demo state with two of four rebuttals received',
    poses: [[0, 1.4, -365, -10], [2.5, 1.4, -365, -10], [5.3, 1.42, -385, -65], [8.3, 1.4, -365, -1210], [12, 1.42, -375, -1400]],
    annotations: [
      { at: 3.1, duration: 3.1, kicker: 'RESPONSES ARRIVE', lines: ['They answer', 'each other.'], body: 'Two of four rebuttals have arrived in this captured demo.' },
      { at: 6.35, duration: 5.65, kicker: 'DISAGREEMENT STAYS VISIBLE', lines: ['The debate', 'is part of', 'the story.'] },
    ],
  },
  {
    id: 'f07-chronicle', duration: 11, handoff: 2.7, split: true, name: 'Keep the story and the disagreement',
    kicker: 'THE HISTORIAN’S CHRONICLE', lines: ['Keep the story.', 'Keep the disagreement.'],
    body: 'The shared record carries both forward.', smallTitle: true,
    image: 'capture-interactions/04-chronicle.png', alt: 'The actual local demo chronicle, belief, open question, and adopted ritual',
    poses: [[0, 1.17, -302, 50], [2.7, 1.17, -302, 50], [5.4, 1.19, -310, -20], [8.2, 1.17, -300, -280], [11, 1.15, -290, -310]],
    annotations: [
      { at: 3.35, duration: 2.1, kicker: 'THE SHARED FACT', lines: ['The tower', 'watched', 'the valley.'] },
      { at: 5.5, duration: 2.5, kicker: 'THE OPEN QUESTION', lines: ['What were', 'its keepers', 'watching for?'] },
      { at: 8.05, duration: 2.95, kicker: 'YOUR NEXT DISCOVERY', lines: ['Go outside', 'again.'], body: 'Follow the hint, or bring any photo.' },
    ],
  },
  {
    id: 'f08-belief', duration: 4, montage: true, split: true, name: 'Belief',
    kicker: 'WHAT THE WORLD KEEPS', lines: ['Belief.'], body: 'A promise to the river.',
    image: 'capture-interactions/06-beliefs.png', alt: 'Real Priest panel showing the held belief about the tower’s light',
    poses: [[0, 1.36, -1140, -100], [1.8, 1.42, -1190, -140], [4, 1.47, -1230, -190]],
  },
  {
    id: 'f08-mystery', duration: 4, montage: true, split: true, name: 'Mystery',
    kicker: 'WHAT REMAINS OPEN', lines: ['Mystery.'], body: 'Who first lit the tower?',
    image: 'capture-interactions/07-mysteries.png', alt: 'Real open mystery asking who first lit the tower',
    poses: [[0, 1.17, -360, -260], [1.8, 1.2, -375, -305], [4, 1.22, -385, -350]],
  },
  {
    id: 'f08-culture', duration: 4, montage: true, split: true, name: 'Culture',
    kicker: 'WHAT BECOMES A PRACTICE', lines: ['Culture.'], body: 'The Evening Watch becomes a ritual.',
    image: 'capture-interactions/08-culture.png', alt: 'Real adopted Evening Watch ritual in the culture tab',
    poses: [[0, 1.4, -390, -440], [1.8, 1.5, -425, -470], [4, 1.58, -450, -500]],
  },
  {
    id: 'f09-history-end', duration: 9, end: true, split: true, name: 'A history that keeps growing',
    kicker: 'A HISTORY THAT KEEPS GROWING', lines: ['The ritual', 'remains.'], body: 'The tower’s original purpose stays disputed.',
    image: 'capture-interactions/09-history.png', alt: 'The discovery preserved in the real civilization History tab',
    poses: [[0, .9, -255, -300], [2, .92, -275, -325], [4, .93, -290, -345], [9, .94, -300, -360]],
  },
]
let filmTime = 0
for (const scene of scenes) {
  scene.image = scene.image.replace('capture-interactions/', 'capture-polish/')
  scene.sourceSize = measurements[path.basename(scene.image, '.png')]
  if (!scene.sourceSize?.height) throw new Error(`Missing capture dimensions: ${scene.image}`)
  scene.start = filmTime
  scene.overlap = scene.start ? .45 : 0
  scene.clipStart = scene.start - scene.overlap
  scene.clipDuration = scene.duration + scene.overlap
  filmTime += scene.duration
}

const esc = value => String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;')
const words = (text, kind = 'title-word') => String(text).split(/\s+/).map(word => `<span class="word-mask" data-layout-allow-overflow><span class="${kind}" data-layout-allow-overflow>${esc(word)}</span></span>`).join(' ')
const lines = values => values.map((line, i) => `<span class="type-line${i === values.length - 1 ? ' accent' : ''}">${words(line)}</span>`).join('')
const cursorMarkup = scene => scene.cursor ? `<div class="cursor" id="${scene.id}-cursor" aria-hidden="true" data-layout-ignore><svg viewBox="0 0 38 48"><path d="M3 3v34l9-9 7 16 7-3-7-16h14L3 3Z" fill="#faf8f3" stroke="#152330" stroke-width="2.5" stroke-linejoin="round"/></svg></div>${scene.cursor.clicks.map((event, i) => event.press ? `<div class="ripple" id="${scene.id}-ripple-${i}" aria-hidden="true" data-layout-ignore></div>` : '').join('')}` : ''
const annotationMarkup = (annotation, id) => `<div class="annotation" id="${id}" data-layout-allow-overflow><div class="annotation-label">${esc(annotation.kicker)}</div><h2 data-layout-allow-overlap>${lines(annotation.lines)}</h2>${annotation.body ? `<p>${words(annotation.body, 'body-word')}</p>` : ''}</div>`

function htmlFor(scene, index) {
  const id = scene.id
  const instant = scene.montage || scene.end
  const intro = instant ? '' : `<div class="intro${scene.photo ? ' intro--photo' : ''}${scene.roles ? ' intro--roles' : ''}${scene.smallTitle ? ' intro--compact' : ''}" id="${id}-intro">
    <div class="intro-kicker">${esc(scene.kicker)}</div>
    <h1 data-layout-allow-overlap>${lines(scene.lines)}</h1>
    <p class="intro-body">${words(scene.body, 'body-word')}</p>
    ${scene.photo ? `<div class="photo-prop" data-layout-allow-overflow><img id="${id}-photo-prop" src="assets/watchtower.webp" alt="The bundled watchtower photograph"></div>` : ''}
    ${scene.roles ? `<div class="role-list">${['Priest', 'Scientist', 'Soldier', 'Historian'].map(role => `<div class="role">${esc(role)}</div>`).join('')}</div>` : ''}
  </div>`
  const annotations = scene.annotations?.map((annotation, i) => annotationMarkup(annotation, `${id}-annotation-${i}`)).join('') ?? ''
  const persistentType = instant ? annotationMarkup(scene, `${id}-story-type`) : ''
  const close = scene.end ? `<div class="close" id="${id}-close">
    <div class="close-lockup"><img id="${id}-brand-symbol" src="capture-home-final/assets/svgs/logo-24eb1a94.svg" alt=""><span>mythborn</span></div>
    <h1 data-layout-allow-overlap>${lines(['Start a', 'civilization.'])}</h1>
    <p>Your next walk. Their next chapter.</p>
  </div>` : ''
  const selector = `#root-${id}`
  const style = `
    @font-face { font-family: Almendra; src: url("assets/fonts/almendra-regular.ttf") format("truetype"); font-weight: 400; }
    @font-face { font-family: Almendra; src: url("assets/fonts/almendra-bold.ttf") format("truetype"); font-weight: 700; }
    @font-face { font-family: Uncial; src: url("assets/fonts/uncial-antiqua.ttf") format("truetype"); font-weight: 400; }
    @font-face { font-family: DM; src: url("assets/fonts/dm-sans-regular.ttf") format("truetype"); font-weight: 400; }
    @font-face { font-family: DM; src: url("assets/fonts/dm-sans-semibold.ttf") format("truetype"); font-weight: 600; }
    ${selector} { position: absolute; inset: 0; width: 100%; height: 100%; overflow: hidden; color: #202932; }
    ${selector} * { box-sizing: border-box; }
    ${selector} .intro, ${selector} .product { position: absolute; inset: 0; width: 100%; height: 100%; background: #f3f0e9; }
    ${selector} .intro { z-index: 3; transform-origin: left center; }
    ${selector} .intro-kicker { position: absolute; top: 196px; left: 112px; color: #ad3f2c; font: 600 24px/1.3 DM, sans-serif; letter-spacing: .12em; }
    ${selector} h1 { position: absolute; top: 286px; left: 106px; width: 1690px; margin: 0; font: 700 152px/.99 Almendra, Georgia, serif; letter-spacing: -.025em; }
    ${selector} .type-line { display: block; white-space: nowrap; }
    ${selector} .accent { color: #ad3f2c; }
    ${selector} .word-mask { display: inline-block; overflow: hidden; vertical-align: bottom; line-height: 1.12; }
    ${selector} .title-word, ${selector} .body-word { display: inline-block; transform-origin: left bottom; will-change: transform; }
    ${selector} .intro-body { position: absolute; left: 112px; top: 726px; width: 1530px; margin: 0; color: #354e62; font: 400 36px/1.35 DM, sans-serif; }
    ${selector} .intro--compact h1 { font-size: 128px; }
    ${selector} .intro--photo h1, ${selector} .intro--roles h1 { width: 1010px; font-size: 132px; }
    ${selector} .intro--photo .intro-body, ${selector} .intro--roles .intro-body { width: 940px; }
    ${selector} .photo-prop { position: absolute; left: 1080px; top: 252px; width: 740px; height: 556px; overflow: hidden; }
    ${selector} .photo-prop img { width: 100%; height: 100%; object-fit: cover; transform-origin: center; }
    ${selector} .role-list { position: absolute; left: 1290px; top: 306px; width: 540px; font: 400 67px/1.46 Almendra, Georgia, serif; }
    ${selector} .role:nth-child(even) { color: #ad3f2c; }
    ${selector} .product { z-index: 1; overflow: hidden; }
    ${selector} .surface { position: absolute; inset: 0; overflow: hidden; width: ${scene.split ? `${scene.surfaceWidth ?? 1150}px` : '100%'}; }
    ${selector} .camera { position: absolute; top: 0; left: 0; width: 1920px; height: 1080px; transform-origin: 0 0; will-change: transform; }
    ${selector} .screen { display: block; width: 1920px; height: auto; clip-path: inset(0 16px 0 0); }
    ${selector} .annotation { position: absolute; z-index: 2; left: 1240px; top: 288px; width: 560px; }
    ${selector} .annotation-label { color: #ad3f2c; font: 600 22px/1.4 DM, sans-serif; letter-spacing: .09em; }
    ${selector} h2 { margin: 28px 0 0; font: 700 76px/1.04 Almendra, Georgia, serif; letter-spacing: -.025em; }
    ${selector} .annotation p { margin: 26px 0 0; color: #354e62; font: 400 31px/1.4 DM, sans-serif; }
    ${selector} .annotation .type-line { white-space: normal; }
    ${selector} .annotation .type-line:last-child { color: #ad3f2c; }
    ${selector} .cursor { position: absolute; z-index: 5; top: -1.875px; left: -1.895px; width: 24px; height: 30px; opacity: 0; transform-origin: 1.895px 1.875px; }
    ${selector} .cursor svg { display: block; width: 100%; height: 100%; }
    ${selector} .ripple { position: absolute; z-index: 4; top: 0; left: 0; width: 38px; height: 38px; margin: -19px 0 0 -19px; border: 2px solid #ad3f2c; border-radius: 50%; opacity: 0; }
    ${selector} .close { position: absolute; z-index: 5; inset: 0; background: #152330; color: #faf8f3; }
    ${selector} .close-lockup { position: absolute; left: 112px; top: 150px; display: flex; align-items: center; gap: 24px; font: 400 70px/1.1 Uncial, Georgia, serif; }
    ${selector} .close-lockup img { width: 76px; height: 76px; filter: brightness(0) invert(1); }
    ${selector} .close h1 { top: 338px; width: 1650px; font-size: 156px; }
    ${selector} .close .accent { color: #e6bc7a; }
    ${selector} .close p { position: absolute; top: 800px; left: 112px; width: 1480px; margin: 0; color: #e6bc7a; font: 400 34px/1.4 DM, sans-serif; }
  `
  const script = `
    (() => {
      const root = document.getElementById("root-${id}");
      const START = ${scene.start};
      const DURATION = ${scene.duration};
      const EASE = { arrive: "cubic-bezier(.16,1,.3,1)", carry: "cubic-bezier(.65,0,.35,1)", snap: "cubic-bezier(.22,.9,.3,1)", linear: "linear" };
      const animate = (element, keyframes, at, duration, easing = EASE.arrive) => {
        if (!element) return null;
        const animation = element.animate(keyframes, { delay: (START + at) * 1000, duration: duration * 1000, fill: "both", iterations: 1, easing });
        animation.pause();
        return animation;
      };
      const moveWords = (container, at, variant = 0) => {
        if (!container) return;
        container.querySelectorAll(".title-word").forEach((word, i) => {
          const from = variant % 3 === 0 ? "translate3d(0,115%,0) rotate(3deg)" : variant % 3 === 1 ? "translate3d(108%,0,0)" : "translate3d(0,-112%,0) scale(1.08)";
          animate(word, [{ transform: from, opacity: 0 }, { transform: "translate3d(0,0,0) rotate(0deg) scale(1)", opacity: 1 }], at + i * .065, .6, variant % 3 === 1 ? EASE.snap : EASE.arrive);
        });
        container.querySelectorAll(".body-word").forEach((word, i) => animate(word, [{ transform: "translate3d(0,105%,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], at + .66 + i * .023, .4));
      };
      const intro = root.querySelector(".intro");
      if (intro) {
        const handoff = ${scene.handoff ?? 0};
        const span = DURATION + ${scene.overlap};
        animate(intro, [
          { transform: "translate3d(${index ? '1920' : '0'}px,0,0)", opacity: 1, offset: 0, easing: EASE.carry },
          { transform: "translate3d(0,0,0)", opacity: 1, offset: ${scene.overlap ? (scene.overlap + .04) / (scene.duration + scene.overlap) : 0.01} },
          { transform: "translate3d(0,0,0)", opacity: 1, offset: (handoff + ${scene.overlap}) / span, easing: EASE.carry },
          { transform: "translate3d(-1920px,0,0)", opacity: 0, offset: (handoff + .7 + ${scene.overlap}) / span },
          { transform: "translate3d(-1920px,0,0)", opacity: 0, offset: 1 },
        ], -${scene.overlap}, span, EASE.linear);
        animate(root.querySelector(".intro-kicker"), [{ transform: "translate3d(-45px,0,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], ${index ? '-.22' : '.1'}, .48);
        moveWords(intro, ${index ? '-.12' : '.16'}, ${index});
        ${scene.photo ? `animate(root.querySelector(".photo-prop"), [{ transform: "translate3d(300px,90px,0) rotate(7deg)", opacity: 0 }, { transform: "translate3d(0,0,0) rotate(0deg)", opacity: 1 }], .35, .85, EASE.carry);
        animate(root.querySelector(".photo-prop img"), [{ transform: "scale(1.14)" }, { transform: "scale(1)" }], .35, 2.6, EASE.linear);` : ''}
        ${scene.roles ? `root.querySelectorAll(".role").forEach((role, i) => animate(role, [{ transform: "translate3d(180px,0,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], .32 + i * .16, .56, EASE.snap));` : ''}
      }
      const product = root.querySelector(".product");
      ${instant
        ? `animate(product, [{ transform: "translate3d(0,1080px,0)", opacity: 1 }, { transform: "translate3d(0,0,0)", opacity: 1 }], -${scene.overlap}, ${scene.overlap + .25}, EASE.carry);`
        : `animate(product, [{ transform: "translate3d(660px,0,0) scale(1.12)", opacity: 0 }, { transform: "translate3d(0,0,0) scale(1)", opacity: 1 }], ${scene.handoff - .05}, .8, EASE.carry);`}
      const poses = ${JSON.stringify(scene.poses)};
      animate(root.querySelector(".camera"), poses.map(([t, s, x, y], i) => ({ transform: "translate3d(" + x + "px," + y + "px,0) scale(" + s + ")", offset: t / DURATION, easing: i === poses.length - 1 ? EASE.linear : EASE.carry })), 0, DURATION, EASE.linear);
      ${scene.annotations ? scene.annotations.map((annotation, i) => `{
        const annotation = document.getElementById("${id}-annotation-${i}");
        animate(annotation, [
          { opacity: 0, transform: "translate3d(95px,0,0)", offset: 0, easing: EASE.snap },
          { opacity: 1, transform: "translate3d(0,0,0)", offset: .25 },
          { opacity: 1, transform: "translate3d(-8px,0,0)", offset: .82, easing: EASE.snap },
          { opacity: 0, transform: "translate3d(-100px,0,0)", offset: 1 },
        ], ${annotation.at}, ${annotation.duration}, EASE.linear);
        moveWords(annotation, ${annotation.at + .02}, ${i + index});
      }`).join('') : ''}
      ${instant ? `{
        const story = document.getElementById("${id}-story-type");
        animate(story, [{ transform: "translate3d(100px,0,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], .18, .55, EASE.snap);
        moveWords(story, .28, ${index});
        animate(story.querySelector(".annotation-label"), [{ transform: "translate3d(0,-20px,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], .15, .5);
      }` : ''}
      ${scene.cursor ? `{
        const cursor = document.getElementById("${id}-cursor");
        const clicks = ${JSON.stringify(scene.cursor.clicks)};
        let previous = ${JSON.stringify(scene.cursor.from)};
        const pose = (x, y, scale = 1) => "translate3d(" + x + "px," + y + "px,0) scale(" + scale + ")";
        const frame = (x, y, time, opacity = 1, scale = 1) => ({ transform: pose(x, y, scale), opacity, offset: time / DURATION, easing: EASE.linear });
        const frames = [frame(...previous, 0, 0)];
        clicks.forEach((click, i) => {
          const begin = click.at - 1.1;
          const dx = click.x - previous[0], dy = click.y - previous[1];
          const bend = Math.min(36, Math.abs(dx) * .12);
          const c1 = [previous[0] + dx * .32 + bend, previous[1] + dy * .12];
          const c2 = [click.x - dx * .22 + bend, click.y - dy * .28];
          frames.push(frame(...previous, begin, i ? 1 : 0));
          // A deterministic sampled Bezier keeps one pointer visible between
          // targets. Smoothstep controls speed; every endpoint is a measured
          // DOM center, and the SVG hotspot stays fixed during the click pulse.
          for (let step = 0; step <= 16; step++) {
            const t = step / 16, u = t * t * (3 - 2 * t), v = 1 - u;
            const x = v*v*v*previous[0] + 3*v*v*u*c1[0] + 3*v*u*u*c2[0] + u*u*u*click.x;
            const y = v*v*v*previous[1] + 3*v*v*u*c1[1] + 3*v*u*u*c2[1] + u*u*u*click.y;
            frames.push(frame(x, y, begin + .1 + t * .84));
          }
          frames.push(frame(click.x, click.y, click.at - .05), frame(click.x, click.y, click.at, 1, click.press ? .9 : 1), frame(click.x, click.y, click.at + .12), frame(click.x, click.y, click.at + .45));
          if (click.press) animate(document.getElementById("${id}-ripple-" + i), [{ transform: pose(click.x, click.y, .35), opacity: .65 }, { transform: pose(click.x, click.y, 2.1), opacity: 0 }], click.at + .02, .55, EASE.arrive);
          previous = [click.x, click.y];
        });
        frames.push(frame(...previous, clicks.at(-1).at + .9, 0), frame(...previous, DURATION, 0));
        animate(cursor, frames, 0, DURATION, EASE.linear);
      }` : ''}
      ${scene.end ? `{
        const close = document.getElementById("${id}-close");
        animate(close, [{ transform: "translate3d(0,1080px,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], 3.55, .85, EASE.carry);
        animate(root.querySelector(".close-lockup"), [{ transform: "translate3d(-110px,0,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], 4.1, .7);
        moveWords(close, 4.35, 0);
        animate(root.querySelector(".close p"), [{ transform: "translate3d(0,38px,0)", opacity: 0 }, { transform: "translate3d(0,0,0)", opacity: 1 }], 5.2, .65);
        animate(product, [{ opacity: 1 }, { opacity: 0 }], 4.35, .15, EASE.linear);
      }` : ''}
    })();
  `
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>${esc(scene.name)}</title></head><body><template>
    <style>${style}</style>
    <div id="root-${id}" data-composition-id="${id}" data-width="1920" data-height="1080" data-duration="${scene.clipDuration}" data-no-timeline>
      <div class="product" id="${id}-product">
        <div class="surface"><div class="camera" id="${id}-camera" data-layout-allow-overflow><img class="screen" id="${id}-screen" src="${esc(scene.image)}" alt="${esc(scene.alt)}" width="${scene.sourceSize.width}" height="${scene.sourceSize.height}" loading="eager" decoding="sync" data-layout-allow-overflow>${cursorMarkup(scene)}</div></div>
        ${annotations}${persistentType}
      </div>
      ${intro}${close}
    </div><script>${script}</script>
  </template></body></html>`
}

for (const [index, scene] of scenes.entries()) {
  await writeFile(path.join(framesDir, `${scene.id}.html`), htmlFor(scene, index).replace(/[\t ]+$/gm, ''))
  await writeFile(path.join(framesDir, `${scene.id}.motion.json`), JSON.stringify({ duration: scene.clipDuration, assertions: [{ kind: 'keepsMoving', withinSelector: `#root-${scene.id}`, maxStaticSec: 1.5 }] }, null, 2) + '\n')
}
const hosts = scenes.map((scene, index) => `    <div class="clip" id="host-${scene.id}" data-label="${esc(scene.name)}" data-composition-id="${scene.id}" data-composition-src="compositions/frames/${scene.id}.html" data-start="${scene.clipStart}" data-duration="${scene.clipDuration}" data-track-index="${index % 2 + 1}" data-width="1920" data-height="1080" style="z-index:${index + 1}"></div>`).join('\n')
await writeFile(path.join(root, 'index.html'), `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=1920,height=1080"><title>Mythborn — a living history</title>
<style>*{box-sizing:border-box}html,body{margin:0;width:1920px;height:1080px;overflow:hidden;background:#f3f0e9}#root{position:relative;width:100%;height:100%;overflow:hidden;background:#f3f0e9}.clip{position:absolute;inset:0}</style></head><body>
<div id="root" data-composition-id="mythborn-showcase" data-width="1920" data-height="1080" data-duration="${filmTime}" data-no-timeline>
${hosts}
</div></body></html>\n`)
await writeFile(path.join(root, 'MOTION_PLAN.json'), JSON.stringify({ duration: filmTime, runtime: 'waapi', clock: 'global', scenes: scenes.map(({ id, start, duration, clipStart, clipDuration, handoff, poses, annotations, cursor, image }) => ({ id, start, duration, clipStart, clipDuration, handoff, poses, annotations, cursor, image })) }, null, 2) + '\n')
console.log(`Built ${scenes.length} kinetic story sections (${filmTime}s). Unboxed typography, overlapping handoffs, and correctly offset WAAPI motion.`)
