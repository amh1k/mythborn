# Mythborn: shortcomings and a more distinctive design direction

**Visual scope correction (11 October 2026):** The user asked to retain the previous illustrated mythology design. The field-journal redesign below is superseded: preserve the rotating illustrated hero, world-cover artwork, civilization banners, medieval typography, and existing page layouts. Reliability, live debate, saved stage feedback, polling, and recovery controls remain in scope.

This is a work plan based on the current repository, including uncommitted changes, reviewed on 10 October 2026. Code observations are distinguished from risks and work that still needs verification. Visual recommendations are design judgments based on the current components, styles, artwork documentation, and the mobile capture reviewed earlier in this session. This is not a new live usability or production audit.

**The priority:** make the discovery experience reliable, then give it the character of a civilization's field journal. The most interesting material is already specific to Mythborn: a real photo, four conflicting interpretations, changing beliefs, and a recorded consequence. Let those things determine the interface.

## 1. What already exists

Do not spend the next iteration rebuilding these features:

- Local Temporal setup, API and worker startup, and backend restart commands.
- Four founding agent templates: Priest, Scientist, Soldier, and Historian.
- Live reactions and rebuttals with temporary debate previews.
- Automatic model rate-limit waits, retry countdowns, and preservation of successful sibling responses during those waits.
- A Rounds view and reusable round progress component in the current working tree.
- Locally bundled fonts, light and dark themes, reduced-motion handling, and responsive layouts.

### Agreed behavior: stage feedback and automatic retries

The progress UI should explain the current work, rather than relying on a small status label. This follow-up implements the following behavior in the discovery page and records it here so the requirements and interface remain aligned:

| State | Feedback shown to the player |
| --- | --- |
| Queued | Waiting to start; the page checks for updates automatically. |
| Describing | Preparing the shared photo description before the agents begin. |
| Awaiting review or clarity choice | Waiting for the player's decision; no processing animation. |
| First reactions | Agents are forming their initial interpretations; received responses are counted and shown. |
| Rebuttals | Agents are responding to the first reactions; the UI explicitly says “Rebuttals in progress,” counts received rebuttals, and marks unfinished agents as preparing a rebuttal. |
| Writing | The Historian is writing the chronicle after the debate; other agents show that their debate is complete. |
| Model rate limit | Automatic retry countdown for the affected agent or phase; completed responses stay visible. No reload or manual retry is required for a quota wait. |
| Live view unavailable | Reconnecting to live progress; the saved stage remains visible. Do not imply that the model itself stopped or completed. |
| Paused after an ordinary failure or prolonged quota exhaustion | Clearly paused, with a reason and the existing manual retry action. Ordinary short quota waits still retry automatically. |
| Complete | Saved chronicle, with no processing animation. |

Use a restrained activity indicator only for active work. Completed responses, quota waits, player decisions, and paused rounds must not look as though an agent is still generating a response. Respect reduced motion. Counts represent real received contributions, not an invented percentage or completion estimate.

**Implemented and exercised locally:** HTTP 429 causes a durable Temporal wait, with retries capped at eight attempts and a 45-minute total window. Successful agent contributions remain available while the affected call waits. Separate rolling 60-second request/token schedules now cover the text and embedding models. Budgets are split across the configured worker count; no shared database or distributed scheduler coordinates independently configured deployments. The page polls progress automatically, so refreshing is unnecessary. A paused prolonged outage is different from an ordinary automatic retry countdown.

**Verified locally:** the full Go suite, quota interruption/checkpoint workflow cases, isolated PostgreSQL migration checks, frontend production build, and mocked browser navigation/stage scenarios pass. These checks do not replace the real-photo acceptance run below.

The four agents are characters. The player roles are Observer, God, and Messenger; Admin is an account permission. Keep those concepts distinct in copy and documentation.

## 2. Shortcomings to resolve

Priority meanings: **P0** before relying on the app for a public demo; **P1** before a wider release; **P2** after the core experience is dependable.

### P0 — Validate the prolonged quota policy and lifecycle interruption

**Original issue:** quota waits previously retried indefinitely while the coordinator handled a command synchronously. A persistent outage could keep later lifecycle commands queued and accumulate history.

**Current follow-up implementation:** [rate_limit.go](internal/workflows/rate_limit.go) now contains an increasing-delay retry budget, and [Coordinator](internal/workflows/coordinator.go) listens for lifecycle interrupts so active work can be canceled before handling end/delete/set-aside commands. These changes are present in the working tree; their existence does not establish successful deployment or recovery behavior.

