# Local Temporal and Mythborn

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
