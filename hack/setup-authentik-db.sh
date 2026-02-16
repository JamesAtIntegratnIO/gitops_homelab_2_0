#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <ssh-host> <password> [pg-port] [ssh-user]"
  echo "  ssh-host  - Host to SSH into (where PostgreSQL is running)"
  echo "  password  - Password for the authentik database user"
  echo "  pg-port   - PostgreSQL port (default: 5432)"
  echo "  ssh-user  - SSH user (default: current user)"
  exit 1
fi

SSH_HOST="$1"
DB_PASS="$2"
DB_PORT="${3:-5432}"
SSH_USER="${4:-$(whoami)}"
DB_USER="authentik"
DB_NAME="authentik"

echo "==> SSHing to ${SSH_USER}@${SSH_HOST} to create role '${DB_USER}' and database '${DB_NAME}'"

ssh "${SSH_USER}@${SSH_HOST}" bash -s "${DB_USER}" "${DB_NAME}" "${DB_PASS}" "${DB_PORT}" <<'REMOTE'
set -euo pipefail

DB_USER="$1"
DB_NAME="$2"
DB_PASS="$3"
DB_PORT="$4"

TMPFILE=$(mktemp /tmp/authentik-db-setup.XXXXXX.sql)
trap "rm -f $TMPFILE" EXIT

cat > "$TMPFILE" <<EOSQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${DB_USER}') THEN
    CREATE ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASS}';
  ELSE
    ALTER ROLE ${DB_USER} WITH PASSWORD '${DB_PASS}';
  END IF;
END
\$\$;

SELECT 'CREATE DATABASE ${DB_NAME} OWNER ${DB_USER}'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DB_NAME}')\gexec

ALTER DATABASE ${DB_NAME} OWNER TO ${DB_USER};
GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};

\c ${DB_NAME}
GRANT ALL ON SCHEMA public TO ${DB_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO ${DB_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO ${DB_USER};
EOSQL

chmod 644 "$TMPFILE"

# Use su if root, otherwise sudo
if [ "$(id -u)" -eq 0 ]; then
  su postgres -c "psql -p ${DB_PORT} -f ${TMPFILE}"
else
  sudo -u postgres psql -p "${DB_PORT}" -f "${TMPFILE}"
fi
REMOTE

echo "==> Done. Database '${DB_NAME}' is ready for Authentik on ${SSH_HOST}."
