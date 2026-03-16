#!/usr/bin/env bash
# scripts/test-misconfigured.sh — verify that misconfigured examples fail with
# the expected error, not a provider panic.
#
# Each single-workspace example must:
#   1. Fail on apply (non-zero exit code)
#   2. Produce no "panic" in the output (no provider nil-deref)
#
# Multi-workspace examples (cross_state_orphan, vpc_deleted_before_private_network)
# require sequential applies across two directories; the failure is expected at
# destroy time, not apply time.
#
# Usage:
#   ./scripts/test-misconfigured.sh                          # run all
#   ./scripts/test-misconfigured.sh security_group_name_not_id lb_acl_missing_frontend
#
# Exit codes: 0 = all passed, 1 = one or more failed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EXAMPLES_DIR="${REPO_ROOT}/examples/misconfigured"

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
       echo 18081)

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

reset_state() {
  curl -s -X POST "http://localhost:${PORT}/mock/reset" >/dev/null
}

check_no_panic() {
  local output="$1"
  local name="$2"
  if echo "${output}" | grep -q "^goroutine\|^panic:"; then
    echo "FAIL: ${name} — provider panic detected" >&2
    echo "${output}" | grep -A5 "^panic:\|^goroutine" >&2
    return 1
  fi
  return 0
}

PASSED=()
FAILED=()

# ── Multi-workspace: cross_state_orphan ──────────────────────────────────────
run_cross_state_orphan() {
  local name="cross_state_orphan"
  local base="${EXAMPLES_DIR}/${name}"
  echo ""
  echo "════════════════════════════════════════"
  echo "  ${name} (multi-workspace)"
  echo "════════════════════════════════════════"

  reset_state

  local app_tmp platform_tmp
  app_tmp=$(mktemp -d)
  platform_tmp=$(mktemp -d)
  cp -r "${base}/platform/." "${platform_tmp}/"
  cp -r "${base}/app/." "${app_tmp}/"

  local run_ok=true
  local output

  # 1. Apply platform (creates IAM application).
  if ! output=$("${BIN}" -chdir="${platform_tmp}" init -input=false -no-color -reconfigure 2>&1) || \
     ! check_no_panic "${output}" "${name}/platform-init"; then
    echo "${output}"
    echo "FAIL: ${name} — platform init failed" >&2
    run_ok=false
  fi

  if $run_ok; then
    if ! output=$("${BIN}" -chdir="${platform_tmp}" apply -auto-approve -input=false -no-color 2>&1) || \
       ! check_no_panic "${output}" "${name}/platform-apply"; then
      echo "${output}"
      echo "FAIL: ${name} — platform apply failed (expected success)" >&2
      run_ok=false
    fi
  fi

  # 2. Apply app (creates resources referencing the platform's IAM application).
  if $run_ok; then
    if ! output=$("${BIN}" -chdir="${app_tmp}" init -input=false -no-color -reconfigure 2>&1) || \
       ! check_no_panic "${output}" "${name}/app-init"; then
      echo "${output}"
      echo "FAIL: ${name} — app init failed" >&2
      run_ok=false
    fi
  fi

  if $run_ok; then
    if ! output=$("${BIN}" -chdir="${app_tmp}" apply -auto-approve -input=false -no-color 2>&1) || \
       ! check_no_panic "${output}" "${name}/app-apply"; then
      echo "${output}"
      echo "FAIL: ${name} — app apply failed (expected success)" >&2
      run_ok=false
    fi
  fi

  # 3. Destroy platform — must fail with 409 (app resources still reference platform resources).
  if $run_ok; then
    set +e
    output=$("${BIN}" -chdir="${platform_tmp}" destroy -auto-approve -input=false -no-color 2>&1)
    local destroy_exit=$?
    set -e
    check_no_panic "${output}" "${name}/platform-destroy" || run_ok=false
    if $run_ok && [[ ${destroy_exit} -eq 0 ]]; then
      echo "FAIL: ${name} — destroy succeeded but should have failed with 409" >&2
      run_ok=false
    fi
    if $run_ok && ! echo "${output}" | grep -qiE "409|conflict|dependent"; then
      echo "FAIL: ${name} — expected 409/conflict error, got: ${output}" >&2
      run_ok=false
    fi
  fi

  rm -rf "${app_tmp}" "${platform_tmp}"

  if $run_ok; then
    PASSED+=("${name}")
    echo "PASS: ${name}"
  else
    FAILED+=("${name}")
  fi
}

