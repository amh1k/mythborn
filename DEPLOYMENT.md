# Hosted release setup

`render.yaml` defines the Go API and one Go worker. The frontend is deployed
separately from `web/` on Vercel. The worker needs one always-on instance because
it dispatches the Tiger outbox and polls Temporal. The Blueprint deliberately
leaves credentials as unsynced variables. It does not create a database: Tiger
Cloud remains the application database and Temporal Cloud retains workflow
history.

The root `Dockerfile` runs the Go test suite during the image build, compiles
both Go services, and packages them in a small non-root runtime image. Render
starts the same image with `/mythborn-api` or `/mythborn-worker`. The frontend is
not included in that image.

## Local Docker run

Docker Compose starts the API, worker, and a local Temporal development server;
it does not start the frontend or PostgreSQL. Copy `.env.example` to `.env`, set
the Tiger Cloud and Supabase values, and leave Temporal configured for the
Compose service:

```dotenv
TEMPORAL_ADDRESS=temporal:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_API_KEY=
```

Then run:

```bash
docker compose up --build
```

The API is at `http://localhost:8080`, and the local Temporal UI is at
`http://localhost:8233`. The Temporal development server keeps its local state
in a Docker volume. This is for local development; the Render services should
connect to Temporal Cloud or a separately operated production Temporal service,
not run the development server in the application container. Start the Vite
frontend separately with `cd web && npm ci && npm run dev` when needed.

The `temporal-volume-init` service gives the Temporal user ownership of the
database volume before startup. It exits after preparing permissions; the
Temporal server itself continues to run as the image's non-root user. Keep the
volume when restarting so its workflow history survives.

## Before the first deploy

1. Set up Tiger Cloud PostgreSQL with the database extensions in
   `migrations/001_initial_schema.sql`. Review `DATABASE_DESIGN.md`, then apply
   all Goose migrations from a trusted machine. Do not run migrations from both
   services or during every application start:

   ```bash
   go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
   export PATH="$(go env GOPATH)/bin:$PATH"
   goose -dir migrations postgres "$DATABASE_URL" status
   goose -dir migrations postgres "$DATABASE_URL" up
   goose -dir migrations postgres "$DATABASE_URL" status
   psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f seeds/founding_agents.sql
   ```

2. Create a Temporal Cloud namespace and API key. Set the namespace endpoint,
   namespace name, and API key in each service’s own `sync: false` fields. Do not put these secrets in
   `mythborn-runtime`: Render environment groups cannot request secret values. The clients enable TLS when the API key is
   configured. Local Temporal continues to use an empty API key.
3. Configure Supabase Auth providers and redirect URLs, create the private
   `mythborn-photos` bucket, and set its server-only service role key in Render.
   Configure CORS to the exact Vercel production origin (and preview origin if
   needed). Set the public Supabase URL and publishable key as Vercel environment
   variables.
4. Configure the Google model API key and choose `MODEL_RPM`, `MODEL_INPUT_TPM`, `MODEL_EMBEDDING_RPM`, and
   `MODEL_EMBEDDING_INPUT_TPM` from the actual project/model allowance. Keep one worker
   (`MODEL_WORKER_COUNT=1`) to start. Every replica must receive the same
   declared worker count; changing the replica count requires updating and
   restarting all workers together. The scheduler enforces a per-worker share
   within each process; it is not a distributed coordination service.
5. Create the Render Blueprint, set the service-level `sync: false` secrets in
   the dashboard, and deploy. Set `VITE_API_BASE_URL`, `VITE_SUPABASE_URL`, and
   `VITE_SUPABASE_PUBLISHABLE_KEY` in Vercel for production and preview as
   appropriate. Set the Vercel project root to `web`; `web/vercel.json` routes
   client-side paths back through the SPA entry point. Add both local and hosted
   OAuth callback URLs in Supabase. Run the first-admin SQL statement in
   `README.md` for an account you control.

The API readiness endpoint requires Tiger Cloud access. Confirm the API reports
ready, the worker connects to Temporal and claims an outbox command, and the
browser can sign in before exposing the site to players. Never put service role,
Tiger, Temporal, or model keys in `web/.env.local` or a static-site variable.

## Release checks

CI runs Go tests including SQL integration checks, `go vet`, both binary builds,
the production Docker image build, and the locked frontend production build.
Connect the repository to Render and Vercel to deploy from the chosen branch;
CI validates changes but does not run database migrations or deploy secrets.
A complete release still needs the
real integration walk in `PROJECT_SHORTCOMINGS_AND_STYLE_DIRECTION.md`: upload a
photo, finish a discovery, reload it, submit a related photo, verify actual
retrieval of prior lore, restart a worker during work, and observe one controlled
429. Save the run's elapsed time and the resulting chronicle IDs. Do this against
an isolated test civilization and a controlled provider quota, then delete that
test data afterward.