**Resolve:** verify the retry budget, the reason shown when exhausted, safe interruption, and checkpoint retention. Expose the new set-aside endpoint in the interface if it is not already wired there. Confirm that bounded waits and lifecycle interruption keep history growth under control while retaining automatic retries for short quota windows.

**Done when:** a simulated extended quota outage keeps prior responses intact, does not generate a chronicle, and still allows a requested deletion to complete. A long-running retry simulation stays within a defined history budget.

### P0 — Validate proactive model budgets across worlds and workers

**Original issue:** limiting simultaneous calls alone did not schedule requests against a shared request or input-token budget. **Current follow-up implementation:** [model scheduling](internal/models/budget.go) spaces requests, estimates input-token usage, caps text prompts at 48 KiB, and records usage and wait timings. Instances with the same provider key and budget share a scheduler within one process. `MODEL_RPM`, `MODEL_INPUT_TPM`, and `MODEL_WORKER_COUNT` configure the allowance; separate workers divide it using the declared worker count. This is a configured allocation, not a distributed quota coordinator.

**Remaining:** compare estimates against actual provider usage during a real run and configure quotas from the account dashboard. Confirm production worker replicas share the declared total worker count. The configured scheduler splits allowance across replicas; independently deployed processes do not exchange live reservations.

**Done when:** you can explain the approximate request/token cost of a discovery and demonstrate that several worlds do not continuously send another burst immediately after the same quota window expires.

### P0 — Verify the new image-processing path

**Original issue:** upload accepted images using byte/signature checks without decoding, normalization, or metadata removal. **Current follow-up implementation:** [photo processing](internal/app/photos.go) now decodes images, enforces a 24-megapixel limit, normalizes orientation, resizes to a maximum edge of 2048 pixels, and re-encodes JPEG before [upload](internal/app/observations.go). The implementation comments specify retention of only the normalized image.

**Verified locally:** tests cover all eight EXIF orientations, GPS/comment removal, transparent input, edge resizing, corrupt images, and oversized decoded dimensions. Duplicate detection hashes the submitted original while only the resized, metadata-free JPEG is stored. Upload guidance now states the byte and pixel limits. A live private-bucket upload and retrieval still needs the acceptance run.

**Done when:** rotated phone photos display correctly, oversized decoded images are rejected, corrupt inputs receive useful errors, and the image sent to the model contains no retained location metadata. “Location metadata is not used” should not be confused with “location metadata is removed.”

### P0 — A repeatable full discovery demonstration still needs evidence

**Verification gap:** earlier work established local connectivity, a successful model probe, automated retry tests, and browser checks with mocked data. Those checks do not establish a repeatable successful journey through real upload, all agent calls, saved lore, and later retrieval. Older implementation notes also contain statements that have become stale.

**Documented in [DEPLOYMENT.md](DEPLOYMENT.md); still to run:** create a world, submit a photo, finish its chronicle, reload, submit a related photo, and confirm that prior history is actually retrieved for the next episode. Include one worker interruption and one controlled rate limit. Record failures and timings instead of treating a passing UI build as completion of the integration.

**Done when:** another person can follow the same run and inspect the resulting chronicles, relevant belief changes, and absence of duplicate commits.

### P1 — Verify individual checkpoints beyond the quota path

**Original issue:** successful contributions from a failed phase could be lost on a later manual retry. **Current follow-up implementation:** `runAgentPhase` now checkpoints accepted contributions individually and skips saved agents, with evidence-aware checkpoint invalidation. Verify this behavior rather than treating it as an absent feature.

**Verified locally:** workflow tests show three accepted agent responses survive a fourth agent's ordinary failure, then only that missing agent runs on manual retry. Evidence changes invalidate the checkpoint. Debate text remains transient workflow state and is not added to permanent tables. A worker-restart replay is still part of the live acceptance run.

**Done when:** three successful contributions survive a fourth agent's non-quota failure and a manual retry only requests the missing contribution, unless its inputs changed.

### P1 — The four characters need an editorial quality standard

**Observed:** [agent activities](internal/activities/activities.go) provide personality, beliefs, memory, and phase instructions. Structured validation checks output shape and permitted changes. That does not establish whether the four voices are memorable, whether they react to the actual photo, or whether the Historian accurately preserves dissent.

**Implemented:** prompts now include grounded-evidence and uncertainty rules, varied-length guidance, and motivations for belonging/promises (Priest), testable mechanisms (Scientist), reversible safety choices (Soldier), and attributed dissent (Historian).

