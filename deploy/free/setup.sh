#!/bin/sh
set -eu
: "${SQL_HOST:?SQL_HOST is required}"
: "${SQL_USER:?SQL_USER is required}"
: "${SQL_PASSWORD:?SQL_PASSWORD is required}"
: "${SQL_DATABASE:?SQL_DATABASE is required}"
export SQL_PORT="${SQL_PORT:-5432}" SQL_PLUGIN=postgres12
export PGHOST="$SQL_HOST" PGPORT="$SQL_PORT" PGUSER="$SQL_USER" PGPASSWORD="$SQL_PASSWORD" PGDATABASE="$SQL_DATABASE"
if [ -n "${SQL_TLS_CA_BASE64:-}" ]; then
  export SQL_TLS_CA_FILE=/tmp/mythborn-database-ca.pem
  printf '%s' "$SQL_TLS_CA_BASE64" | base64 -d > "$SQL_TLS_CA_FILE"
fi
if [ "${SQL_TLS:-true}" = true ]; then
  export PGSSLMODE=verify-full SQL_TLS=true
  export PGSSLROOTCERT="${SQL_TLS_CA_FILE:-/etc/ssl/certs/ca-certificates.crt}"
fi

# Run once from a trusted machine. The free Render tier has no pre-deploy jobs.
# These isolated schemas contain Temporal state, never game tables.
psql -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE SCHEMA IF NOT EXISTS mythborn_temporal;
CREATE SCHEMA IF NOT EXISTS mythborn_temporal_visibility;
SQL
export SQL_TLS_SERVER_NAME="$SQL_HOST"
for kind in temporal visibility; do
  case "$kind" in
    temporal) schema=mythborn_temporal ;;
    visibility) schema=mythborn_temporal_visibility ;;
  esac
  export SQL_CONNECT_ATTRIBUTES="search_path=$schema"
  temporal-sql-tool setup-schema -v 0.0
  temporal-sql-tool update-schema -d "/etc/temporal/schema/postgresql/v12/$kind/versioned"
done
