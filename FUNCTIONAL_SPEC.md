# Mythborn: Functional Specification

Status: Version 1 product behavior is specified. This document describes the intended web app, not features already implemented. The choices made by the project creator and the defaults chosen at their request appear at the end. See [TECHNOLOGIES.md](./TECHNOLOGIES.md) for the technology choices, [ARCHITECTURE.md](./ARCHITECTURE.md) for service boundaries, and [DATABASE_DESIGN.md](./DATABASE_DESIGN.md) for the agreed persistence rules and proposed schema.

## 1. Purpose

Mythborn gives a single player a small AI civilization that interprets the player's outdoor photos as extraordinary events. Four persistent agents disagree, remember what they have seen, develop beliefs and shared traditions, and record their history. A player should spend little time capturing an observation, go back to the real world, and return to see what the society made of it.

The defining demonstration is one photo producing four distinct reactions, a round of rebuttals, belief and culture changes, and a chronicle. A later photo should reveal that the agents remember earlier events.

A civilization is one player's persistent world. An observation is a submitted photo and any role-specific player statement. A discovery episode is the agents' response to one observation. A chronicle is the historian's saved account of an episode. A tradition is a shared myth, ritual, or taboo.

## 2. Player account and world creation

Players sign in before creating or opening a civilization. Version 1 supports email and password plus Google and GitHub sign-in. A player may own several independent civilizations and access them from another browser or device. Each civilization belongs to exactly one account. Multiplayer and shared control are outside version 1.

Creation flow:

1. The player opens **New civilization**.
2. The app presents previews of **Observer**, **God**, and **Messenger**, including what the player can do and one example of how the agents would treat a rain photo.
3. The player selects a role and sees that the choice is permanent for this civilization.
4. The app suggests a civilization name, which the player may edit, and creates the world.
5. The player sees the four founding agents and an invitation to make the first outdoor observation.

The world begins with the agents' personalities, a short prologue, and initial beliefs defined by administrators. Starting configuration and beliefs are copied when the game is created; later template edits affect new games only. Starting beliefs are character assumptions rather than evidence of an outside observation. The world has no adopted traditions initially. A player's role cannot change within that world; a new civilization is needed to try another role.

An account can also have the system-level Admin permission, separate from its role in any game. Administrators have access to all application operations through the admin portal, including initial belief configuration and account/game management. Administrative access preserves game-role immutability and data consistency.

The civilization list shows active worlds and completed archives separately. Signing out leaves all worlds intact. Ownership checks apply to every photo, conversation, belief, tradition, chronicle, and workflow operation.

## 3. Player roles

The same four-agent simulation powers all three roles. The role changes who the agents think the player is, how a photo reaches them, and which player actions are available.

| Role | What the agents know | Player actions | Agent interpretation |
| --- | --- | --- | --- |
| Observer | They do not know a player supplies discoveries. | Submit photos and inspect the society; no in-world messages. | A photo arrives as an unexplained phenomenon. Agents can invent conflicting explanations without player clarification. |
| God | They recognize a claimed divine presence. | Submit photos and optionally attach one public proclamation to each observation. | The photo is a possible sign. Agents may accept, misread, question, or reject the proclamation. |
| Messenger | They know a mortal outsider brings evidence. | Submit photos with optional firsthand testimony; after a discovery, open a direct conversation with an agent about it. | Agents judge the evidence and the messenger's credibility, and can answer questions or objections. |

Observer mode has no statement field. God proclamations are public to all four agents and are tied to an observation in version 1. Messenger testimony is included with the observation; later direct messages go to one selected agent in that observation's thread. The recipient replies and remembers the exchange through a short saved summary. A messenger may follow up within the thread; exact message transcripts are not permanent records. Reopening a thread restores its summary rather than the full conversation. Individual beliefs may change when an agent reflects on that exchange. Shared culture is reconsidered by the next full discovery debate or scheduled council.

The input channel has consequences but does not override an agent's mind. A god can say, “This rain is my blessing,” and the soldier can still argue that it is a flood. Agents distinguish what the player said from what the photo shows. Role permissions are enforced by the server as well as the interface.

## 4. Observation and photo flow

The main web interface offers **Take photo** on devices and browsers that support camera capture and **Upload photo** from a file picker. Laptop and desktop users can upload a photo. Camera access starts only after the player chooses capture. If it is unavailable or denied, upload remains available.

