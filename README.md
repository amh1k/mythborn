# Mythborn

Mythborn is a React and Go app where a small cast of characters interprets
outdoor photos, debates their meaning, and records the civilization's history.
The cast is a Priest, Scientist, Soldier, and Historian. Each character has
their own beliefs, and the Historian writes a chronicle after the other three
have responded to the same evidence.

## Run locally

You need Go 1.26+, Node.js with npm, Python 3, a PostgreSQL database on Tiger
Cloud, a Supabase project with Auth and a private photo bucket, a Gemini API
key, and the Temporal CLI. Local development uses Temporal's free development
server; it does not need a Temporal Cloud account. See
[LOCAL_DEVELOPMENT.md](./LOCAL_DEVELOPMENT.md) for the full service and recovery
guide.

1. Copy `.env.example` to `.env` and set `DATABASE_URL`, `SUPABASE_URL`,
   `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_PHOTO_BUCKET`, and `GEMINI_API_KEY`.
   Set Temporal to `TEMPORAL_ADDRESS=localhost:7233`,
   `TEMPORAL_NAMESPACE=default`, and an empty `TEMPORAL_API_KEY`.
2. Create `web/.env.local` with the browser settings for your own project:

   ```dotenv
   VITE_API_BASE_URL=http://localhost:8080
   VITE_SUPABASE_URL=https://your-project.supabase.co
   VITE_SUPABASE_PUBLISHABLE_KEY=your-public-publishable-key
   ```

   Keep service role keys and the Gemini key in the root `.env`; browser
   settings must contain only the public Supabase key.
3. Install the Temporal CLI into `bin/temporal` using the steps in [LOCAL_DEVELOPMENT.md](./LOCAL_DEVELOPMENT.md). Then install the frontend dependency lock and build the Go services:

   ```bash
   go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
   export PATH="$(go env GOPATH)/bin:$PATH"
   set -a
   . ./.env
   set +a
   goose -dir migrations postgres "$DATABASE_URL" up
   psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f seeds/founding_agents.sql
   cd web && npm ci && cd ..
   go build -o bin/mythborn-api ./cmd/api
   go build -o bin/mythborn-worker ./cmd/worker
   ```

   Encode special characters in the Tiger password as URL percent escapes
   inside `DATABASE_URL`. Since this setup sources `.env`, quote values that
   contain shell metacharacters. Apply migrations only to the intended database.
   After signing in once and obtaining the account UUID from Supabase Auth,
   bootstrap the first admin in Tiger Cloud SQL Editor or with `psql`:

   ```sql
   INSERT INTO accounts (id, system_role)
   VALUES ('YOUR_SUPABASE_AUTH_USER_UUID', 'admin')
   ON CONFLICT (id) DO UPDATE SET system_role = 'admin';
   ```

   Run this once for an account you control. The UUID is the Auth user's `id`,
   not their email. Account UUIDs and game data are stored in Tiger Cloud.
4. Start the app, API, worker, and local Temporal server:

   ```bash
   python3 scripts/local-dev.py start
   ```

   Open <http://localhost:5173>. Run `python3 scripts/local-dev.py status` to
   check services and `python3 scripts/local-dev.py stop` to stop the managed
   local processes. After backend code changes, rebuild the binaries and run
   `python3 scripts/local-dev.py restart-backend`; this keeps Temporal's stored
   workflow history and timers running.

The local helper stores logs and Temporal's persistent database in
`bin/local-dev/`. Keep that directory when restarting if you want to resume
existing workflow history.

## Rate limits and episode progress

When Gemini returns HTTP 429, Temporal waits at least one minute, or longer if
the provider requests it, then retries the affected call automatically. The
episode shows the retry countdown. Responses already received stay visible and
are reused, and the Historian waits for all four agents before writing the
chronicle. Automatic recovery pauses after eight attempts or 45 minutes; retry
the saved stage later, or set the round aside. Deletion and end-game commands can
interrupt an active quota wait. Restarting the worker does not discard a
scheduled retry.

## Current acceptance checks

`go test ./...` runs deterministic model, workflow, HTTP, and image checks.
The repository integration test also verifies migrations, belief revision
attribution, abandoned-round protection, and closing-round retry when run against
a disposable PostgreSQL database with pgvector migrations applied. A browser
check uses mock service data. The real Supabase/Tiger/Gemini discovery and second
photo memory-retrieval run is still outstanding.

## Project guides

- [Hosted deployment](./DEPLOYMENT.md)
- [Local development and Temporal](./LOCAL_DEVELOPMENT.md)
- [Functional specification](./FUNCTIONAL_SPEC.md)
- [Architecture and reliability](./ARCHITECTURE.md)
- [Database design](./DATABASE_DESIGN.md)
- [Technology choices](./TECHNOLOGIES.md)
- [Implementation plan and current status](./IMPLEMENTATION_PLAN.md)
- [API contract](./api/openapi.yaml)
- [Founding agent templates and seed instructions](./seeds/README.md)
