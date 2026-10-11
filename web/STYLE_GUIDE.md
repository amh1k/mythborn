# Mythborn visual direction

Mythborn uses illustrated mythology: a woodcut landscape, midnight ink, weathered ivory, and a vermilion accent. The artwork establishes a place; functional pages organize the civilization's evidence, beliefs, and history.

## Typography and color

Uncial Antiqua supplies the medieval wordmark, page titles, and hero headlines. Almendra supplies calligraphic section headings, world and agent names, chronicle prose, and italic suggestions. The type scale is more compact to accommodate the wider manuscript letterforms. Chronicle text uses generous leading and a decorative initial. DM Sans supplies navigation, forms, metadata, and explanations. All fonts are bundled in `public/fonts`; their SIL Open Font License files accompany them. Pages do not contact a font CDN.

Typography references include [Pentiment](https://pentiment.obsidian.net/) for its manuscript and woodcut direction and [Kingdom Come: Deliverance II](https://www.deepsilver.com/games/kingdom-come-deliverance-ii) for medieval presentation. This is an original pairing, not a reproduction of those sites' exact fonts. See `public/fonts/README.md` for source links and placement.

Use ivory surfaces with ink text for reading. Reserve the dark landscape for sign-in, the civilization atlas, founding, and world banners. Text over artwork needs a dark backing to preserve contrast. Vermilion marks primary actions, selected roles, and the brand. Statuses retain visible labels and their own semantic colors.

Dark mode uses midnight surfaces, warm ivory text, and softer copper accents. The theme switch is always available in the main header and on sign-in. An initial visit follows the device color preference; a manual choice is saved locally and takes precedence. Apply the theme before styles load to avoid a flash. Forms, semantic statuses, and agent marks use theme tokens; illustration colors remain fixed.

The Mythborn logo is the angular mountain-shaped M beneath a red sun in `public/brand/mythborn-mark.svg`. Use `BrandMark` beside the wordmark with explicit dimensions so it remains visible on small screens. The same SVG serves as the favicon. Role and agent icons use `Sigil` separately from the brand.

## Layout and components

The sign-in page pairs its woodcut illustration with a focused form. The atlas uses three colorful medieval scenes in an animated hero: a citadel, a harbor, and a forest sanctuary. Founding uses its own portrait of a village under construction. World covers use a separate watchtower, monastery, and river-crossing collection; a world's detail banner retains its cover. A civilization's overview uses a primary evidence column and a compact contextual aside; beliefs and chronicles use readable lists and rules.

Hero images fade between scenes every 7.5 seconds with a gentle pan. Pause on hover, keyboard focus, and hidden tabs. Manual selection pauses automatic rotation. Keep previous/next, scene selectors, and a play/pause control visible. With reduced motion, disable rotation, image panning, and fades while retaining manual selection. Load all scenes before beginning rotation to avoid blank frames. Use a contrast overlay for text rather than changing the artwork itself.

Avoid decorative emoji, circular initial avatars, repeated rounded cards, heavy shadows, and ornamental gradients. Use `Sigil` for role and agent marks. Illustrations are shared scenery, not depictions of generated game state or uploaded evidence.

The palette and common controls live in `src/styles/theme.css`; page layouts live in `src/styles/app.css`. Legacy moss/copper token names remain compatible with existing components, but their values follow the new palette.

## Accessibility

Keep native form labels, visible keyboard focus, touch-sized controls, responsive columns, and reduced-motion support. Landscape backgrounds are decorative. World names, roles, statuses, and actions must remain readable without the artwork.
