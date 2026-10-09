# Mythborn: Technology Stack

Status: **technology decision for version 1; no integration is implemented yet.** The [functional specification](./FUNCTIONAL_SPEC.md) defines product behavior. This document fixes the stack; [ARCHITECTURE.md](./ARCHITECTURE.md) details service boundaries, data flows, workflows, and failure handling.

## The idea

A society of tiny AI agents lives inside the application. They interpret the user's outdoor observations as supernatural events:

- Rain becomes the origin of a new mythology.
- An ant becomes a terrifying newly discovered species.
- A traffic light inspires a religion around three sacred colors.

Each character maintains its own perspective, reacts to discoveries, debates other characters, and contributes to an evolving civilization chronicle.

The main demo: submit one outdoor photo, watch several agents react differently, and see their debate produce a new chapter in the civilization's history.

## Chosen stack

| Layer | Choice | Responsibility |
| --- | --- | --- |
| Browser app | **React + TypeScript + Vite**, plain CSS, React Router, TanStack Query | Mobile-friendly camera/upload UI, authenticated pages, and polling episode progress |
| API | **Go 1.26+ + `net/http` ServeMux** | Authorization, upload validation, world and episode endpoints, photo access, and workflow commands through the database outbox |
| Workflow runtime | **Temporal Go SDK + Temporal Cloud** | Durable civilization coordinator, four persistent agent workflows, retries, and council timers |
| Deployment | **Render Static Site + paid Web Service + paid Background Worker** | Host the agent-facing React frontend, run the Go API, and execute the four agents in an always-on Temporal worker |
| Generative model | **Gemma 4 `gemma-4-26b-a4b-it` via the Gemini API**, using `google.golang.org/genai` in the worker | One image description; four character reactions and rebuttals; chronicle, council note, and suggestion |
| Memory embeddings | **`gemini-embedding-001` via the Gemini API**, 768 dimensions | Embed saved lore and retrieval queries for semantic recall |
| Application database | **Tiger Cloud PostgreSQL 17+**, `pgx/v5` with `pgxpool`, and Goose SQL migrations | Canonical account-linked worlds, observations, reactions, beliefs, traditions, conversations, chronicles, and outbox |
| Memory search | **Tiger Data `pg_textsearch` + `pgvectorscale`** | BM25 keyword and vector retrieval over prior evidence and lore |
| Account identity | **Supabase Auth**, `@supabase/supabase-js` | Email/password, Google, GitHub, recovery, and session JWTs |
| Photo objects | **Supabase Storage private bucket** | Durable image files, with owner-gated access and short-lived signed read URLs |

Use **Sentry Agent Tracing** after the core loop works, and **Entire** to document meaningful development sessions if useful for the submission. **Backboard** stays deferred; Tiger Data is the one source of truth for memory. No Redis, separate vector database, agent framework, or self-hosted model server is needed for version 1.

Go is the backend language for **both** the HTTP API and Temporal worker. Use the standard library's method-aware routes and typed request/response structs. `pgxpool` handles database connections; SQL queries stay explicit so Tiger's BM25/vector operators are easy to use. Goose runs versioned SQL migrations. The worker calls Gemma and the embedding model through Google's official Go SDK. [Go routing](https://go.dev/blog/routing-enhancements), [Temporal Go SDK](https://docs.temporal.io/develop/go), [pgx](https://github.com/jackc/pgx), [Goose](https://github.com/pressly/goose), and [Google GenAI Go SDK](https://ai.google.dev/gemini-api/docs/libraries).

