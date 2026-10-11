#!/bin/sh
set -eu
: "${SQL_HOST:?SQL_HOST is required}"
: "${SQL_USER:?SQL_USER is required}"
: "${SQL_PASSWORD:?SQL_PASSWORD is required}"
export TEMPORAL_BROADCAST_ADDRESS="${TEMPORAL_BROADCAST_ADDRESS:-$(getent hosts "$(hostname)" | awk 'NR==1 {print $1}')}"
: "${TEMPORAL_BROADCAST_ADDRESS:?Cannot resolve the container address}"

# Create the namespace after the real server becomes healthy. The foreground
# server receives Render's SIGTERM directly; workflows persist in Postgres.
(
  attempts=0
  until temporal operator cluster health --address 127.0.0.1:7233 >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 180 ]; then
      echo 'Temporal namespace setup timed out' >&2
      exit 1
    fi
    sleep 1
  done
  if ! temporal operator namespace describe --address 127.0.0.1:7233 --namespace default >/dev/null 2>&1; then
    temporal operator namespace create --address 127.0.0.1:7233 --namespace default --retention 168h
  fi
) &

# No public ingress exists for this service; only trusted Render services connect.
exec temporal-server --allow-no-auth start
