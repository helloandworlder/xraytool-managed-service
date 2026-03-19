# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and follows the repository release tags.

## [Unreleased]

## [v0.1.37] - 2026-03-19

### Added

- GoSea-Light telemetry reporting with configurable node identity, scheduler push cadence, version/capability payloads, and route-level runtime stats.
- Residential order regression coverage for public-IP selection, duplicate dedicated line rejection, and safer CPU sampling across Unix and fallback runtimes.
- `threeuitui` SOCKS export support with `--export-socks-txt` and wildcard IP filtering for direct panel extraction workflows.

### Changed

- Release workflow now injects build version, commit, build time, and protocol metadata into shipped binaries.
- Order export and frontend controls now support residential TXT layout selection and refreshed panel settings wiring.

## [v0.1.36] - 2026-03-15

### Changed

- Release workflow now only builds and publishes `linux/amd64` artifacts.

## [v0.1.35] - 2026-03-15

### Added

- GitHub Release CI that can build Linux `amd64` and `arm64` artifacts from either tag pushes or manual workflow dispatch.
- Shared `scripts/package_release.sh` packaging flow so GitHub Release assets and OrbStack production E2E use the same release layout.
- Dedicated ingress line API metadata for `dedicatedInboundId` and protocol discovery to support runtime-aware platform integrations.
- Dedicated runtime API coverage for VLESS/Vmess/compat link generation regression checks.

### Changed

- Dedicated ingress route selection no longer falls back to unrelated route types when no matching enabled ingress line exists.
- VLESS dedicated links are now generated from inbound-derived share parameters instead of a lossy compatibility string.
- Release workflow now uploads workflow artifacts and `checksums.txt`, and can generate GitHub Releases from manual runs.

## [v0.1.34] - 2026-03-13

### Added

- Dedicated runtime compatibility APIs for platform integration.
- Dedicated inbound SOCKS5 export and long-order timeout guards.

### Fixed

- Managed rebuild restart coalescing for runtime stability.
