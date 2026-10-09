# Mythborn style guide

Mythborn should feel like a field journal that becomes a living archive: warm paper, deep ink, moss green, and small copper accents. Use the system serif stack for editorial headings and the system sans stack for interface text. Keep photos and camera actions prominent, with calm surfaces and high-contrast controls that remain legible outdoors. The theme uses no remote fonts or visual assets.

Import `src/styles/theme.css` once from the React entry point. The stylesheet supplies a small global foundation rather than page layouts or components.

Use these tokens for screen-specific styling:

- **Color:** `--color-ink`, `--color-paper`, `--color-paper-raised`, `--color-moss`, `--color-moss-dark`, `--color-copper`, `--color-sky`, `--color-danger`, and `--color-focus`.
- **Semantic roles:** `--surface-page`, `--surface-card`, `--surface-subtle`, `--text-primary`, `--text-secondary`, `--text-inverse`, `--border-subtle`, `--accent-primary`, and `--accent-secondary`.
- **Type:** `--font-body`, `--font-display`, `--font-mono`, and `--text-xs` through `--text-3xl`.
- **Layout:** `--space-1` through `--space-16`, `--radius-sm` through `--radius-pill`, `--shadow-card`, `--shadow-raised`, `--content-width`, `--reading-width`, and `--control-height`.

Available baseline classes are `.container`, `.reading-width`, `.surface-card`, `.surface-subtle`, `.eyebrow`, `.text-muted`, `.text-small`, `.button`, `.button-secondary`, `.button-quiet`, `.button-danger`, and `.status-pill`. A status pill can use `data-state="active"`, `"complete"`, or `"attention"`; always include visible text so state is not conveyed by color alone.

Use native buttons, links, labels, inputs, and headings. Keep keyboard focus visible, provide accessible names for icon-only controls, associate error text with invalid fields, and use `aria-invalid="true"` when appropriate. Buttons and fields have touch-friendly minimum heights, and the theme respects reduced-motion preferences. For photo capture, retain the browser's native camera/file input behavior and add screen-specific framing without weakening the focus indicator or control contrast.
