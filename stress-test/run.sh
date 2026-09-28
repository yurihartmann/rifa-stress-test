#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT}"

export K6_BASE_URL="${K6_BASE_URL:-http://localhost:8080}"
export K6_WEBHOOK_SECRET="${K6_WEBHOOK_SECRET:-lab-webhook-secret}"
export K6_ADMIN_TOKEN="${K6_ADMIN_TOKEN:-lab-admin-token}"
export K6_TOTAL_TICKETS="${K6_TOTAL_TICKETS:-1000}"
export K6_TICKET_PRICE_CENTS="${K6_TICKET_PRICE_CENTS:-1000}"
export K6_QUANTITY="${K6_QUANTITY:-1}"
export K6_VUS="${K6_VUS:-2}"
export K6_DURATION="${K6_DURATION:-20s}"
export K6_GRACEFUL_STOP="${K6_GRACEFUL_STOP:-30s}"
export K6_SETUP_TIMEOUT="${K6_SETUP_TIMEOUT:-120s}"
export K6_HTTP_TIMEOUT="${K6_HTTP_TIMEOUT:-60s}"
export K6_POLL_TIMEOUT_MS="${K6_POLL_TIMEOUT_MS:-15000}"
export K6_POLL_INTERVAL_SEC="${K6_POLL_INTERVAL_SEC:-0.5}"

if [[ $# -gt 0 && "$1" != -* ]]; then
  export K6_SCENARIO="$1"
  shift
fi
export K6_SCENARIO="${K6_SCENARIO:-funnel}"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 não está instalado. Os scripts estão em ${ROOT}." >&2
  echo "Instale o k6 e rode de novo: https://grafana.com/docs/k6/latest/set-up/install-k6/" >&2
  exit 127
fi

echo "k6 scenario=${K6_SCENARIO} vus=${K6_VUS} duration=${K6_DURATION} base=${K6_BASE_URL}"

exec k6 run --summary-export="${ROOT}/summary.json" "$@" "${ROOT}/script.js"
