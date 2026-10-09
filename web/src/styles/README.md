# Mythborn visual foundation

The theme pairs a warm field-journal paper surface with dark ink, moss green, and restrained copper. Display headings use a system serif stack to suggest an archive; body text uses a clear system sans stack so controls and outdoor photo flows remain easy to scan. No remote fonts or image assets are required.

Import `web/src/styles/theme.css` once from the application entry point. Build screens with semantic HTML, then use the CSS variables for product-specific styling. Common starting classes are `.container`, `.reading-width`, `.surface-card`, `.surface-subtle`, `.eyebrow`, `.button`, `.button-secondary`, `.button-quiet`, `.button-danger`, and `.status-pill` (with `data-state="active|complete|attention"`). Buttons and fields keep native semantics; set accessible names and use `aria-invalid="true"` for invalid fields.

Color, typography, spacing, shape, and elevation values live in `:root`. Prefer those tokens over local one-off values so the civilization, agent, culture, and chronicle screens feel like one product. The stylesheet includes visible keyboard focus, touch-sized controls, a skip-link style, narrow-screen foundations, and reduced-motion handling.
