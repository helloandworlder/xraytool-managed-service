#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Docker context"
docker context ls

echo "==> [Linux] Backend build + tests"
docker run --rm -v "${ROOT_DIR}:/work/xraytool" -w /work/xraytool golang:1.24 bash -lc "export PATH=/usr/local/go/bin:\$PATH GOTOOLCHAIN=auto && go test ./... && go build ./..."

echo "==> [Linux] Frontend pnpm build"
docker run --rm -e CI=true -v "${ROOT_DIR}:/work/xraytool" -w /work/xraytool/frontend node:22-bookworm bash -lc "corepack enable && corepack prepare pnpm@10.28.2 --activate && pnpm install --frozen-lockfile && pnpm run build"

echo "==> [Linux] API smoke test"
docker run --rm -v "${ROOT_DIR}:/work/xraytool" -w /work/xraytool golang:1.24 bash -lc "export PATH=/usr/local/go/bin:\$PATH GOTOOLCHAIN=auto && apt-get update >/dev/null && apt-get install -y curl >/dev/null && go build -o xraytool ./cmd/xraytool && ./xraytool >/tmp/xraytool.log 2>&1 & pid=\$!; sleep 3; curl -sf http://127.0.0.1:18080/healthz; kill \$pid || true; wait \$pid || true"

echo "==> Linux full suite passed"
