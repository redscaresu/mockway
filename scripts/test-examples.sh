#!/usr/bin/env bash
# scripts/test-examples.sh — manual idempotency harness for examples/working/
#
# Runs apply → plan (no-op check) → destroy for each working example directory.
# This is a manual debugging aid; the authoritative CI test is:
#   go test -tags provider_e2e ./e2e
#
# Usage:
#   ./scripts/test-examples.sh                  # test all examples/working/* dirs
#   ./scripts/test-examples.sh iam_full lb      # test specific dirs by name
#
# Requirements: tofu or terraform in PATH, mockway in PATH or built locally.
#
# Exit codes: 0 = all passed, 1 = one or more failed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EXAMPLES_DIR="${REPO_ROOT}/examples/working"

# Choose IaC binary.
if command -v tofu &>/dev/null; then
  BIN=tofu
elif command -v terraform &>/dev/null; then
  BIN=terraform
else
  echo "ERROR: neither tofu nor terraform found in PATH" >&2
  exit 1
fi

# Start mockway on a random free port.
PORT=$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()" 2>/dev/null || \
       ruby -e "require 'socket'; s=TCPServer.new(0); puts s.addr[1]; s.close" 2>/dev/null || \
       echo 18080)

MOCKWAY_PID=""
cleanup() {
  if [[ -n "${MOCKWAY_PID}" ]]; then
    kill "${MOCKWAY_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# Build or use installed mockway.
if command -v mockway &>/dev/null; then
  MOCKWAY_BIN=mockway
else
  echo "==> Building mockway..."
  go build -o /tmp/mockway-test "${REPO_ROOT}/cmd/mockway"
  MOCKWAY_BIN=/tmp/mockway-test
fi

echo "==> Starting mockway on port ${PORT}..."
"${MOCKWAY_BIN}" --port "${PORT}" &
MOCKWAY_PID=$!
sleep 1  # give it a moment to bind

export SCW_API_URL="http://localhost:${PORT}"
export SCW_ACCESS_KEY="SCWXXXXXXXXXXXXXXXXX"
export SCW_SECRET_KEY="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_PROJECT_ID="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_ORGANIZATION_ID="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_REGION="fr-par"
export SCW_DEFAULT_ZONE="fr-par-1"
export TF_IN_AUTOMATION="1"

# Determine which examples to test.
if [[ $# -gt 0 ]]; then
  DIRS=()
  for name in "$@"; do
    DIRS+=("${EXAMPLES_DIR}/${name}")
  done
else
  mapfile -t DIRS < <(find "${EXAMPLES_DIR}" -mindepth 1 -maxdepth 1 -type d | sort)
fi

PASSED=()
FAILED=()

for dir in "${DIRS[@]}"; do
  name="$(basename "${dir}")"
  echo ""
  echo "════════════════════════════════════════"
  echo "  ${name}"
  echo "════════════════════════════════════════"

  tmp=$(mktemp -d)
  cp -r "${dir}/." "${tmp}/"

  run_ok=true

  # Reset mockway state between examples.
  curl -s -X POST "http://localhost:${PORT}/mock/reset" >/dev/null

  if ! "${BIN}" -chdir="${tmp}" init -input=false -no-color -reconfigure 2>&1; then
    echo "FAIL: init failed for ${name}" >&2
    run_ok=false
  fi

  if $run_ok && ! "${BIN}" -chdir="${tmp}" apply -auto-approve -input=false -no-color 2>&1; then
    echo "FAIL: apply failed for ${name}" >&2
    run_ok=false
  fi

  if $run_ok; then
    # no-op plan check: exit 0 = no changes, exit 2 = drift
    set +e
    "${BIN}" -chdir="${tmp}" plan -detailed-exitcode -input=false -no-color 2>&1
    plan_exit=$?
    set -e
    if [[ ${plan_exit} -eq 2 ]]; then
      echo "FAIL: second apply is not idempotent (drift detected) for ${name}" >&2
      run_ok=false
    elif [[ ${plan_exit} -eq 1 ]]; then
      echo "FAIL: plan error for ${name}" >&2
      run_ok=false
    fi
  fi

  if $run_ok && ! "${BIN}" -chdir="${tmp}" destroy -auto-approve -input=false -no-color 2>&1; then
    echo "FAIL: destroy failed for ${name}" >&2
    run_ok=false
  fi

  rm -rf "${tmp}"

  if $run_ok; then
    PASSED+=("${name}")
    echo "PASS: ${name}"
  else
    FAILED+=("${name}")
  fi
done

echo ""
echo "════════════════════════════════════════"
echo "Results: ${#PASSED[@]} passed, ${#FAILED[@]} failed"
for p in "${PASSED[@]+"${PASSED[@]}"}"; do echo "  ✓ ${p}"; done
for f in "${FAILED[@]+"${FAILED[@]}"}"; do echo "  ✗ ${f}"; done
echo "════════════════════════════════════════"

[[ ${#FAILED[@]} -eq 0 ]]
