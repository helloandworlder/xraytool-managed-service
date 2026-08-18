# XrayTool Managed Xray Fork

This directory is a vendored, source-built fork of Xray-core derived from the
reviewed IPIPX production fork. It is the XrayTool managed data-plane
dependency and must not be replaced with an unmodified upstream binary:
XrayTool depends on runtime connection limits, per-account bidirectional rate
limits, instance fair-share scheduling, and hot-update control APIs implemented
here.

## Provenance

- This snapshot was copied from IPIPX at commit
  `70ea8e3d2af07b7f6d8d4635dbca934e552ab8ec` on 2026-08-19.
- The vendored tree entered this repository in IPIPX commit `1cf75e48`.
- The exact upstream commit used for that initial import was not recorded. Do
  not invent one or infer it from file timestamps.
- The upstream comparison reference reviewed on 2026-07-20 is XTLS/Xray-core
  tag `v26.6.1` (`94ffd500`). This is a comparison point, not a claim that the
  original import was based on that commit.
- License and upstream notices remain in `LICENSE`, `README.md`, and the source
  headers in this directory.

## IPIPX Delta

The authoritative audit trail is the IPIPX Git history for this path. The
current post-import changes are:

| IPIPX commit | Purpose |
| --- | --- |
| `cd8e71b9` | Mixed proxy protocol support and per-user runtime connection/rate limits. |
| `766347e7` | Convert fair-share configuration into runtime byte limits. |
| `8faf5abc` | Prevent splice fast paths from bypassing governed links. |
| `db053c1b` | Apply the shared user bucket to both upload and download paths. |
| `9a374fc1` | Keep sniffing compatible with wrapped rate-limited readers. |
| `05cc7c53` | Extend hot-update/control APIs and inbound runtime behavior. |
| `45a96e73` | Add fair-share control, connection limits, and limiter hardening. |

XrayTool additions in this vendored snapshot:

| Date | Purpose |
| --- | --- |
| `2026-08-19` | Split account uplink/downlink buckets and add generated directional account fields. |
| `2026-08-19` | Add process-wide directional instance buckets around the real dispatcher pipe readers, including anonymous/system traffic. |
| `2026-08-19` | Keep limiter cancellation/apply errors visible and support zero-value dynamic disable without stale buckets. |

Use `git log -- third_party/xray-fork` and `git show <commit> --
third_party/xray-fork` for the exact patch set. Commit messages are descriptive
context; tests and the diff are the behavioral authority.

## Upstream Update Procedure

1. Record the candidate upstream tag and full commit SHA before changing files.
2. Reconstruct a clean upstream tree outside this directory and compare it to
   the current vendor tree. Never overwrite the vendor tree first.
3. Reapply each IPIPX delta above deliberately, resolving generated protobuf
   files with the repository toolchain rather than hand-editing them.
4. Run the focused dispatcher, proxy, buffer, protocol, proxyman, and fair-share
   tests, then `go build ./...` in this directory.
5. Build and test `node/`, which consumes this fork through its local `replace`.
6. Validate plain HTTP and SOCKS traffic in both directions with connection and
   bandwidth limits enabled. Validate Xray blue-green adoption separately.
7. Update this file with the new upstream SHA and delta commits in the same
   change. A source update without provenance or behavioral verification is not
   releasable.

Known upstream-wide `go vet` findings must be recorded explicitly. They are not
permission to ignore new findings in files touched by an update.