**Remaining:** use the reference set and scoring sheet in [CHARACTER_EVALUATION.md](CHARACTER_EVALUATION.md) to review actual outputs for concrete evidence, distinct language, continuity, justified changes, and repetition. Let an uneventful discovery remain uneventful; do not force a new religion or ritual every time.

**Done when:** readers can distinguish speakers without seeing their labels, identify which visible detail caused each interpretation, and trace the chronicle's claims back to the debate. Treat this as an evaluation task, not a claim that all current outputs are bad.

### P1 — Verify photo evidence during JSON repair

**Original issue:** JSON repair omitted the original photo. **Current follow-up implementation:** [GenerateJSON](internal/models/gemini.go) now attaches the image again and instructs the repair to preserve evidence and uncertainty without inventing observations.

**Verified in a request-level model-client test:** an invalid draft triggers a second request that contains the original image and the uncertainty-preservation instruction. Review a real repair against its photo during the live acceptance run.

**Done when:** malformed model JSON does not cause a repaired description to invent details or silently change the observed evidence.

### P1 — Verify diagnostics from the player report through the worker

**Current follow-up implementation:** [request diagnostics](internal/api/diagnostics.go) now emit `X-Request-ID` and structured request timings. Common [HTTP errors](internal/httpapi/handlers/games.go) and authentication errors include useful messages and a request reference. Model generation logs include game, round, agent, phase, estimated and actual usage where available, and queue wait time. API logs include request IDs and returned command/round IDs; model events include the same round and workflow event IDs. The browser shows request references on errors. A live log query and outage report still need checking.

**Remaining:** verify the correlation with deployed logs and measure queue age and activity duration during a controlled outage. Prompts, model responses, photos, and credentials are not logged.

**Done when:** a report such as “this discovery has been waiting for ten minutes” can be traced to a specific stage and cause using one reference.

### P1 — Local operation is better documented than fresh setup and hosting

**Implemented:** README instructions install Goose, apply migrations and seeds, and document first-admin setup. `DEPLOYMENT.md`, a Render Blueprint, and GitHub Actions checks now describe the Go API/worker, static site, and release commands. The Temporal CLI is used from `bin/` in local development. These deployment files have not been validated by creating a hosted Render environment.

**Remaining:** verify clean-clone instructions on another machine, validate the Blueprint in Render, configure real service secrets, and run migrations/deployment through a staged release. Local CI checks cannot establish recovery on the hosted environment.

**Done when:** a clean checkout can be configured without undocumented local files, and the chosen hosted environment can recover its worker and persistent workflow state after a restart. Local Temporal should remain a supported development option.

### P2 — UI structure and polling will become harder to maintain

**Partly addressed:** the discovery route now lives in [episode.tsx](web/src/pages/episode.tsx), and shared polling pauses hidden/offline tabs and backs off after errors. World-round and live-debate views use that hook. Other screens, including world setup and admin, still live in `app.tsx`; they need staged extraction.

**Remaining:** extract the remaining screens when they change, and unify Messenger command polling. Browser checks cover world navigation and responsive episode stages; verify request counts on multiple open tabs before a wider release.

**Done when:** opening multiple tabs does not multiply background requests unnecessarily, and changing episode status handling does not require editing several competing implementations.

## 3. Why the current style can feel generic

These are design observations, not implementation bugs:

1. **The theme is stronger than the page hierarchy.** Uncial titles, calligraphic headings, uppercase labels, status marks, and decorative illustrations compete for attention. Too many elements announce that this is a fantasy product.
2. **Repeated page framing delays the substance.** A large page heading, stage panel, explanatory text, another section heading, and an agent roster can precede the actual debate. On a phone this pushes useful content far down the page.
3. **Scenery is weakly connected to the player's history.** [World covers](web/src/lib/world-art.ts) are selected from three images using the world ID. They give atmosphere but do not convey what distinguishes one civilization from another.
4. **The components share too much visual weight.** Existing CSS already uses square corners, fine rules, and open debate rows. The remaining problem is composition and hierarchy; merely removing more border radii will not solve it.
5. **The experience often introduces itself instead of showing its content.** Copy such as “Four voices, one discovery” describes the concept repeatedly. A specific photo caption, disagreement, or changed belief would make that space more valuable.
6. **The artwork collection lacks one strict visual grammar.** The art notes describe both restrained woodcut scenery and more colorful, richly detailed medieval scenes. This is a consistency issue to resolve through art direction. Being generated is not, by itself, the reason an image feels generic.

