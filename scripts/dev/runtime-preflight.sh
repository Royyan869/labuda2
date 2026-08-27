#!/usr/bin/env bash
# runtime-preflight.sh — read-only snapshot of the dev runtime state.
#
# Answers the single question "is my dev box in a state where a governance /
# runtime batch can be validated against an official HTTP flow?" without
# starting anything, mutating state, or requiring auth.
#
# Designed to be safe to run before every governance batch so the operator
# never wastes a session discovering Docker is off or the backend isn't up.

set -u

probe_port() {
  local host=$1 port=$2 name=$3
  if (echo >/dev/tcp/"$host"/"$port") >/dev/null 2>&1; then
    echo "  OK   $name (:$port) reachable"
    return 0
  else
    echo "  --   $name (:$port) not reachable"
    return 1
  fi
}

probe_http() {
  local url=$1 name=$2
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 "$url" 2>/dev/null)
  code=${code:-000}
  if [ "$code" = "200" ]; then
    echo "  OK   $name → 200 ($url)"
  else
    echo "  --   $name → $code ($url)"
  fi
}

env_mode() {
  local f=$1
  [ -f "$f" ] || { echo "absent"; return; }
  local v
  v=$(grep -E '^SEARCH_CONTENT_EVALUATOR_MODE=' "$f" 2>/dev/null | tail -1 | cut -d= -f2-)
  echo "${v:-unset → defaults to shadow}"
}

echo "== runtime preflight =="
echo "[infrastructure]"
docker info >/dev/null 2>&1 && echo "  OK   docker daemon" || echo "  --   docker daemon (start Docker Desktop)"
probe_port localhost 5432 "postgres"
probe_port localhost 6379 "redis"
probe_port localhost 8080 "backend"

echo "[backend endpoints]"
probe_http "http://localhost:8080/health/live"  "/health/live"
probe_http "http://localhost:8080/health/ready" "/health/ready"
probe_http "http://localhost:8080/metrics"      "/metrics"

echo "[evaluator mode]"
echo "  .env             SEARCH_CONTENT_EVALUATOR_MODE=$(env_mode backend/.env)"
echo "  .env.example     SEARCH_CONTENT_EVALUATOR_MODE=$(env_mode backend/.env.example)"

echo "[hint]"
echo "  next steps if anything above is '--':"
echo "    docker daemon  → start Docker Desktop"
echo "    postgres/redis → docker-compose up -d"
echo "    backend        → cd backend && go run ./cmd/core_server"
echo "    enforce mode   → echo SEARCH_CONTENT_EVALUATOR_MODE=enforce >> backend/.env"