For version 1, accept JPEG, PNG, and WebP images up to 10 MB. Reject unsupported or unreadable files before creating an episode. Do not require GPS or make claims that a photo was taken outdoors based only on metadata. The product encourages real outdoor observations through its prompts and examples.

An active civilization processes one discovery episode at a time. While it is processing, the player may browse the world but cannot start another episode in that civilization. Other civilizations under the account are independent. If the same exact image was previously submitted to this civilization, show the earlier episode and let the player deliberately reuse the image as a new observation if desired.

After upload, one image-capable open-weight model pass produces a grounded, neutral description of visible objects, conditions, and uncertainty. The description is saved as shared evidence and supplied unchanged to all four agents. A role-specific proclamation or testimony is saved separately and attributed to the player.

**Review photo description** is a game setting that is off by default. When on, the player may inspect and correct the factual description before the agents see it. Corrections are labeled as player corrections; interpretation and divine claims belong in the role-specific statement. When review is off, a usable description proceeds automatically.

If the image is too unclear to describe reliably, show the uncertainty and offer **Try another photo** or **Continue with uncertainty**. Agents who receive an uncertain description must not treat an invented object as observed fact. Photos unrelated to a previous suggestion are accepted. Version 1 does not require suggestions to be completed or verify where a photo was taken.

## 5. Founding agents and discovery episode

Every civilization starts with four persistent AI agents. They may use the same Gemma model with different instructions and memories; each remains a separate character and durable workflow. For each reaction, retrieve relevant earlier observations, chronicles, and beliefs from that civilization's history so an agent can connect the new evidence to its past.

| Agent | Main perspective | Lasting contribution |
| --- | --- | --- |
| Priest | Sacred meaning, prophecy, and ritual. | Develops religious explanations and proposes myths or rituals. |
| Scientist | Mechanisms, observations, and uncertainty. | Tests explanations and revises theories. |
| Soldier | Threats, security, and survival. | Frames hazards, defenses, and warnings. |
| Historian | Earlier events and competing accounts. | Tracks continuity and writes the chronicle. |

A discovery episode follows this order:

1. Save the observation and shared photo description.
2. Signal all four agents with the same evidence, their separate memories, and any attributed player statement.
3. Each agent makes one initial reaction. Live previews may show reactions as they become available; individual reactions are temporary workflow data, not permanent game records.
4. Gather the tradition ideas raised in the initial reactions. Give every agent the other reactions and the same candidate list for its one rebuttal.
5. Each agent's rebuttal includes proposed belief revisions and explicit support or opposition for the candidate traditions. Rebuttals are temporary workflow data. A new idea first raised in a rebuttal can be summarized in the chronicle for consideration in a later episode or council.
6. Calculate the proposed cultural outcome using the rule in section 6. The historian determines the leading interpretation from the complete debate and writes the chronicle from the evidence, both phases, final positions, and proposed outcome. Its interpretation outcome is consensus, majority, or unresolved; no player approval is required.
7. Save the chronicle, belief revisions, tradition revisions, and one optional suggestion together as the completed episode. Only then mark the episode complete.

The debate has exactly one reaction and one rebuttal per agent; it does not wait for unanimity or another debate to break a tie. The historian is one of the four agents in both phases and also writes the final chronicle. A round means the complete discovery process, not an individual reaction/rebuttal phase. The chronicle records the leading interpretation or unresolved disagreement, its evidence, important dissent, and what changed in the society. It must link back to the observation. Keep this final record and essential metadata rather than permanent reaction/rebuttal records.

The player can leave or close the browser at any point. Progress is durable, and reopening the world shows the current stage or the completed result. Processing stages shown in the interface are **Describing**, **Reacting**, **Debating**, **Writing the chronicle**, and **Complete**.

## 6. Beliefs and shared culture

Each agent has a separate, persistent belief ledger. A belief has a claim, a qualitative state, and a history of changes. Version 1 uses **forming**, **held**, **questioned**, and **abandoned** as current states. Change events can say **formed**, **strengthened**, **weakened**, **revised**, or **abandoned**. There are no numeric confidence scores.

Each change records which observation, conversation, or scheduled council prompted it and why the agent changed its mind. An agent may keep a belief despite the leading interpretation in a chronicle. Relevant new evidence can change a belief; the app does not force every agent to change after every event. The player can inspect current beliefs and their history for all four agents.

The civilization also has a shared culture ledger for myths, rituals, and taboos. It records the idea, supporting agents, state, origin, and revisions. A candidate idea is assessed after a discovery debate or scheduled council:

