#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DIST_DIR="${DIST_DIR:-${PROJECT_ROOT}/dist}"
ARCH="${1:-}"

if [[ -z "${ARCH}" ]]; then
  echo "Usage: $0 <amd64|arm64>"
  exit 1
fi

BIN_XRAYTOOL="${DIST_DIR}/xraytool-linux-${ARCH}"
BIN_XRAYTOOLCTL="${DIST_DIR}/xraytoolctl-linux-${ARCH}"
BIN_XRAY="${DIST_DIR}/xray-linux-${ARCH}"
WEB_DIST="${PROJECT_ROOT}/web/dist"
OUT_ARCHIVE="${DIST_DIR}/xraytool-linux-${ARCH}.tar.gz"

[[ -f "${BIN_XRAYTOOL}" ]] || { echo "Missing binary: ${BIN_XRAYTOOL}"; exit 1; }
[[ -f "${BIN_XRAYTOOLCTL}" ]] || { echo "Missing binary: ${BIN_XRAYTOOLCTL}"; exit 1; }
[[ -f "${BIN_XRAY}" ]] || { echo "Missing fork binary: ${BIN_XRAY}"; exit 1; }
[[ -d "${WEB_DIST}" ]] || { echo "Missing web dist: ${WEB_DIST}"; exit 1; }

WORKDIR="$(mktemp -d)"
cleanup() {
  rm -rf "${WORKDIR}"
}
trap cleanup EXIT

mkdir -p "${WORKDIR}/release"
mkdir -p "${WORKDIR}/release/scripts"
cp "${BIN_XRAYTOOL}" "${WORKDIR}/release/xraytool"
cp "${BIN_XRAYTOOLCTL}" "${WORKDIR}/release/xraytoolctl"
cp "${BIN_XRAY}" "${WORKDIR}/release/xray"
cp -R "${PROJECT_ROOT}/deploy" "${WORKDIR}/release/deploy"
cp "${PROJECT_ROOT}/scripts/online_regression.py" "${WORKDIR}/release/scripts/online_regression.py"
cp -R "${WEB_DIST}" "${WORKDIR}/release/web-dist"
cp "${PROJECT_ROOT}/.env.example" "${WORKDIR}/release/.env.example"
cp "${PROJECT_ROOT}/README.md" "${WORKDIR}/release/README.md"

tar -C "${WORKDIR}" -czf "${OUT_ARCHIVE}" release
echo "Created ${OUT_ARCHIVE}"
