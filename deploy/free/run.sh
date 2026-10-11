#!/bin/sh
set -eu
export TEMPORAL_ADDRESS=127.0.0.1:7233 TEMPORAL_NAMESPACE=default TEMPORAL_API_KEY=''
export TEMPORAL_BIND_IP=127.0.0.1 TEMPORAL_BROADCAST_ADDRESS=127.0.0.1
export TEMPORAL_HISTORY_SHARDS=16
export SQL_HISTORY_SCHEMA=mythborn_temporal SQL_VISIBILITY_SCHEMA=mythborn_temporal_visibility
export SQL_VISIBILITY_DATABASE="${SQL_DATABASE:?SQL_DATABASE is required}"
export GOMAXPROCS=1
temporal_pid='' worker_pid='' api_pid=''
shutdown() {
  trap - EXIT INT TERM
  # Stop accepting work before stopping the persistence coordinator.
  for pid in "$api_pid" "$worker_pid"; do
    if [ -n "$pid" ]; then kill -TERM "$pid" 2>/dev/null || true; fi
  done
  for pid in "$api_pid" "$worker_pid"; do
    if [ -n "$pid" ]; then wait "$pid" 2>/dev/null || true; fi
  done
  if [ -n "$temporal_pid" ]; then
    kill -TERM "$temporal_pid" 2>/dev/null || true
    wait "$temporal_pid" 2>/dev/null || true
  fi
}
trap shutdown EXIT
trap 'exit 0' INT TERM

GOMEMLIMIT=200MiB /etc/mythborn/start.sh &
temporal_pid=$!
attempts=0
until temporal operator namespace describe --address 127.0.0.1:7233 --namespace default >/dev/null 2>&1; do
  kill -0 "$temporal_pid" 2>/dev/null || exit 1
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 180 ]; then echo 'Temporal startup timed out' >&2; exit 1; fi
  sleep 1
done
GOMEMLIMIT=100MiB /mythborn-worker &
worker_pid=$!
GOMEMLIMIT=80MiB /mythborn-api &
api_pid=$!

# Restart the whole service if any essential process exits. BusyBox supports
# wait -n, and the EXIT trap forwards shutdown to every remaining process.
set +e
wait -n "$temporal_pid" "$worker_pid" "$api_pid"
result=$?
if [ "$result" -eq 0 ]; then result=1; fi
exit "$result"
