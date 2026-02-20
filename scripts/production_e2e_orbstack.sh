#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INSTALLER="${PROJECT_ROOT}/deploy/public-install.sh"

INSTALL_DIR="${INSTALL_DIR:-/opt/xraytool-e2e}"
PORT="${PORT:-28680}"
ADMIN_USER="${ADMIN_USER:-e2eadmin}"
ADMIN_PASS="${ADMIN_PASS:-E2Epass123456}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/production_e2e_orbstack.sh [--port 28680] [--admin-user e2eadmin] [--admin-pass pass] [--install-dir /opt/xraytool-e2e]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)
      PORT="$2"
      shift 2
      ;;
    --admin-user)
      ADMIN_USER="$2"
      shift 2
      ;;
    --admin-pass)
      ADMIN_PASS="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing command: $1"
    exit 1
  fi
}

echo "==> checking local dependencies"
require_cmd orb
require_cmd pnpm
require_cmd tar
require_cmd docker

if [[ ! -f "$INSTALLER" ]]; then
  echo "Installer not found: $INSTALLER"
  exit 1
fi

echo "==> ensuring OrbStack is running"
if [[ "$(orb status)" != "Running" ]]; then
  orb start
fi

if orb -u root systemctl is-active --quiet xraytool; then
  echo "Refusing to run E2E: xraytool service is already active in Orb machine."
  exit 1
fi

ORB_ARCH_RAW="$(orb uname -m)"
case "$ORB_ARCH_RAW" in
  x86_64|amd64)
    ORB_GOARCH="amd64"
    ;;
  aarch64|arm64)
    ORB_GOARCH="arm64"
    ;;
  *)
    echo "Unsupported Orb architecture: $ORB_ARCH_RAW"
    exit 1
    ;;
esac

TMP_DIR="$(mktemp -d "${PROJECT_ROOT}/.tmp-e2e.XXXXXX")"
TMP_DIR_REL="$(basename "$TMP_DIR")"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo "==> building frontend bundle"
CI=true pnpm --dir "${PROJECT_ROOT}/frontend" install --frozen-lockfile
CI=true pnpm --dir "${PROJECT_ROOT}/frontend" run build

echo "==> building Linux binaries for ${ORB_GOARCH} (cgo-enabled container)"
docker run --rm --platform "linux/${ORB_GOARCH}" \
  -v "${PROJECT_ROOT}:/work" \
  -w /work \
  golang:1.26-bookworm \
  bash -lc "/usr/local/go/bin/go build -o '/work/${TMP_DIR_REL}/xraytool' ./cmd/xraytool && /usr/local/go/bin/go build -o '/work/${TMP_DIR_REL}/xraytoolctl' ./cmd/xtoolctl"

echo "==> packaging local release tarball"
mkdir -p "${TMP_DIR}/release"
cp "${TMP_DIR}/xraytool" "${TMP_DIR}/release/xraytool"
cp "${TMP_DIR}/xraytoolctl" "${TMP_DIR}/release/xraytoolctl"
cp -R "${PROJECT_ROOT}/deploy" "${TMP_DIR}/release/deploy"
cp -R "${PROJECT_ROOT}/web/dist" "${TMP_DIR}/release/web-dist"
cp "${PROJECT_ROOT}/.env.example" "${TMP_DIR}/release/.env.example"
cp "${PROJECT_ROOT}/README.md" "${TMP_DIR}/release/README.md"
COPYFILE_DISABLE=1 tar -C "${TMP_DIR}" -czf "${TMP_DIR}/xraytool-linux-${ORB_GOARCH}.tar.gz" release

echo "==> installing service inside Orb machine"
orb -u root env \
  XTOOL_PACKAGE_PATH="${TMP_DIR}/xraytool-linux-${ORB_GOARCH}.tar.gz" \
  bash "${INSTALLER}" \
  --non-interactive \
  --install-dir "${INSTALL_DIR}" \
  --port "${PORT}" \
  --admin-user "${ADMIN_USER}" \
  --admin-pass "${ADMIN_PASS}"

echo "==> verifying service health"
orb -u root systemctl is-active --quiet xraytool
orb curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null

echo "==> verifying login API"
LOGIN_BODY="$(orb curl -fsS -X POST "http://127.0.0.1:${PORT}/api/auth/login" -H 'Content-Type: application/json' -d "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}")"
if [[ "$LOGIN_BODY" != *"token"* ]]; then
  echo "Login check failed, response: $LOGIN_BODY"
  exit 1
fi

echo "==> production E2E passed"
echo "Panel URL: http://<orb-machine-ip>:${PORT}"
