#!/usr/bin/env bash
# Pre-push smoke test that mirrors the `installer-smoke` job in
# .github/workflows/ci.yml. Brings up the lab stack, probes ports,
# and tears down. Requires docker or podman.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Reuse the Makefile's engine detection by asking it to print the
# resolved compose command.
COMPOSE="$(make -s container-info 2>/dev/null | awk -F': *' '/^compose:/ {print $2}')"
ENGINE="$(make -s container-info 2>/dev/null | awk -F': *' '/^engine:/ {print $2}')"

if [ -z "$COMPOSE" ] || [ "$ENGINE" = "none" ]; then
  echo "installer-smoke: no container engine found; skipping" >&2
  exit 0
fi

echo "installer-smoke: bringing up lab stack via $COMPOSE"
cd installer/docker-compose
$COMPOSE up -d --wait

cleanup() {
  echo "installer-smoke: tearing down"
  $COMPOSE down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

failures=0
for port in 5432 7233 1883 5000; do
  ok=0
  for _ in $(seq 1 30); do
    nc -z localhost "$port" 2>/dev/null && { ok=1; break; }
    sleep 2
  done
  if [ "$ok" -ne 1 ]; then
    echo "installer-smoke: port $port never came up" >&2
    failures=$((failures+1))
  fi
done

[ "$failures" -eq 0 ]
