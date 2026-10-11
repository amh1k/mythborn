# Mythborn showcase — frame direction

## Source and intent

Derived from the local Mythborn app capture at `capture-home-final/` and the civilization, discovery, debate, chronicle, belief, mystery, culture, and history captures in `capture-interactions/`. The polished cut uses settled, complete captures in `capture-polish/`, with control rectangles recorded in `measurements.json`. The product screens and bundled artwork are real project assets; the civilization content and agent responses are fictional local demo fixtures.

The film follows one tower photograph until it becomes a contested memory and a daily ritual. Keep Mythborn's editorial, old-world register and the app's own UI legible. Treat the interface as a place the camera visits, not a mockup to decorate.

## Palette

- Canvas / paper: `#F3F0E9` (captured `--color-paper`).
- Raised paper: `#FAF8F3` (captured `--color-paper-raised`).
- Ink: `#202932` (captured `--color-ink`).
- Night surface: `#152330` (captured `--surface-inverse`).
- Copper focal: `#AD3F2C` (captured `--color-copper`).
- Quiet text: `#5C6269` (captured `--color-ink-muted`).
- Rules: `#D0CEC5` (captured `--color-line`).
- Warm light: `#E6BC7A` (captured in the captured palette and hero artwork).

Use warm paper as the default designed-frame canvas, with ink and copper carrying hierarchy. Keep dark surfaces for the actual atlas hero and brief contrast moments. Do not invent a second accent color.

## Type

- Display: Almendra, 400/700, with Georgia fallback. Use for the story's large statements and the civilization name.
- Product heading: Uncial Antiqua, 400, with Georgia fallback. Preserve its use in the actual product UI.
- Interface and callouts: DM Sans, 400/600/700, with a sans-serif fallback.
- Small metadata: DM Sans 600 with tracked capitals; avoid tiny simulated terminal text.

For designed video frames, use 128–156px for opening phrases, 76px for words beside the product, 30–40px for supporting copy, and 22–24px for labels. Keep exact interface typography and sizing inside captured screens. The family names and palette are taken from `capture-home-final/extracted/tokens.json`.

## Composition and motion

- Work at 1920×1080. Favor asymmetric editorial layouts, a single visual anchor, and generous paper around the app surface.
- Keep the product screenshot large enough to read; crop browser/app chrome where it carries no story, and never redraw the UI.
- Copper appears as a decisive word or actual app control. No orange side stripe.
- Words enter individually through masks, using alternating vertical and horizontal directions. Editorial text has no box, border, or backing card.
- Each opening phrase carries left as the product arrives from the right. Adjacent scene hosts overlap by 0.45 seconds; belief, mystery, and culture continue upward through the same paper surface.
- Move through meaningful app details in several camera poses. Slow travel gives the chronicle reading time without stopping the film. Cursor cues remain attached to the actual control coordinates.
- Native WAAPI receives absolute film times, including each scene's start. All animation is paused and deterministic for seeking.
- No narration, music, or sound effects. The film is silent; text carries the guide.

## Avoid

No generic AI glow, gradient headline, neon/cyan palette, floating browser mockup, fake app controls, stock icons, repeated equal cards, decorative glyph clutter, boxed captions, or long static slide holds. No left-edge accent bar. Camera movement must select a meaningful detail rather than repeatedly zooming the same composition. Do not cover meaningful app copy with explanatory text. Keep account/admin chrome out of the focus when a crop can do so cleanly.
