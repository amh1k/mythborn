*This is a submission for the [Hacktoberfest Open-Source AI Challenge Week 1: Touch Grass](https://dev.to/challenges/hacktoberfest-week1-2026-10-05).*

## What I Built

I built **Mythborn**, a photo-driven civilization simulator where something you notice outdoors becomes part of a fictional society's history.

The premise is simple: go for a walk, photograph something that catches your attention, and bring it back to your civilization. A tree, an old structure, or a pattern in the landscape gives its inhabitants something to interpret. Their explanations can change what they believe, introduce a mystery, or influence a shared tradition.

Four AI agents form the civilization's council:

- **The Priest** looks for symbolism, meaning, and the sacred.
- **The Scientist** looks for observable patterns and explanations.
- **The Soldier** considers protection, danger, and practical consequences.
- **The Historian** connects the discovery to previous events and records what the council concludes, including what remains disputed.

Each agent makes an initial interpretation, then responds to the others in a rebuttal round. The Historian writes a chronicle that preserves the consequences of that exchange. Beliefs, mysteries, culture, and history remain available for later discoveries.

Players choose a separate role: **Observer**, **God**, or **Messenger**. That choice changes how the civilization understands their contributions. The Messenger can also speak directly with an agent about a discovery.

Mythborn is for people who enjoy worldbuilding, exploration, and stories that accumulate over time. It gives a short walk a creative purpose: the next chapter starts with something you find beyond the screen. Chronicles can suggest another observation to look for, while players remain free to bring back any photo. There is no mandatory location check.

## Demo

[Try Mythborn live](https://mythborn.vercel.app).

Sign in, found a civilization, choose your player role, and submit a photo from an outdoor discovery. Follow the council's interpretations and rebuttals, then explore the chronicle and the beliefs, mysteries, and traditions that develop around it.

[Watch the 90-second product walkthrough](https://github.com/amh1k/mythborn/raw/refs/heads/main/docs/assets/mythborn-showcase.mp4).

The walkthrough follows the actual interface through the civilization atlas, founding a world, submitting a discovery, the council's interpretations and rebuttals, and the resulting chronicle, beliefs, mysteries, and culture.

The video uses a fictional demo civilization and a bundled watchtower illustration to make the complete journey visible. The live app's frontend is deployed on **Vercel**, and its API, agent worker, and Temporal server run on **Railway**.

The backend's [public health endpoint](https://mythborn-backend-production.up.railway.app/healthz) is available separately. This is a demo deployment on Railway's Free plan, with [limited monthly compute credit](https://railway.com/pricing); availability depends on that allowance.

![The Mythborn civilization page](https://raw.githubusercontent.com/amh1k/mythborn/main/videos/mythborn-showcase/capture-polish/03-world.png)

## Code

[GitHub repository: amh1k/mythborn](https://github.com/amh1k/mythborn)

The repository includes the Go API and worker, React interface, database migrations, founding agent templates, workflow tests, local development instructions, and the HyperFrames video source.

## How I Built It

The frontend uses **React, TypeScript, and Vite** and is served by **Vercel**. A **Go API** handles application requests, while a separate Go worker runs the agents through **Temporal**. The API, worker, and self-hosted Temporal server share one Docker service on **Railway**. **Tiger Data PostgreSQL** stores application state, searchable memories, and Temporal's persistent workflow history in separate schemas. **Supabase Auth and private Storage** handle sign-in and discovery photos.

The services have separate responsibilities: the browser talks to the API, while only the worker coordinates model calls and database changes. Temporal keeps execution history in its own Tiger Data schemas.

![Mythborn service architecture](https://raw.githubusercontent.com/amh1k/mythborn/main/docs/assets/mythborn-architecture.png)

### Gemma: four perspectives on shared evidence

The model adapter uses **Gemma 4**, specifically `gemma-4-26b-a4b-it`, through Google's Gemini API. Google provides [hosted access to this Gemma model](https://ai.google.dev/gemma/docs/core/gemma_on_gemini_api), including image understanding.

Gemma supports the photo-description and interpretation pipeline, the agents' reactions and rebuttals, and the Historian's record. Each agent receives its own personality, beliefs, relevant memories, and the current discovery. Separate founding templates establish those differences before the first round begins.

The prompts require agents to distinguish evidence from interpretation and preserve uncertainty. Model responses use structured JSON; the backend validates proposed changes before saving them. A malformed response receives a bounded repair attempt rather than being written directly into the civilization's history.

### Temporal: keeping a discovery intact through failure

A discovery involves several dependent operations: describing the photo, gathering four reactions, collecting four rebuttals, writing the chronicle, and committing the resulting changes. A failure halfway through that sequence should leave completed work recoverable.

Mythborn uses a **Temporal coordinator workflow for each civilization**, with child workflows for agent phases and activities for model calls, storage, and database operations. The coordinator orders civilization changes and keeps checkpoints for completed contributions. A worker restart can resume an execution from its recorded history.

The hosted deployment runs **Temporal 1.32.0** alongside the API and worker. Temporal listens inside the container, while execution history and visibility records live in the isolated `mythborn_temporal` and `mythborn_temporal_visibility` schemas in Tiger Data. This keeps workflow state outside Railway's temporary filesystem, so a container replacement can recover recorded progress without relying on Temporal Cloud.

The deployed model key is currently using Google AI Studio's free tier, so Gemini quota limits can cause rate limits, especially during bursts of agent calls. Rate limits are handled in the backend: when the model provider returns a quota error, the workflow schedules a durable retry timer and exposes the next retry time to the interface. Successful agents' contributions remain available while the affected operation waits. Retries use backoff and a bounded quota budget; persistent exhaustion pauses the round for attention.

![Temporal rate-limit recovery](https://raw.githubusercontent.com/amh1k/mythborn/main/docs/assets/mythborn-rate-limit-retry.png)

This matters to the story itself: a provider error should remain processing status, rather than becoming an agent's opinion or a fictional unresolved rebuttal.

The application also uses a **transactional outbox** to hand database events to the worker. Final round changes are committed together, with guards against duplicate completion. The repository includes tests for rate-limit recovery, individual checkpoints, and workflow interruptions.

This sequence shows how a discovery moves through the Temporal coordinator. The photo-description and player-review steps are omitted here so the diagram stays focused on the agent debate and durable commit.

![A discovery through Temporal](https://raw.githubusercontent.com/amh1k/mythborn/main/docs/assets/mythborn-temporal-flow.png)

[Explore the workflow implementation](https://github.com/amh1k/mythborn/tree/main/internal/workflows).

### Tiger Data: a civilization that remembers

Tiger Data stores civilizations, agent state, discoveries, chronicles, beliefs, mysteries, and traditions. It also provides the PostgreSQL foundation for the agents' memory retrieval.

Mythborn stores **768-dimensional embeddings in pgvector** and combines vector similarity with PostgreSQL full-text search. Memories can refer to previous observations, chronicles, beliefs, and conversation summaries. Retrieval is scoped to the current civilization and the relevant agent, so another world's history cannot become context for this debate.

Keyword matching helps recover explicit references; vector similarity helps find related meaning when the wording changes. For example, a later discovery near water can retrieve earlier records about a flood or a watchtower. The supplied history gives agents something concrete to reconsider as new evidence arrives.

The current embedding provider is `gemini-embedding-001`. If embedding generation is unavailable, retrieval can fall back to keyword search. Supplemental memory lookup is designed to avoid blocking the entire discovery.

[See the hybrid retrieval query](https://github.com/amh1k/mythborn/blob/main/internal/repository/search.go).

### Deployment: connecting the interface to durable execution

The deployment separates the public interface from the backend processes. Vercel serves the application, and its configured API URL points to Railway over HTTPS. Only the Go API is exposed publicly; the worker connects to Temporal over loopback inside the container. Database, model, and Supabase service-role credentials remain on the backend.

The Docker build runs the Go test suite before compiling the API and worker. During deployment, I verified the public health endpoint, database readiness, Temporal worker startup, authenticated-route protection, and browser CORS. I also redeployed the frontend and checked that its built JavaScript points to the Railway backend.

## Why Does Open Innovation Matter?

Mythborn's characters, memory, and workflow are all visible in the repository. Someone can inspect the founding prompts, revise an agent's assumptions, change how memories are selected, or study the rules that turn a model proposal into a lasting belief.

**Gemma's open weights provide a path toward owning the inference layer.** They make future self-hosting and model experimentation possible beyond a single hosted API. The current implementation uses hosted Gemma and hosted embeddings; local inference would require additional adapter and deployment work.

**Temporal can run locally**, which let me build and exercise durable orchestration without requiring Temporal Cloud. Its workflow model makes retries and recovery explicit parts of the implementation.

The same self-hosting option carries into the live deployment: I can run the Temporal server with my application processes while retaining its history in PostgreSQL. Local development and hosted execution use the same workflow code.

**PostgreSQL and pgvector keep memory accessible through ordinary data and SQL.** The civilization's history can be inspected alongside the retrieval logic that selects it for an agent. Tiger Data supplies the managed database for this implementation.

For Mythborn, openness matters because the system's behavior can be changed at several levels: the model, the character templates, the memory, and the orchestration. The open-weight model and open-source infrastructure give the project room to evolve, while the published implementation lets other builders understand how today's version works.

## My Agent Session

Development was AI-assisted, including work on the backend, workflow recovery, interface, and product video. The repository's [commit history](https://github.com/amh1k/mythborn/commits/main/) documents the resulting changes.

I have not published a DevRelay session archive for this submission.

## Prize Categories

I am entering the following categories from the [Week 1 challenge](https://dev.to/challenges/hacktoberfest-week1-2026-10-05):

- **Best Use of Temporal:** civilization workflows coordinate agent phases, checkpoint completed contributions, retry rate-limited operations with durable timers, and support recovery after worker interruptions. The live backend runs a self-hosted Temporal server with PostgreSQL-backed execution history.
- **Best Use of Tiger Data:** PostgreSQL stores the civilization's persistent state, while pgvector and full-text search provide hybrid retrieval for agent memories. Separate schemas also persist the hosted Temporal server's execution and visibility history.
- **Best Use of Gemma:** Gemma 4 powers the discovery interpretation pipeline, council debate, and Historian's chronicle.
