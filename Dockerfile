FROM node:22-bookworm AS web-build

WORKDIR /src/frontend
RUN corepack enable && corepack prepare pnpm@10.28.2 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm run build

FROM golang:1.26-bookworm AS go-build

WORKDIR /src
COPY go.mod go.sum ./
COPY third_party/xray-fork ./third_party/xray-fork
RUN go mod download
COPY . ./
COPY --from=web-build /src/web/dist ./web/dist

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown
RUN export LDFLAGS="-X xraytool/internal/buildinfo.Version=${VERSION} -X xraytool/internal/buildinfo.Commit=${COMMIT_SHA} -X xraytool/internal/buildinfo.BuildTime=${BUILD_TIME} -X xraytool/internal/buildinfo.ProtocolVersion=1" \
    && CGO_ENABLED=1 go build -buildvcs=false -ldflags "${LDFLAGS}" -o /out/xraytool ./cmd/xraytool
RUN cd third_party/xray-fork && CGO_ENABLED=1 go build -buildvcs=false -o /out/xray ./main

FROM debian:bookworm-slim AS runtime

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="XrayTool Managed Service" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_TIME}"

RUN apt-get update \
    && apt-get install --no-install-recommends --yes ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/xraytool
COPY --from=go-build /out/xraytool ./xraytool
COPY --from=go-build /out/xray ./xray
COPY --from=go-build /src/web/dist ./web/dist

ENV XTOOL_LISTEN=:18080 \
    XTOOL_DATA_DIR=/var/lib/xraytool \
    XTOOL_XRAY_BIN=/opt/xraytool/xray
RUN mkdir -p /var/lib/xraytool

EXPOSE 18080
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=6 \
  CMD curl --fail --silent http://127.0.0.1:18080/healthz || exit 1

ENTRYPOINT ["/opt/xraytool/xraytool"]
