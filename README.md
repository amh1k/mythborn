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
3. Install the web dependencies and build the Go services:

   ```bash
   cd web && npm install && cd ..
   go build -o bin/mythborn-api ./cmd/api
   go build -o bin/mythborn-worker ./cmd/worker
   ```
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
chronicle. Restarting the worker does not discard a scheduled retry.

## Project guides

- [Local development and Temporal](./LOCAL_DEVELOPMENT.md)
- [Functional specification](./FUNCTIONAL_SPEC.md)
- [Architecture and reliability](./ARCHITECTURE.md)
- [Database design](./DATABASE_DESIGN.md)
- [Technology choices](./TECHNOLOGIES.md)
- [Implementation plan and current status](./IMPLEMENTATION_PLAN.md)
- [API contract](./api/openapi.yaml)
- [Founding agent templates and seed instructions](./seeds/README.md)
