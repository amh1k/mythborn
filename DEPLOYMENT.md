# Hosted release setup

## Free deployment without a payment method

`render.yaml` now describes one free `mythborn-backend` web service. Its Docker
image runs the API, worker, and real Temporal 1.32.0 server together. Only the API
binds to the public HTTP port. Temporal listens on loopback and stores execution
and visibility history in two isolated Tiger schemas: `mythborn_temporal` and
`mythborn_temporal_visibility`. Supabase continues to own Auth and private images;
game tables remain in Tiger's original application schema. No additional Render
database, private service, worker plan, or persistent disk is needed.

This is a hobby/demo deployment, with Render's normal free-plan limits. It sleeps
after 15 minutes without inbound traffic; the next request has a cold start.
Rounds and council timers pause while the container is asleep and resume after
waking. Workflow history survives because it is stored in Tiger rather than in
the ephemeral container filesystem. All three processes share 512 MB; the wrapper
sets Go memory targets and restarts the container if an essential process exits.
Normal monthly build, bandwidth, outbound traffic, and instance-hour limits apply.
Do not use external keep-alive pings to pretend this is an always-on service.

The image is built from `deploy/free/Dockerfile`; the standard backend Dockerfile
and local Compose setup are unchanged. Initialize the two Temporal schemas once
from a trusted machine before deployment (the free plan cannot run pre-deploy
jobs). Load the existing `.env` without printing credentials:

```bash
set -a
. ./.env
set +a
docker build -f deploy/free/Dockerfile -t mythborn-free .
python3 deploy/free/database-env.py /tmp/mythborn-temporal.env
docker run --rm --env-file /tmp/mythborn-temporal.env \
  --entrypoint /etc/mythborn/setup.sh mythborn-free
```

Set the usual server-only application values and the SQL values from that private
env file on the Render service. Set `HTTP_ADDRESS=:10000`, `SQL_TLS=true`, and the
exact Vercel domains in `CORS_ALLOWED_ORIGINS`. Never commit the generated env
file or copy database/model/service-role credentials to Vercel. Set Vercel's
public `VITE_API_BASE_URL` to the resulting Render HTTPS URL and redeploy the
frontend. If Render's Blueprint flow requires a payment method, create the web
service directly with plan **Free**, Dockerfile `deploy/free/Dockerfile`, and
health-check path `/readyz`.

Before the hosted service starts, stop the local worker (`docker compose stop
worker`) so workers attached to different Temporal servers cannot claim the same
Tiger outbox. Preserve the local Temporal volume: existing execution checkpoints
are not imported into the new schemas. Completed game history stays available.
A local round already marked `needs_attention` can be retried on hosted Temporal;
its unsaved debate phases must be regenerated. New commands use Signal-With-Start
to create hosted coordinators for existing games.

## Optional paid deployment

`render.paid.yaml` defines the Go API, one Go worker, a private Temporal server,
and a small PostgreSQL instance dedicated to Temporal. The frontend is deployed
separately from `web/` on Vercel. The worker needs one always-on instance because
it dispatches the Tiger outbox and polls Temporal. The Blueprint deliberately
leaves application credentials as unsynced variables. Tiger
Cloud remains the application database. Render Postgres stores only Temporal's
execution and visibility history, in `temporal` and `temporal_visibility` databases.
All four Render resources use Singapore so the API and worker can reach Temporal
over the private network. The initial always-on compute budget is approximately
$45/month ($7 API, $7 worker, $25 Temporal, $6 workflow database), plus PostgreSQL
storage and applicable usage. These plans do not provide high availability.

The root `Dockerfile` runs the Go test suite during the image build, compiles
both Go services, and packages them in a small non-root runtime image. Render
starts the same image with `/mythborn-api` or `/mythborn-worker`. The frontend is
not included in that image.

`deploy/temporal/Dockerfile` pins the real Temporal 1.32.0 server and matching
admin tools by image digest. Its pre-deploy command applies Temporal's own
PostgreSQL schemas idempotently; these are separate from Goose's game migrations.
The server starts all four Temporal services in one process and creates the
`default` namespace with seven-day retention after startup. It uses bounded SQL
connection pools. No public Temporal UI, gRPC endpoint, or database access is
enabled. Trusted services on this Render workspace's private network can access
Temporal without an API key. Never turn this private service into a public web
service without configuring Temporal authentication and TLS.

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
in a Docker volume. This is for local development; Render uses the production
server image with its own PostgreSQL persistence, not the development server.
Start the Vite
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

2. Deploy `mythborn-temporal-db` and `mythborn-temporal` using `render.paid.yaml`. The Blueprint supplies
   the database credentials to Temporal and supplies its private `hostport` to
   the API and worker automatically. Use namespace `default` and an empty
   `TEMPORAL_API_KEY` on this private network. The database's public IP allowlist
   is empty. Temporal's schema setup runs as the private service's pre-deploy
   command, before the server starts.
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

Before starting the Render worker against an existing Tiger database, stop the
local worker: `docker compose stop worker`. Two workers connected to different
Temporal servers must not claim the same Tiger outbox. Local SQLite workflow
histories are not copied to the hosted PostgreSQL server. Completed chronicles
and game records remain in Tiger. Finish or explicitly recover unfinished local
rounds before cutover; new commands start coordinators on the hosted server via
the existing Signal-With-Start dispatcher. Do not delete the local Temporal
volume during this transition.

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