## 4. Proposed direction: the civilization's field journal

Imagine a record assembled over time by people trying to explain their surroundings: photographed evidence, brief testimony, marginal disagreement, and a formal historical account.

Use three recurring devices:

- **Evidence plate:** the player's photo, a factual caption, and the discovery number.
- **Speaker mark:** one consistent emblem and a clear name for each agent.
- **Historical annotation:** a compact note showing a real belief or tradition change and the round that caused it.

Those devices should recur across the app. Their content should come from the player's world. Avoid manufacturing meaningful-looking stamps, maps, dates, or statistics when no corresponding state exists.

This direction intentionally revises the broad medieval typography, rotating scenery, and repeated banners in [the current style guide](web/STYLE_GUIDE.md). Update that guide when the new direction is adopted so future work follows one set of rules.

### Rule 1 — Give every screen one dominant purpose

| Screen | What should dominate | What should recede |
| --- | --- | --- |
| Sign-in | A clear form and one memorable illustration | Repeated product explanation |
| World list | World names, latest event, and an active round if present | Large rotating promotional scenery |
| World overview | Latest consequence and the next photo action | A second introductory hero |
| Active discovery | Evidence and incoming contributions | Large stage cards and generic asides |
| Chronicle | A specific title, verdict, and readable narrative | Dashboard furniture |
| Agent | Current convictions and meaningful changes | A generic biography block |
| Admin | Fast editing, comparison, and validation | Storytelling decoration |

Use cards where they group a real interactive unit. Use lists, spacing, and rules for reading. Avoid wrapping every heading, paragraph, and button in another container.

### Rule 2 — Reduce decorative typography

- Keep Uncial Antiqua for the wordmark and occasional short chapter titles.
- Use one readable face for body text, controls, and routine headings. DM Sans already exists and can serve that role without adding another dependency.
- Keep Almendra for selected names or short quotations only if it improves the composition. Do not automatically use it for every contribution and long chronicle paragraph.
- Start body copy around 16–18 px with 1.5–1.7 line height. Aim for roughly 55–75 characters per line on wide screens.
- Use a small type hierarchy: page title, section title, body, metadata. Repeated uppercase eyebrows should be rare.
- At 320 px, a routine page title should usually fit within two lines. Reduce its size or shorten the wording before allowing it to consume the first screen.

### Rule 3 — Keep the existing palette, give it stricter jobs

The current paper, ink, and vermilion palette is usable. A new palette alone will not establish identity.

| Color role | Intended use |
| --- | --- |
| Paper / near-black reading surface | Most of the page |
| Ink / warm light text | Main content |
| Muted ink / muted light text | Dates, labels, secondary details |
| Vermilion | The main action and a small number of important marks |
| Semantic success/error colors | Actual state, accompanied by a label |

Do not make every subtitle, icon, rule, and status red. Keep status meaning independent of character identity. Tune dark mode for readable long passages, not dramatic glow. Verify contrast on the actual combinations used.

### Rule 4 — Make the uploaded photo the central image

Show it at a useful size with its caption and a clear distinction between visible evidence and the player's interpretation. Never replace it with decorative scenery during the most important part of a discovery.

Use one art language across decorative assets: consistent line weight, level of detail, palette, and treatment of light. Favor recognizable compositions over indiscriminate texture and intricate fantasy architecture. A restrained handmade mark or a commissioned illustration can help, but additional art should solve a specific compositional need.

### Rule 5 — Let real differences create world identity

A civilization becomes distinctive through its name, discoveries, disputes, beliefs, and traditions. Surface those differences in its list entry and overview.

For example: show its latest chronicle title and a real contested belief rather than relying on a different castle picture. If a visual motif is eventually derived from traditions, define that mapping honestly and keep it stable. Do not imply that existing decorative covers depict simulated settlements.

### Rule 6 — Lay out debate as a readable exchange

Use a single reading column with clear speaker labels. On wider screens, a narrow margin can hold agent marks or annotations; on phones, place that information above the contribution. Avoid four equal chat panels that force the reader to jump between columns.

Suggested order for an active discovery:

```text
Round number · current stage
Specific discovery title, when available

[Evidence photo]
Factual caption / player statement

First reactions                       3 of 4 received
Priest       [completed contribution]
Scientist    [completed contribution]
Soldier      [completed contribution]
Historian    Rate limit reached. Retrying in 1 minute.

Rebuttals, as they arrive
```

Collapse the evidence only through an understandable control. Keep completed responses in place while waiting; avoid layout jumps, invented typing, or auto-scrolling that interrupts someone reading an earlier response.

