# Mythborn styles

Import `theme.css` and `app.css` once from the application entry point. `theme.css` defines locally hosted fonts, palette tokens, native form styles, shared controls, focus visibility, and reduced-motion handling. `app.css` defines responsive page and component layouts.

The current direction is illustrated mythology. See `web/STYLE_GUIDE.md` for composition, type, artwork, and accessibility guidance. Art and font assets are served from `web/public`; there are no runtime font CDN requests.

Shared classes include `.container`, `.surface-card`, `.surface-subtle`, `.eyebrow`, `.button`, `.button-secondary`, `.button-quiet`, and `.status-pill`. Despite the retained surface class names, use open layouts and borders where reading or comparison benefits from them. Reserve the illustration for narrative framing.
