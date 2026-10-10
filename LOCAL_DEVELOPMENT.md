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

After changing Go source, stop the services, rebuild, and start them again:

```bash
go build -o bin/mythborn-worker ./cmd/worker
go build -o bin/mythborn-api ./cmd/api
python3 scripts/local-dev.py start
```

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

This uses Temporal's development server, suitable for local development and demos.
See the [official CLI guide](https://docs.temporal.io/cli/setup-cli).
