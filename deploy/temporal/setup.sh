#!/bin/sh
set -eu
: "${SQL_HOST:?SQL_HOST is required}"
: "${SQL_USER:?SQL_USER is required}"
: "${SQL_PASSWORD:?SQL_PASSWORD is required}"
export SQL_PORT="${SQL_PORT:-5432}" SQL_PLUGIN=postgres12
export PGHOST="$SQL_HOST" PGPORT="$SQL_PORT" PGUSER="$SQL_USER" PGPASSWORD="$SQL_PASSWORD"

# Run as a Render pre-deploy command, never against Mythborn's Tiger database.
# The managed instance already contains temporal; visibility needs its own DB.
psql -X -v ON_ERROR_STOP=1 -d temporal <<'SQL'
SELECT 'CREATE DATABASE temporal_visibility' WHERE NOT EXISTS
  (SELECT FROM pg_database WHERE datname = 'temporal_visibility')
\gexec
SQL
for database in temporal temporal_visibility; do
  export SQL_DATABASE="$database"
  temporal-sql-tool setup-schema -v 0.0
  case "$database" in
    temporal) schema=/etc/temporal/schema/postgresql/v12/temporal/versioned ;;
    temporal_visibility) schema=/etc/temporal/schema/postgresql/v12/visibility/versioned ;;
  esac
  temporal-sql-tool update-schema -d "$schema"
done