This is a pragmatic hackathon deployment. Google documents hosted Gemma 4 text and image calls, including this exact model ID. It is an open-weight *model* accessed through a hosted API here; the app is **not self-hosting** its weights. [Google's Gemma API guide](https://ai.google.dev/gemma/docs/core/gemma_on_gemini_api) and [Gemma 4 model card](https://ai.google.dev/gemma/docs/core/model_card_4) support this choice.

## Request and deployment shape

```text
React static site ──JWT/API──> Go API ──SQL──> Tiger Cloud
       │                         │                 ↑
       └──sign in────────────> Supabase Auth      │
                                 └──private photos──> Supabase Storage
                                                     ↑
                                      Render worker ─┘
                                           │
                                           ├──claim outbox / SQL──> Tiger Cloud
                                           ├──start / signal / tasks──> Temporal Cloud
                                           ├──read / write photos──> Supabase Storage
                                           └──Gemma 4 / embeddings──> Google API
```

**Why both Tiger Data and Supabase?** Tiger Cloud is the managed PostgreSQL database for all Mythborn game data and memory search. It does not replace the application's sign-in flow or private photo-file service. Supabase supplies those two features: Auth handles password, Google, and GitHub sessions; Storage holds image objects outside the game database. A Supabase project does include its own internal Postgres for Auth and Storage metadata, so there are technically two managed databases, but Mythborn creates **no game tables** there. This avoids building password/social-login infrastructure or storing large image bytes in Tiger Data. [Tiger Cloud essentials](https://www.tigerdata.com/docs/learn/tiger-cloud/tiger-cloud-essentials), [Supabase Auth architecture](https://supabase.com/docs/guides/auth/architecture), and [Supabase Storage quickstart](https://supabase.com/docs/guides/storage/quickstart).

The frontend talks to Supabase only for authentication. Go middleware validates each JWT against Supabase's signing keys (using a cached JWKS and `jwx/v4`) and checks the authenticated account owner on every Tiger Data query; account-level administrators receive the separately defined application-wide access. Verify signature, expected algorithm, issuer, audience, and expiry before trusting the `sub` claim. The Supabase service-role credential stays server-side. The browser never receives Temporal, database, model, or service-role secrets. The `jwx/v4` dependency requires Go 1.26+. [Supabase Auth](https://supabase.com/docs/guides/auth), [JWT signing keys](https://supabase.com/docs/guides/auth/signing-keys), [jwx JWT/JWKS documentation](https://pkg.go.dev/github.com/lestrrat-go/jwx/v4/jwt), [jwx v4 compatibility notes](https://github.com/lestrrat-go/jwx/blob/develop/v4/Changes-v4.md), [Google sign-in](https://supabase.com/docs/guides/auth/social-login/auth-google), and [GitHub sign-in](https://supabase.com/docs/guides/auth/social-login/auth-github) cover the required account methods.

The API accepts a photo, checks its decoded type, file size, and pixel count, then decodes and re-encodes it to strip EXIF metadata (including possible location) before storing it in a private bucket. Go's standard image packages handle JPEG/PNG; `golang.org/x/image/webp` decodes WebP before normalizing it to JPEG/PNG. It stores only the object path in Tiger Data. Use the Supabase Storage HTTP API from Go; the browser gets a short-lived photo URL only after an ownership check. Deletion removes both the database records and objects. [Supabase private bucket docs](https://supabase.com/docs/guides/storage/buckets/fundamentals).

Deploy three Render services from one repository: a static site for `web/`, a paid web service running `cmd/api`, and a paid background worker running `cmd/worker`. The two Go binaries build from one `go.mod` and share `internal/` workflow payloads, database code, and model clients. Use Temporal Cloud rather than deploying a Temporal server/database on Render. **A free Render web service sleeps when idle, and Render does not offer a free background worker**; the project's Hacktoberfest $50 Render credit makes paid compute the intended demo setup. Render's free static site is suitable for the browser assets. [Render service types](https://render.com/docs/service-types), [free-service limits](https://render.com/docs/free), and [compute plans](https://render.com/docs/compute-plans).

The challenge's Render category explicitly allows either using Render as an **AI runtime** or **hosting an agent's frontend**. Mythborn clearly matches the frontend option: players interact with the civilization agents through a React app hosted on Render. The Go background worker also executes agent orchestration and model calls on Render. Temporal Cloud persists workflow state; Google's API performs Gemma inference. Describe those boundaries accurately in the submission and show the deployed URL plus Render API/worker services or logs. Do **not** claim that Gemma model weights run on Render. [Official Render prize-category wording](https://dev.to/challenges/hf26#best-use-of-render) and [Render background workers](https://render.com/docs/background-workers).

For the credit budget, start the API on `0.5c-512mb` (about **$7/month**) and the worker on `1c-2g` (about **$25/month**), with one instance each and a free static site. That is roughly **$32 for 30 days of continuous compute**, leaving about $18 of the $50 credit for overage or a brief upgrade. The worker gets 2 GB to leave room for image decoding, workflow tasks, and concurrent model calls; it does **not** run Gemma weights. Keep this sizing until actual memory use is measured, then adjust if needed. Render bills compute proportionally to runtime; confirm that the promotional credit has been applied and covers these service charges before deployment. This credit does not pay Temporal Cloud, Tiger Cloud, Supabase, or any model-provider charges. [Render pricing](https://render.com/pricing) and [Render's September 2026 deployment cost example](https://render.com/articles/production-rails-hosting-guide).

## Data and workflow contracts

Use UUIDs throughout. [DATABASE_DESIGN.md](./DATABASE_DESIGN.md) defines the proposed tables: `accounts`, `games`, `agent_templates`, `initial_belief_templates`, `agents`, `observations`, `rounds`, `chronicles`, `beliefs`, `belief_revisions`, `traditions`, `tradition_supporters`, `tradition_revisions`, `conversation_summaries`, `search_documents`, `workflow_outbox`, and `account_deletion_jobs`. Game-owned records carry or join to `game_id`; player access is scoped by the authenticated owner, while account-level administrators have full application access. The Supabase Auth user UUID identifies the local application account, with no local password table. Game roles are immutable and separate from Admin permission. Starting configuration and beliefs are copied from admin-defined templates on game creation.

An observation has a photo object path, source-byte hash, shared visual description, attributed player statement, and optional correction; evidence is immutable after acceptance. A round has a state and stage. Individual reactions/rebuttals are bounded temporary workflow data, not permanent game records. Persist progress and the final historian verdict with essential metadata. Belief and tradition revisions are append-only history. The final chronicle, proposed revisions, current-state updates, and optional suggestion commit in **one PostgreSQL transaction**; a failed generation leaves the round incomplete without partially advancing society. Council notes and closing records also use `chronicles`, distinguished by round kind; suggestions are optional chronicle fields. Enforce one chronicle per round and stable revision event keys so retries cannot duplicate outputs.

The API writes a pending command to `workflow_outbox` in the same transaction as the observation/ending/deletion record. The worker claims and dispatches it to Temporal, then marks delivery; processing is marked separately with the game-state transaction that completes the command. Retries use stable workflow and event IDs. This closes the database-commit/Temporal-signal gap if the API crashes between those steps. A civilization coordinator workflow owns the episode queue and daily council timer. Four separately addressable agent workflows hold their own waiting state; they receive the shared evidence, produce initial responses, then rebuttals. External model, storage, embedding, and SQL work happens in Temporal **activities**, with bounded retries and idempotent writes. Workflow code uses Temporal's deterministic primitives for waits and concurrency; ordinary Go network calls and goroutines stay outside workflow logic. Long-lived workflows use Continue-As-New periodically to bound history size. [Temporal Go SDK guide](https://docs.temporal.io/develop/go).

The UI uses `GET /episodes/{id}` polling while an episode is active; no WebSocket or SSE infrastructure is needed for the first demo. The minimum API surface is authentication-aware world list/create/get/end/delete, observation upload, episode detail/progress, agent belief history, culture/history, Messenger conversation, and the photo URL endpoint. Mutations take idempotency keys where repeat submission could create duplicate episodes.

## Memory and model behavior

Store bounded searchable text for committed observations, chronicles including council notes, current beliefs, and relevant Messenger conversation summaries. Raw reactions, rebuttals, and messages are not indexed or kept as permanent history. Generate a 768-dimensional embedding for each searchable item with `gemini-embedding-001`. Before an agent responds, query only that game's records and respect agent-private memory scope; combine BM25 matches and vector neighbors, then include a small, ordered set of relevant memories plus the agent's current belief ledger. Keep photo description and player statement in separate fields so a proclamation is never presented as observed fact. The embedding API is an auxiliary retrieval tool; **Gemma 4 generates all in-world agent behavior**. [Google embeddings guide](https://ai.google.dev/gemini-api/docs/embeddings) and [Tiger Data hybrid search guide](https://www.tigerdata.com/docs/learn/tutorials/hybrid-search).

Use one shared, neutral Gemma image description per observation, then run four initial reactions concurrently and four rebuttals concurrently. Decode model JSON into typed Go structs and validate required fields/enums; on malformed output, retry the activity with an error-specific repair prompt. The historian determines consensus, majority, or unresolved interpretation from the evidence and temporary debate, and writes the final record without player approval. Unresolved disagreement completes the round. Deterministic Go code separately computes the three-of-four tradition rule. Messenger replies and councils reuse the same model and memory retrieval path; only short rolling conversation summaries and resulting belief changes are permanent. Keep model/prompt versions in bounded final-record metadata. Configure workflow-history retention separately from permanent game storage.

The Gemini API's current Gemma 4 entry is **free-tier only**, with model usage limits and no paid tier shown; its pricing page says free-tier content may be used to improve Google's products. This matters for user photos: disclose external AI processing before upload and avoid promising that a privately stored photo is processed only by Mythborn. Check actual rate limits in AI Studio before a live demo and keep sample inputs ready. If the endpoint proves too constrained, move the same model behind a dedicated hosted Gemma endpoint; that changes infrastructure and cost, not app behavior. [Gemini API pricing](https://ai.google.dev/gemini-api/docs/pricing) and [Gemma API guide](https://ai.google.dev/gemma/docs/core/gemma_on_gemini_api).

## Build order and readiness

1. Scaffold `web/` and a Go module with `cmd/api`, `cmd/worker`, `internal/`, and `migrations/`; run local PostgreSQL 17 with Tiger extensions and a local Temporal dev server.
2. Implement Supabase sign-in and server ownership checks; create civilizations with immutable player roles.
3. Build private photo upload, neutral Gemma description, pending episode row, and outbox dispatch.
4. Implement four agent workflows, reaction/rebuttal UI polling, atomic chronicle commit, and belief/tradition history.
5. Add embedding-backed memory, Messenger conversations, daily councils, archives, and deletion.
6. Deploy the static site, API, and always-on worker; rehearse a real two-photo demo and a worker restart.

Set Render secrets for `DATABASE_URL`, `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`, Temporal Cloud credentials, `GEMINI_API_KEY`, `SUPABASE_URL`, and a server-only Supabase key. Expose only the Supabase public URL/key and API origin to the browser. Configure exact frontend/API CORS origins and OAuth redirect URLs for both local and deployed domains. Do not put secrets or real photos in the repository.

Temporal Cloud currently advertises trial credits, while Tiger Cloud, Supabase usage, and model rate limits have separate terms. The user's $50 Hacktoberfest Render credit is a deployment budget, not a permanent free tier. Check the account dashboards before committing spend. [Temporal Cloud pricing](https://temporal.io/pricing), [Render pricing](https://render.com/pricing), and [Supabase pricing](https://supabase.com/pricing).

## How each partner fits

### Render: the agent app and runtime

Render serves the player-facing React app, runs the Go API that accepts discoveries, and runs the always-on Go Temporal worker where the four agent workflows execute their activities. A live photo-to-chronicle demonstration from the Render URL, backed by visible worker logs, is the concrete Render integration. Model inference remains on the hosted Gemma API.

### Temporal: a civilization that persists

Give civilization characters independent durable workflows. They wait for new observations, perform model calls through activities, exchange reactions, and periodically evolve their society.

Keep model calls and external database writes in activities. Use stable observation identifiers and idempotent writes so retries do not produce duplicate discoveries or chronicles.

Demo opportunity: interrupt a worker during a discovery, restart it, and show the civilization continuing from its recorded progress.

### Gemma: the open-weight AI core

Use the selected Gemma 4 model to turn a photo into a grounded observation. Pass that observation to characters with different instructions and relevant memories.

The fixed founding cast:

- **Priest:** interprets discoveries as signs and prophecies.
- **Scientist:** proposes explanations and challenges existing beliefs.
- **Soldier:** evaluates danger and proposes defenses.
- **Historian:** connects discoveries to earlier events and writes the chronicle.

An open-weight model should drive the actual experience. Explain in the submission how model choice, self-hosting, or modifying agent behavior benefits the project.

### Tiger Data: history that changes future reactions

Store observations, character beliefs and revisions, traditions, short conversation summaries, and final chronicles in PostgreSQL. Raw debates are temporary workflow data. Retrieve relevant earlier events through keyword and vector search before a character reacts. Scope retrieval by game and character when memories should be private.

Example: after an earlier rain observation, a new puddle photo causes the historian to recall the rain chronicle:

> “The sky's tears have gathered into a sacred mirror. The priest's prophecy was true.”

This makes the society evolve across observations rather than generating unrelated stories for each upload.

### Backboard: a deferred memory alternative

Backboard is outside the first-version stack. Consider it later if managed character memory proves more useful than retrieval implemented on Tiger Data.

Create separate assistants for the priest, scientist, soldier, and historian. Backboard memory is shared across threads belonging to the same assistant, so separate assistants provide the useful boundary for independent character memories. Scope assistants by civilization as well when supporting multiple users.

Example: the priest remembers rain as a blessing, while the soldier remembers it as a catastrophic flood.

Tiger Data holds the first-version canonical history and agent memory. A later Backboard integration would need a distinct responsibility, such as managed character recall, to avoid maintaining two competing memory sources.

### Sentry Agent Tracing: evidence of agent behavior

Instrument the discovery pipeline:

`Photo analysis → character reactions → debate → beliefs and traditions → chronicle`

Record useful timing, model usage, and failure information. Include trace screenshots and a real debugging finding in the submission, such as a slow historian call or a recovered model failure.

### Entire: development documentation

Capture coding-agent sessions during development and share a meaningful session in the write-up. Good examples include implementing durable debates, fixing inconsistent character behavior, or explaining an architectural decision.

Entire documents how the application was built; it does not provide memory for the in-app civilization agents.

## Partner category strategy

**Tiger Data and Entire** are candidates for potentially overlooked categories in the first-version plan. Backboard remains a possible later category only if it is meaningfully integrated. This is a strategic hypothesis, not a verified competition ranking. Reliable entry counts by partner were not established during the initial research.

Prioritize meaningful integrations that can be demonstrated clearly. A project can enter multiple categories it genuinely uses, but can win only once per challenge.

Other options to defer:

- **Tinker:** consider later if there is a useful training dataset and time to fine-tune. Its category requires a demonstrated improvement over a baseline.
- **Arduino:** requires an Arduino UNO Q and adds hardware scope.
- **TabPFN:** fits a real tabular prediction feature better than the initial photo-to-mythology loop.
- **Mastra:** consider if agent tools or memory justify another framework alongside Temporal.
- **ElevenLabs:** optional spoken chronicles could improve the demo after the main experience works.

## Make the outdoor loop central

Have the civilization optionally suggest discoveries that motivate a walk:

> “Find evidence of a six-legged beast.”

The user may follow that suggestion or submit any outdoor photo. They collect the photo, put the phone away, and return to see the civilization interpret it. Keep outdoor capture brief; let the longer debate and chronicle experience happen afterward.

For the submission, demonstrate:

1. The live agent frontend on Render receiving a real outdoor observation.
2. Different character reactions to the same photo.
3. A debate that produces a chronicle.
4. A later observation that recalls earlier history.
5. Temporal recovery after a worker interruption, if practical.

Protect time for a clear write-up: writing quality is the challenge's most heavily weighted judging criterion.

## References

- [Week 1 challenge announcement and judging criteria](https://dev.to/devteam/join-the-hacktoberfest-open-source-ai-challenge-week-1-touch-grass-2450-in-prizes-across-17-4pom)
- [Week 1 challenge requirements](https://dev.to/challenges/hacktoberfest-week1-2026-10-05)
- [Partner categories and requirements](https://dev.to/challenges/hf26)
- [Temporal documentation](https://docs.temporal.io/)
- [Render documentation](https://render.com/docs)
- [Gemma model documentation](https://ai.google.dev/gemma/docs/core)
- [Tiger Data hybrid search documentation](https://www.tigerdata.com/docs/learn/tutorials/hybrid-search)
- [Backboard memory documentation](https://docs.backboard.io/concepts/memory)
- [Backboard model access FAQ](https://docs.backboard.io/faq)
- [Sentry Agent Tracing category](https://dev.to/challenges/hf26#best-use-of-sentry-agent-tracing)
- [Entire documentation](https://docs.entire.io/)

Partner requirements, service capabilities, and contest categories can change. Recheck the official challenge page before submitting.
