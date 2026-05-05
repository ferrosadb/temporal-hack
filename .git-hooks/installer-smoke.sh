#!/usr/bin/env bash
# Pre-push smoke test that mirrors the `installer-smoke` job in
# .github/workflows/ci.yml. Brings up the CI cluster (alternate ports
# so it can coexist with a running `make lab-up`), probes ports, and
# tears down. Requires docker or podman.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Reuse the Makefile's engine detection.
ENGINE="$(make -s container-info 2>/dev/null | awk -F': *' '/^engine:/ {print $2}')"
if [ -z "$ENGINE" ] || [ "$ENGINE" = "none" ]; then
  echo "installer-smoke: no container engine found; skipping" >&2
  exit 0
fi

cleanup() {
  echo "installer-smoke: tearing down CI cluster"
  make ci-down >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "installer-smoke: bringing up CI cluster on alt ports"
make ci-up

failures=0
# Probe the CI cluster's ports (alt port set).
for spec in postgres:25432 temporal:27233 mqtt:21883 registry:25050; do
  name="${spec%%:*}"
  port="${spec##*:}"
  ok=0
  for _ in $(seq 1 30); do
    nc -z localhost "$port" 2>/dev/null && { ok=1; break; }
    sleep 2
  done
  if [ "$ok" -ne 1 ]; then
    echo "installer-smoke: $name port $port never came up" >&2
    failures=$((failures+1))
  fi
done

[ "$failures" -eq 0 ]
