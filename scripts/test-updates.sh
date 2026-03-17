#!/usr/bin/env bash
# scripts/test-updates.sh — manual update idempotency harness for examples/updates/
#
# Runs apply v1 → plan (no-op) → apply v2 → plan (no-op) → destroy for each
# update example directory. Each example must have main.tf, v1.tfvars, v2.tfvars.
#
# This is a manual debugging aid; the authoritative CI test is:
#   go test -tags provider_e2e ./e2e -run TestExamplesUpdatesIdempotency
#
# Usage:
#   ./scripts/test-updates.sh                    # test all examples/updates/* dirs
#   ./scripts/test-updates.sh rename_server      # test specific dir by name

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EXAMPLES_DIR="${REPO_ROOT}/examples/updates"

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
sleep 1

export SCW_API_URL="http://localhost:${PORT}"
export SCW_ACCESS_KEY="SCWXXXXXXXXXXXXXXXXX"
export SCW_SECRET_KEY="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_PROJECT_ID="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_ORGANIZATION_ID="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_REGION="fr-par"
export SCW_DEFAULT_ZONE="fr-par-1"
export TF_IN_AUTOMATION="1"

# Determine which examples to test.
DIRS=()
if [[ $# -gt 0 ]]; then
  for name in "$@"; do
    DIRS+=("${EXAMPLES_DIR}/${name}")
  done
else
  while IFS= read -r dir; do
    DIRS+=("$dir")
  done < <(find "${EXAMPLES_DIR}" -mindepth 1 -maxdepth 1 -type d | sort)
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

  # Apply v1.
  if $run_ok && ! "${BIN}" -chdir="${tmp}" apply -auto-approve -input=false -no-color -var-file="${tmp}/v1.tfvars" 2>&1; then
    echo "FAIL: apply v1 failed for ${name}" >&2
    run_ok=false
  fi

  # No-op plan after v1.
  if $run_ok; then
    set +e
    "${BIN}" -chdir="${tmp}" plan -detailed-exitcode -input=false -no-color -var-file="${tmp}/v1.tfvars" 2>&1
    plan_exit=$?
    set -e
    if [[ ${plan_exit} -eq 2 ]]; then
      echo "FAIL: v1 plan not idempotent (drift) for ${name}" >&2
      run_ok=false
    elif [[ ${plan_exit} -eq 1 ]]; then
      echo "FAIL: v1 plan error for ${name}" >&2
      run_ok=false
    fi
  fi

  # Apply v2.
  if $run_ok && ! "${BIN}" -chdir="${tmp}" apply -auto-approve -input=false -no-color -var-file="${tmp}/v2.tfvars" 2>&1; then
    echo "FAIL: apply v2 failed for ${name}" >&2
    run_ok=false
  fi

  # No-op plan after v2.
  if $run_ok; then
    set +e
    "${BIN}" -chdir="${tmp}" plan -detailed-exitcode -input=false -no-color -var-file="${tmp}/v2.tfvars" 2>&1
    plan_exit=$?
    set -e
    if [[ ${plan_exit} -eq 2 ]]; then
      echo "FAIL: v2 plan not idempotent (drift) for ${name}" >&2
      run_ok=false
    elif [[ ${plan_exit} -eq 1 ]]; then
      echo "FAIL: v2 plan error for ${name}" >&2
      run_ok=false
    fi
  fi

  # Destroy.
  if $run_ok && ! "${BIN}" -chdir="${tmp}" destroy -auto-approve -input=false -no-color -var-file="${tmp}/v2.tfvars" 2>&1; then
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
