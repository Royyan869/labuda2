#!/bin/sh
set -eu

# ---------------------------------------------------------------------------
# Labuda backend container entrypoint.
#
#   1. Materialize the Firebase service-account JSON from the secret env var
#      FIREBASE_SERVICE_ACCOUNT_JSON into a temp file the Go SDK can read, and
#      point FIREBASE_SERVICE_ACCOUNT_KEY_PATH at it. No secret is baked into
#      the image.
#   2. Run the idempotent migration runner before serving traffic.
#   3. Start the HTTP server.
# ---------------------------------------------------------------------------

# --- 1. Firebase credential materialization -------------------------------
if [ -n "${FIREBASE_SERVICE_ACCOUNT_JSON:-}" ]; then
  fkey="${FIREBASE_KEY_FILE:-/tmp/firebase-service-account.json}"
  printf '%s' "$FIREBASE_SERVICE_ACCOUNT_JSON" > "$fkey"
  chmod 600 "$fkey"
  export FIREBASE_SERVICE_ACCOUNT_KEY_PATH="$fkey"
fi

# --- 2. Migrations ----------------------------------------------------------
# The server never auto-migrates; migrations must be applied before it accepts
# traffic. The runner is idempotent (skips already-applied versions), so this
# is safe on every cold start and after a database is recreated. Set
# RUN_MIGRATIONS_AT_STARTUP=false only for debugging.
if [ "${RUN_MIGRATIONS_AT_STARTUP:-true}" = "true" ]; then
  ./bin/labuda-migrate
fi

# --- 3. Start the HTTP server ------------------------------------------------
exec ./bin/labuda-backend