# ── Multi-workspace: vpc_deleted_before_private_network ──────────────────────
run_vpc_deleted_before_private_network() {
  local name="vpc_deleted_before_private_network"
  local base="${EXAMPLES_DIR}/${name}"
  echo ""
  echo "════════════════════════════════════════"
  echo "  ${name} (multi-workspace)"
  echo "════════════════════════════════════════"

  reset_state

  local vpc_tmp pn_tmp
  vpc_tmp=$(mktemp -d)
  pn_tmp=$(mktemp -d)
  cp -r "${base}/vpc/." "${vpc_tmp}/"
  cp -r "${base}/pn/." "${pn_tmp}/"

  local run_ok=true
  local output

  # 1. Apply vpc workspace.
  if ! output=$("${BIN}" -chdir="${vpc_tmp}" init -input=false -no-color -reconfigure 2>&1) || \
     ! check_no_panic "${output}" "${name}/vpc-init"; then
    echo "${output}"
    echo "FAIL: ${name} — vpc init failed" >&2
    run_ok=false
  fi

  if $run_ok; then
    if ! output=$("${BIN}" -chdir="${vpc_tmp}" apply -auto-approve -input=false -no-color 2>&1) || \
       ! check_no_panic "${output}" "${name}/vpc-apply"; then
      echo "${output}"
      echo "FAIL: ${name} — vpc apply failed (expected success)" >&2
      run_ok=false
    fi
  fi

  # 2. Apply pn workspace using VPC state output.
  if $run_ok; then
    # The pn workspace uses terraform_remote_state pointing to ../vpc/terraform.tfstate.
    # Copy the vpc state into a location the pn config expects.
    mkdir -p "${pn_tmp}/../vpc"
    cp "${vpc_tmp}/terraform.tfstate" "${pn_tmp}/../vpc/terraform.tfstate" 2>/dev/null || true

    if ! output=$("${BIN}" -chdir="${pn_tmp}" init -input=false -no-color -reconfigure 2>&1) || \
       ! check_no_panic "${output}" "${name}/pn-init"; then
      echo "${output}"
      echo "FAIL: ${name} — pn init failed" >&2
      run_ok=false
    fi
  fi

  if $run_ok; then
    if ! output=$("${BIN}" -chdir="${pn_tmp}" apply -auto-approve -input=false -no-color 2>&1) || \
       ! check_no_panic "${output}" "${name}/pn-apply"; then
      echo "${output}"
      echo "FAIL: ${name} — pn apply failed (expected success)" >&2
      run_ok=false
    fi
  fi

  # 3. Destroy vpc workspace — must fail because private network still exists.
  if $run_ok; then
    set +e
    output=$("${BIN}" -chdir="${vpc_tmp}" destroy -auto-approve -input=false -no-color 2>&1)
    local destroy_exit=$?
    set -e
    check_no_panic "${output}" "${name}/vpc-destroy" || run_ok=false
    if $run_ok && [[ ${destroy_exit} -eq 0 ]]; then
      echo "FAIL: ${name} — destroy succeeded but should have failed with 409" >&2
      run_ok=false
    fi
    if $run_ok && ! echo "${output}" | grep -qiE "409|conflict|dependent"; then
      echo "FAIL: ${name} — expected 409/conflict error, got: ${output}" >&2
      run_ok=false
    fi
  fi

  rm -rf "${vpc_tmp}" "${pn_tmp}"

  if $run_ok; then
    PASSED+=("${name}")
    echo "PASS: ${name}"
  else
    FAILED+=("${name}")
  fi
}

# ── Single-workspace examples ─────────────────────────────────────────────────
run_single() {
  local name="$1"
  local dir="${EXAMPLES_DIR}/${name}"
  echo ""
  echo "════════════════════════════════════════"
  echo "  ${name}"
  echo "════════════════════════════════════════"

  reset_state

  local tmp
  tmp=$(mktemp -d)
  cp -r "${dir}/." "${tmp}/"

  local run_ok=true
  local output

  if ! output=$("${BIN}" -chdir="${tmp}" init -input=false -no-color -reconfigure 2>&1); then
    echo "${output}"
    echo "FAIL: ${name} — init failed" >&2
    run_ok=false
  fi
  check_no_panic "${output}" "${name}/init" || run_ok=false

  if $run_ok; then
    set +e
    output=$("${BIN}" -chdir="${tmp}" apply -auto-approve -input=false -no-color 2>&1)
    local apply_exit=$?
    set -e

    check_no_panic "${output}" "${name}/apply" || run_ok=false

    if $run_ok && [[ ${apply_exit} -eq 0 ]]; then
      echo "FAIL: ${name} — apply succeeded but should have failed" >&2
      run_ok=false
    fi

    if $run_ok && ! echo "${output}" | grep -qiE "404|409|not_found|conflict|error"; then
      echo "FAIL: ${name} — apply failed but output has no recognisable error indicator" >&2
      echo "${output}" | tail -10 >&2
      run_ok=false
    fi
  fi

  rm -rf "${tmp}"

  if $run_ok; then
    PASSED+=("${name}")
    echo "PASS: ${name}"
  else
    FAILED+=("${name}")
  fi
}

# ── Multi-workspace directories (contain subdirectories, not main.tf) ─────────
MULTI_WORKSPACE=(cross_state_orphan vpc_deleted_before_private_network)

# Determine which examples to test.
if [[ $# -gt 0 ]]; then
  for name in "$@"; do
    case "${name}" in
      cross_state_orphan)         run_cross_state_orphan ;;
      vpc_deleted_before_private_network) run_vpc_deleted_before_private_network ;;
      *)                          run_single "${name}" ;;
    esac
  done
else
  mapfile -t ALL_DIRS < <(find "${EXAMPLES_DIR}" -mindepth 1 -maxdepth 1 -type d | sort | xargs -I{} basename {})
  for name in "${ALL_DIRS[@]}"; do
    case "${name}" in
      cross_state_orphan)         run_cross_state_orphan ;;
      vpc_deleted_before_private_network) run_vpc_deleted_before_private_network ;;
      *)
        # Skip dirs that have no main.tf (not a single-workspace example).
        if [[ -f "${EXAMPLES_DIR}/${name}/main.tf" ]]; then
          run_single "${name}"
        fi
        ;;
    esac
  done
fi

echo ""
echo "════════════════════════════════════════"
echo "Results: ${#PASSED[@]} passed, ${#FAILED[@]} failed"
for p in "${PASSED[@]}"; do echo "  ✓ ${p}"; done
for f in "${FAILED[@]}"; do echo "  ✗ ${f}"; done
echo "════════════════════════════════════════"

[[ ${#FAILED[@]} -eq 0 ]]
