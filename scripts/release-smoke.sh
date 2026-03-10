#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY_PATH="${1:-${ROOT_DIR}/apus}"
RUNNER_PATH="${ROOT_DIR}/.tmp/bin/fixturerunner"
WORK_ROOT="${ROOT_DIR}/.tmp/release-smoke"
PACKAGE_PATH="${APUS_PACKAGE_PATH:-}"

if [[ ! -x "${BINARY_PATH}" ]]; then
  echo "release smoke: apus binary is missing or not executable: ${BINARY_PATH}" >&2
  exit 1
fi

if [[ ! -x "${RUNNER_PATH}" ]]; then
  echo "release smoke: fixturerunner binary is missing or not executable: ${RUNNER_PATH}" >&2
  exit 1
fi

mkdir -p "${WORK_ROOT}"

runner_args=(
  -apus-bin "${BINARY_PATH}"
  -work-root "${WORK_ROOT}"
)

if [[ -n "${PACKAGE_PATH}" ]]; then
  runner_args+=(-apus-package-path "${PACKAGE_PATH}")
fi

echo "release smoke: version"
"${BINARY_PATH}" --version >/dev/null

echo "release smoke: swiftui-single-target"
"${RUNNER_PATH}" "${runner_args[@]}" -fixture swiftui-single-target

echo "release smoke: nonstandard-layout"
"${RUNNER_PATH}" "${runner_args[@]}" -fixture nonstandard-layout
