# Mythborn visual direction

Mythborn uses illustrated mythology: a woodcut landscape, midnight ink, weathered ivory, and a vermilion accent. The artwork establishes a place; functional pages organize the civilization's evidence, beliefs, and history.

## Typography and color

The lowercase wordmark uses Almendra beside the custom letterpress M. Almendra also supplies civilization names, selected headings, and speaker signatures. DM Sans supplies navigation, forms, metadata, and explanations. Reserve Uncial Antiqua for short introductory headlines rather than every heading. Chronicle text uses generous leading. All fonts are bundled in `public/fonts`; their SIL Open Font License files accompany them. Pages do not contact a font CDN.

Typography references include [Pentiment](https://pentiment.obsidian.net/) for its manuscript and woodcut direction and [Kingdom Come: Deliverance II](https://www.deepsilver.com/games/kingdom-come-deliverance-ii) for medieval presentation. This is an original pairing, not a reproduction of those sites' exact fonts. See `public/fonts/README.md` for source links and placement.

Use ivory surfaces with ink text for reading. Use decorative landscapes for sign-in, the civilization atlas, founding, world covers, and civilization banners. Civilization detail pages pair their restored landscape banner with a typographic journal header below it. Text over artwork needs a dark backing to preserve contrast. Vermilion marks primary actions and selected controls. The brand and speaker signatures use the surrounding ink color. Statuses retain visible labels and their own semantic colors.

Dark mode uses midnight surfaces, warm ivory text, and softer copper accents. The theme switch is always available in the main header and on sign-in. An initial visit follows the device color preference; a manual choice is saved locally and takes precedence. Apply the theme before styles load to avoid a flash. Forms, semantic statuses, and agent marks use theme tokens; illustration colors remain fixed.

The Mythborn logo is the unboxed letterpress M in `public/brand/mythborn-mark.svg`, rendered by `BrandMark` as a color-inheriting mask. The same SVG serves as the favicon. `AgentMark` supplies typographic signatures for the Priest, Scientist, Soldier, and Historian. Names and role labels must accompany those signatures; do not return to sun, atom, shield, and book pictograms for the agents.

## Layout and components

The sign-in page pairs its illustration with a focused form. The atlas uses three medieval scenes: a citadel, a harbor, and a forest sanctuary. Founding and world covers retain their existing artwork. Civilization detail pages begin with the world's landscape banner, followed by its name, founding date, player role, and status. Their overview uses a primary photo column and an open council roster. A selected photo previews in the upload area; no camera icon precedes it. Agents appear as selectable nameplates, while beliefs, chronicles, traditions, rounds, and settings use reading lists rather than repeated boxed panels.

Hero images fade between scenes every 7.5 seconds with a gentle pan. Pause on hover, keyboard focus, and hidden tabs. Slideshow arrows, selectors, and play/pause controls have been removed at the user's request. Reduced motion disables rotation, panning, fades, and pointer depth. Load all scenes before beginning rotation to avoid blank frames. Use a contrast overlay for text rather than changing the artwork itself.

Avoid decorative emoji, circled initials, repeated rounded cards, heavy shadows, and ornamental gradients. Error notices use readable red text, a small warning mark, and an explicit Error label, without tinted backgrounds or borders. Failed-discovery recovery uses a red heading; keep alert announcements and recovery controls. Speaker signatures are printed letterforms with no badge or surrounding shape. `Sigil` remains limited to existing player-role and founding contexts. Illustrations are shared scenery, not depictions of generated game state or uploaded evidence.

The palette and common controls live in `src/styles/theme.css`; shared layouts live in `src/styles/app.css`. `src/styles/civilization.css` defines the journal screens, and `src/styles/interaction-motion.css` defines their motion. Loading states use a custom compass spinner and contextual text, centered for full pages and compact inside panels. Legacy moss/copper token names remain compatible with existing components, but their values follow the new palette.

## Accessibility

Keep native form labels, visible keyboard focus, touch-sized controls, responsive columns, and reduced-motion support. Landscape backgrounds are decorative. World names, roles, statuses, and actions must remain readable without the artwork.