### Rule 7 — Replace atmospheric filler with specific copy

Use direct language for actions and status. Reserve literary language for the characters and chronicles.

| Current pattern | More useful direction |
| --- | --- |
| “A discovery is unfolding” on every episode | “Discovery 12” plus a specific title when one exists |
| “Four voices, one discovery” | “First reactions · 3 of 4 received” |
| “Bring the world a discovery” | “Add a photo” |
| Repeated “What happens next” explanations | Explain once during the first discovery; keep help available |
| Generic “Something went wrong” | Name the failed stage and the next available action |

Example fictional chronicle title: “The evening the bridge became forbidden.” Use such wording only when that actually happened in the world. Specificity comes from content, not random poetic labels.

### Rule 8 — Make consequences legible

After a chronicle, show a compact, factual account of what changed: a belief before and after, a tradition adopted or challenged, or an explicit note that no lasting change occurred. Link changes to their source round.

Do not add generic scores such as “culture energy” or “civilization wisdom” merely to fill dashboard space. Distinguish saved consequences from proposals raised during the debate.

### Rule 9 — Use motion to explain change

Prefer a restrained reveal when a response arrives or a stage completes. Remove automatic scenery rotation from routine world-management screens in the proposed redesign. Keep any decorative motion limited to an intentional introductory setting.

Maintain reduced-motion support, visible focus, keyboard operation, and comfortable touch targets. Never animate a fake percentage or imply an exact completion time when only a retry deadline is known.

### Rule 10 — Design the awkward states first

Prepare compositions for an empty world, one response received, a long rebuttal, simultaneous quota waits, unavailable live preview, review-required photo, completed disagreement, and archived world.

These states should feel like the same product. For a quota wait, use a plain status line and keep the evidence and completed contributions visible. For a completed unresolved debate, present the disagreement as the recorded result; it is a different state from an interrupted model call.

### Rule 11 — Put character into content before ornament

Give the Priest recurring concerns, the Scientist a way of questioning evidence, the Soldier a practical threshold for danger, and the Historian a particular way of remembering. Allow individual history to complicate those roles.

Avoid making every speaker produce the same polished paragraph with a different profession inserted. Vary response length when appropriate. Keep concrete nouns, direct claims, and evidence. Decorative manuscript fonts cannot compensate for interchangeable writing.

### Rule 12 — Preserve usability while becoming distinctive

Keep plain labels for Upload, Retry, Delete, and Settings. Use visible text alongside agent symbols. Maintain reading order when columns collapse, support text enlargement, and inspect both themes.

A design does not become more original by making a familiar action hard to find. The identity should come from composition, editorial choices, imagery, and the civilization's own history.

## 5. Recommended order of work

1. **Reliability:** address prolonged quota handling, lifecycle responsiveness, photo processing, and the repeatable full discovery run.
2. **One representative discovery screen:** design a real example from photo through debate to chronicle, including a rate-limit wait. Begin at phone width, then expand the same hierarchy for desktop.
3. **Typography and structure:** reduce title competition, repeated introductory copy, and container nesting before commissioning or generating more artwork.
4. **Character and consequence:** improve voice evaluation and make saved belief/tradition changes visible.
5. **Apply the established language:** update world overview, history, agents, and finally the entry screens. Keep Admin functionally restrained.
6. **Documentation and release:** reconcile the style guide and setup docs with the implementation, then publish the corresponding changes together.

## 6. Acceptance checklist for the redesign

- [ ] A new player can identify the next action and current state within a few seconds.
- [ ] A phone screen reaches meaningful evidence or a contribution without scrolling past several introductions.
- [ ] Each page has one clear focal point; decoration supports it.
- [ ] Two worlds look different because their visible histories differ.
- [ ] The four characters remain distinguishable when their labels are hidden in a writing review.
- [ ] The photo, interpretation, proposal, and saved consequence are visually distinguishable.
- [ ] A quota wait preserves responses and explains when another attempt is scheduled.
- [ ] Completed disagreement is visibly different from processing failure.
- [ ] Long content, 320 px width, text enlargement, keyboard navigation, both themes, and reduced motion remain usable.
- [ ] The experience is still recognizable as Mythborn when the background illustrations are removed.
- [ ] Every decorative element has a specific purpose; every displayed historical claim comes from actual state.

The strongest version of Mythborn should feel assembled around this civilization's evidence and arguments. A visitor should remember an incident, a character, or a disagreement—not only the fantasy wallpaper.