- **Adopted:** at least three of the four agents explicitly support it.
- **Contested:** a new candidate has one or two supporters, or an adopted tradition falls to exactly two supporters.
- **Retired:** an existing tradition falls to zero or one supporter after explicit reconsideration.

A new idea with no supporters does not enter the culture ledger; the historian may mention it if relevant, but there is no permanent raw debate record. A contested or retired idea can be adopted again if later evidence brings support to three. Unrelated traditions keep their state until an episode or council explicitly revisits them. Every change is saved rather than overwriting the old history. The historian records a dissenting agent's position even when three agents adopt an idea. This tradition-adoption rule is separate from the historian's judgment of the leading interpretation.

For example, one rain photo might create a contested “Sky Tears” myth. Later observations could persuade three agents to adopt a rain ritual. A later drought could put the ritual in doubt or retire it. The underlying rain observations remain separately identifiable.

## 7. Life between observations

The four agent workflows wait for new observations and remain durable while the player is away. Once a civilization has at least one completed discovery, a scheduled council may run at most once every 24 hours when there is unresolved disagreement, a contested tradition, or a new messenger conversation to consider. If there is nothing meaningful to reconsider, the agents continue waiting without generating filler.

In a council, agents reflect on existing evidence and prior arguments. They may revise their own beliefs and their support for an existing tradition. They cannot claim to have observed a new object, weather event, or species without a player observation. The historian saves a short **Council note** explaining any changes. A council note is part of the history but does not count as a new photo chronicle and does not generate a new outdoor suggestion.

Councils run without an open browser and do not send notifications. A council cannot overlap a discovery episode; an episode takes priority. Once a player chooses to end a civilization, its scheduled councils stop.

## 8. Outdoor suggestions

After each completed photo chronicle, show one optional suggestion inspired by the civilization's current beliefs or traditions. For example: “Find another six-legged creature and see whether it bears the ant's mark.” The suggestion should ask the player to notice something physically observable, not require a particular location or result.

A suggestion is an invitation. It never blocks the player from uploading any other photo, and there is no failure for ignoring it. Suggestions appear in the app after a chronicle; version 1 sends no push notifications or email reminders.

## 9. Ending, archives, and deletion

An active civilization continues until its player chooses **End civilization**. The app explains that this ends the playthrough and asks for confirmation. If a discovery is in progress, ending waits for it to complete or fail visibly before proceeding.

After confirmation, the civilization stops accepting new observations and messages. The historian writes one closing chronicle using the saved history and current beliefs and traditions. Once the closing chronicle is stored, the world becomes an archived, read-only civilization. The player can revisit its photos, belief histories, traditions, conversation summaries, and chronicles, but cannot resume its agents. Raw reactions, rebuttals, and chat transcripts are not archived. The player may create another civilization with a different role.

Ending saves the world; it is separate from deletion. A player can later choose **Delete civilization** from an active world or archive, with a separate confirmation. Deletion stops its workflows and removes its stored records and photos. Account deletion similarly removes the player's civilizations and account data. Signing out does neither.

An ending operation is idempotent: retries cannot create multiple closing chronicles. If the closing chronicle cannot be generated immediately, show an **Ending** state and retry. Do not label the world archived until the final chronicle is saved.

## 10. Main screens and settings

| Screen | What the player can see or do |
| --- | --- |
| Sign in | Create an account or sign in with email/password, Google, or GitHub. Recover account access. |
| Civilization list | Open active worlds or read-only archives; start a new civilization. |
| Create civilization | Preview all three roles, choose one permanently, and name the world. |
| Active world | See the current suggestion, take or upload a photo, watch episode progress, and navigate to agents, culture, and history. |
| Discovery episode | See the photo, shared evidence, saved progress, belief changes, cultural outcome, and final chronicle. Reaction/rebuttal previews may appear live but are not permanent history. |
| Agent view | Read an agent's role, current beliefs, and belief history. Messenger mode also offers a conversation thread tied to a discovery. |
| Culture and history | Inspect adopted, contested, and retired traditions; follow their changes to observations, council notes, and chronicles. |
| Settings | Turn photo-description review on or off, manage the account, and end or delete the civilization. |
| Archive | Read all saved history and the closing chronicle without actions that change the world. |
| Admin portal | Administrators manage initial agent/belief templates, accounts, and games with full application permissions. |

