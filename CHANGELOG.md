# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and follows the repository release tags.

## [Unreleased]

### Added

- GitHub Release CI that can build Linux `amd64` and `arm64` artifacts from either tag pushes or manual workflow dispatch.
- Shared `scripts/package_release.sh` packaging flow so GitHub Release assets and OrbStack production E2E use the same release layout.
- Dedicated ingress line API metadata for `dedicatedInboundId` and protocol discovery to support runtime-aware platform integrations.
- Dedicated runtime API coverage for VLESS/Vmess/compat link generation regression checks.

### Changed

- Dedicated ingress route selection no longer falls back to unrelated route types when no matching enabled ingress line exists.
- VLESS dedicated links are now generated from inbound-derived share parameters instead of a lossy compatibility string.
- Release workflow now uploads workflow artifacts and `checksums.txt`, can generate GitHub Releases from manual runs, and only builds `linux/amd64`.

## [v0.1.34] - 2026-03-13

### Added

- Dedicated runtime compatibility APIs for platform integration.
- Dedicated inbound SOCKS5 export and long-order timeout guards.

### Fixed

- Managed rebuild restart coalescing for runtime stability.
