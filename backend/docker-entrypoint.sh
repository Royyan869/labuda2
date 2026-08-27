#!/bin/sh
set -eu

# ---------------------------------------------------------------------------
# Labuda backend container entrypoint (Render staging/demo).
#
# Render Free does not support the paid-only pre-deploy command, so this
# wrapper performs the pre-start steps inside the container:
#
#   1. Materialize the Firebase service-account JSON from the secret env var
#      FIREBASE_SERVICE_ACCOUNT_JSON into a temp file the Go SDK can read, and
#      point FIREBASE_SERVICE_ACCOUNT_KEY_PATH at it. No secret is baked into
#      the image.
#   2. Derive DB_* / REDIS_* variables from Render connection-string
#      references as a fallback, in case an individual referenced property
#      (DB_HOST, REDIS_HOST, ...) did not resolve on this service.
#   3. Run the idempotent migration runner before serving traffic.
#   4. Start the HTTP server.
# ---------------------------------------------------------------------------

# --- 1. Firebase credential materialization -------------------------------
if [ -n "${FIREBASE_SERVICE_ACCOUNT_JSON:-}" ]; then
  fkey="${FIREBASE_KEY_FILE:-/tmp/firebase-service-account.json}"
  printf '%s' "$FIREBASE_SERVICE_ACCOUNT_JSON" > "$fkey"
  chmod 600 "$fkey"
  export FIREBASE_SERVICE_ACCOUNT_KEY_PATH="$fkey"
fi

# --- 2. Fallback derivation of DB_* / REDIS_* ------------------------------
# Render injects RENDER_DATABASE_URL / RENDER_REDIS_URL as connection-string
# references (documented, always available). The individual properties
# (DB_HOST, DB_PORT, REDIS_HOST, REDIS_PORT, ...) are wired directly in
# render.yaml; these fallbacks only apply if one of those references did not
# resolve, so the service still boots.

# Parse postgresql://[user[:pass]@]host[:port]/dbname into DB_* variables.
derive_db() {
  uri="${RENDER_DATABASE_URL#*://}"
  creds_host="${uri%%/*}"
  dbname="${uri#*/}"
  dbname="${dbname%%\?*}"
  hostport="${creds_host#*@}"
  userpass="${creds_host%@*}"
  if [ "$hostport" = "$userpass" ]; then
    userpass=""
  fi
  user="${userpass%%:*}"
  pass="${userpass#*:}"
  host="${hostport%%:*}"
  port="${hostport##*:}"
  if [ "$host" = "$port" ]; then
    port="5432"
  fi
  export DB_USER="${user:-}"
  export DB_PASSWORD="${pass:-}"
  export DB_HOST="${host:-}"
  export DB_PORT="${port:-}"
  export DB_NAME="${dbname:-}"
}

# Parse redis://[user:pass@]host:port into REDIS_* variables.
derive_redis() {
  uri="${RENDER_REDIS_URL#*://}"
  creds_host="${uri%%/*}"
  hostport="${creds_host#*@}"
  userpass="${creds_host%@*}"
  if [ "$hostport" = "$userpass" ]; then
    userpass=""
  fi
  pass="${userpass#*:}"
  host="${hostport%%:*}"
  port="${hostport##*:}"
  export REDIS_HOST="${host:-}"
  export REDIS_PORT="${port:-}"
  export REDIS_PASSWORD="${pass:-}"
}

if [ -z "${DB_HOST:-}" ] && [ -n "${RENDER_DATABASE_URL:-}" ]; then
  derive_db
fi
if [ -z "${REDIS_HOST:-}" ] && [ -n "${RENDER_REDIS_URL:-}" ]; then
  derive_redis
fi

# --- 3. Migrations ----------------------------------------------------------
# The server never auto-migrates; migrations must be applied before it accepts
# traffic. The runner is idempotent (skips already-applied versions), so this
# is safe on every cold start and after a database is recreated. Set
# RUN_MIGRATIONS_AT_STARTUP=false only for debugging.
if [ "${RUN_MIGRATIONS_AT_STARTUP:-true}" = "true" ]; then
  ./bin/labuda-migrate
fi

# --- 4. Start the HTTP server ------------------------------------------------
exec ./bin/labuda-backend