The interface should make the photo action quick. It should remain usable on a phone outdoors and on a laptop. The role-specific controls appear only in the selected role. If the player reloads during an episode, the active screen reconnects to the saved progress.

## 11. Reliability, privacy, and data

The Go API and Go Temporal worker implement the version 1 backend. Temporal manages one durable civilization coordinator and independent durable agent workflows. They wait for observation signals and scheduled councils. Model calls, image processing, and database writes run as retriable activities. Each observation, agent contribution, belief revision, tradition revision, council note, and chronicle has a stable identifier so a retry cannot duplicate it. Commit the chronicle and its belief and culture changes together; a failed episode must not leave a partially changed society. Apply the same rule to a council note and any changes it records.

Persist account permissions, games, initial configuration templates, copied agent configuration, observations and private photo references, agent memory, belief and culture revisions, short conversation summaries, chronicles, optional suggestions, and workflow status. Individual reactions, rebuttals, and chat messages are temporary execution data rather than permanent game tables. Temporal can retain bounded execution payloads for crash recovery; its history retention is separate from game history. Store photos privately, restrict player access to their owner, and avoid using embedded location metadata for gameplay. Administrators have application-wide access. The player can delete photos along with the world. The database and object storage must survive app redeployments.

Use an image-capable Gemma model for shared visual evidence and a Gemma model for agent generation. Temporal provides durable orchestration; Render hosts the web app and worker; Tiger Data stores civilization state and searchable history. Private object storage holds photos. [TECHNOLOGIES.md](./TECHNOLOGIES.md) now selects the specific model and serving API, authentication provider, photo storage, and deployment layout. Sentry Agent Tracing and Entire remain optional additions.

When a model or worker fails, show the episode as still processing or needing attention, retain completed steps, and retry safely. A player may return later. If processing ultimately fails, show a clear retry action without inventing a chronicle or changing beliefs. Unclear images and unsupported uploads use the behavior in section 4.

## 12. Version 1 acceptance scenarios

1. A player signs up with email/password or a social provider, creates an Observer civilization, signs in on another device, and sees the same world.
2. The player submits one photo. All four agents receive the same grounded description, produce different initial reactions and one rebuttal each, and the historian saves a chronicle with the leading interpretation and dissent.
3. A later photo causes at least one agent to recall an earlier event. The player can see before-and-after belief states linked to those events.
4. At least three agent supporters adopt a tradition. If support later falls, its status and history update while dissent remains visible.
5. A God proclamation can be rejected; a Messenger can submit testimony and talk with an agent. An Observer cannot send in-world messages.
6. A photo can be taken on a supported camera device or uploaded from a laptop. Description review is off by default and works when enabled.
7. A world with unresolved tension can produce a scheduled council note while the player is away, without inventing new outside evidence.
8. A worker interruption during an episode does not lose completed progress or create duplicate chronicle records when processing resumes.
9. After a photo chronicle, the app shows one optional outdoor suggestion; an unrelated photo can still be submitted.
10. Ending creates one closing chronicle and a saved read-only archive. Deleting is a separate action.
11. A player cannot read or change another account's civilization or its photos.

## 13. Decision provenance and remaining implementation choices

The project creator chose the three permanent roles and their previews, single-player accounts, both password and social sign-in, the four-agent cast, one reaction and one rebuttal each, the historian's saved chronicle, qualitative evolving beliefs, a three-of-four rule for adopting traditions, camera and upload input, shared photo evidence, optional description review off by default, optional suggestions after chronicles, and player-ended saved archives.

The creator subsequently chose an explicit games table, one immutable player role per game, full account-level Admin permissions, admin-defined starting beliefs copied into new games, retained belief revisions, separate shared-tradition storage, historian-determined verdicts without player approval, and completion even when disagreement remains unresolved. Permanent history excludes raw reactions and rebuttals; Messenger conversations retain short summaries rather than full transcripts. See [DATABASE_DESIGN.md](./DATABASE_DESIGN.md).

Other existing choices are one scheduled council per day only when there is a real unresolved issue; explicit states for traditions as support changes; a proclamation attached to a God observation; Messenger testimony and direct follow-up within a discovery; and retry-or-continue handling for unclear images.

The technology decisions are recorded in [TECHNOLOGIES.md](./TECHNOLOGIES.md). Remaining implementation details include exact interface copy, visual design, resource sizing, and prompt tuning; they do not change the intended user experience.
