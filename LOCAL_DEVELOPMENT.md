# Local Temporal and Mythborn

## Start only the backend

With Docker available and the root `.env` configured, run from the repository
root:

```bash
make api
```

This builds the backend image and starts only the HTTP API at
`http://localhost:8080`. It uses your existing Tiger Cloud and Supabase settings.
It does not start Temporal or the frontend. Saved data and CRUD endpoints work
without Temporal; executing agent rounds and reading live debate previews need
an available Temporal server and worker.

To start both the API and the worker against an already running Temporal server:

```bash
make backend
```

The backend-only Compose file preserves the Temporal settings from `.env`.
When Temporal runs on your host machine, use
`TEMPORAL_ADDRESS=host.docker.internal:7233` and ensure the server listens on an
address accessible to containers. `localhost` inside a container refers to that
container. For a hosted server, use its reachable endpoint instead.

Logs stay in the terminal. Press Ctrl+C to stop, or run `make backend-stop` from
another terminal. Without Make, the API-only command is
`docker compose -f docker-compose.backend.yml up --build api`.

## Start the complete local stack

Temporal runs locally without a cloud account or API key. The CLI is installed
at `bin/temporal`. Mythborn's compiled worker and API are `bin/mythborn-worker`
and `bin/mythborn-api`.

The root `.env` uses:

```dotenv
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_API_KEY=
```

It also needs the existing Tiger Data, Supabase, and Google API configuration.
Browser configuration is read by Vite from `web/.env.local`.

The API also uses the same `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`, and optional
`TEMPORAL_API_KEY` as the worker to read live debate previews. It connects lazily:
if Temporal is unavailable, the discovery screen reconnects its live view while
saved progress and chronicles remain readable. After changing either backend,
rebuild and restart the API and worker using the commands below.

From the repository root:

```bash
python3 scripts/local-dev.py start
python3 scripts/local-dev.py status
python3 scripts/local-dev.py stop
```

`start` launches Temporal, the Go worker, the Go API, and Vite in the background.
It reuses processes already started by this helper and recognizes healthy API
and Mythborn web services started separately. It checks HTTP readiness and refuses
to take over other occupied ports. `stop` only stops the processes recorded by
this helper, checking their process identity before signaling them. Services
started separately keep running and must be stopped from their original terminal.

- App: http://localhost:5173
- API health: http://localhost:8080/healthz
- API database readiness: http://localhost:8080/readyz
- Temporal UI: http://localhost:8233
- Temporal gRPC: localhost:7233
- Namespace: `default`
- Worker task queue: `mythborn-workers`

Logs, process metadata, and the persistent Temporal database are stored under
`bin/local-dev/`, which is excluded from Git. Starting again with the same
database preserves workflow history. Keep this directory when restarting.
Workflows and activities only execute while the local server and worker run.

After changing Go source, rebuild and reload the backend while Temporal and Vite
keep running:

```bash
go build -o bin/mythborn-worker ./cmd/worker
go build -o bin/mythborn-api ./cmd/api
python3 scripts/local-dev.py restart-backend
```

`restart-backend` also replaces an API launched with `go run` from this repository
after verifying its executable and working directory. It refuses to stop an
unrelated process on port 8080.

Check the Temporal server and registered worker pollers:

```bash
./bin/temporal operator cluster health --address localhost:7233
./bin/temporal task-queue describe --address localhost:7233 \
  --namespace default --task-queue mythborn-workers
```

Creating a civilization through the app starts its coordinator workflow. In the
current implementation, the four agents run as child workflows during discovery
reaction and rebuttal phases, so an idle new civilization shows only its
coordinator in the running-workflow list. Completing a discovery additionally
requires valid model credentials, photo storage, and the existing application
schema/templates in Tiger Data.

Model HTTP 429 responses automatically wait at least one minute, or longer when
the provider requests it. The discovery page shows the retry countdown for the
affected phase or agent. Completed responses stay visible and are reused; the
historian waits for every required contribution. These waits use persistent
Temporal timers, so restarting the worker resumes the same discovery. Other
processing errors retain their bounded retries and manual retry action.

This uses Temporal's development server, suitable for local development and demos.
See the [official CLI guide](https://docs.temporal.io/cli/setup-cli).

## Install the local Temporal CLI

The development helper expects the CLI at `bin/temporal`. On Linux, download the
Linux amd64 or arm64 archive linked from [Temporal's official CLI installation
guide](https://docs.temporal.io/cli/setup-cli), extract it, and place the
`temporal` executable here:

```bash
mkdir -p bin
# Move the extracted temporal executable into bin/temporal.
chmod +x bin/temporal
bin/temporal --version
```

Keep the CLI binary in `bin/`; that folder is ignored by Git. Use the same
architecture as the machine running Mythborn. The first run starts Temporal's
local development service with persistent state under `bin/local-dev/`.

## Enable gameplay continuity after updating the code

Apply the new migrations to your application database using the existing quoted `DATABASE_URL`, then rebuild the API and worker:

```bash
set -a
. ./.env
set +a
goose -dir migrations postgres "$DATABASE_URL" up
docker compose up -d --build api worker
```

The existing local Temporal service must be healthy. Restarting the frontend dev server is only necessary if it was stopped. The overview now shows mysteries, agent details show relationships, and culture records show their later influence. Old histories remain readable; new gameplay records grow from subsequent discoveries.

The three `GAMEPLAY_*_ENABLED` controls in `.env.example` default to true. To run database integration tests, use a disposable migrated database with `MYTHBORN_TEST_DATABASE_URL`; never point those tests at your application database. An optional provider check is available with `MYTHBORN_LIVE_GAMEPLAY_TEST=true go test ./internal/activities -run '^TestLiveGameplayRound$' -timeout 7m -v`; it uses fictional evidence and generation calls, and does not write game records.
