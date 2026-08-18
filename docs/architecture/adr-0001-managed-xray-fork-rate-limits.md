# ADR-0001: Managed Xray fork is the rate-limit enforcement authority

## Context

XrayTool currently writes `uplinkLimitBps` and `downlinkLimitBps` into account
JSON, but upstream Xray ignores those unknown fields. That is configuration
metadata, not byte-level enforcement.

IPIPX already carries a source-built Xray fork with token-bucket wrappers in
the dispatcher, stable per-account user objects, node fair-share scheduling,
and regression tests for upload/download direction mapping and splice bypass.

GoSea-Light policy values use bit/s. The first default is exactly
15,000,000 bit/s, which is 1,875,000 bytes/s for the byte-oriented limiter.

## Decision

XrayTool uses the vendored `third_party/xray-fork` module through the local
Go `replace` directive. The fork is the only component allowed to claim that
traffic is rate-limited.

The initial scope is:

- one shared account limiter per XrayTool process and direction;
- one shared instance limiter per XrayTool process and direction;
- no per-connection bucket;
- separate uplink and downlink buckets for every account;
- separate uplink and downlink buckets for the instance;
- governed links stay on buffered copy paths so splice cannot bypass the
  limiter;
- apply failures are errors and must be observable as not applied;
- changes use in-place update where supported, with serialized full rebuilds
  only when unavoidable.

An account-wide limit across multiple XrayTool processes is explicitly not
claimed by this ADR. It requires account affinity or a distributed limiter and
will be a separate architecture decision.

## Alternatives considered

1. Keep writing custom JSON fields: rejected because upstream Xray discards
   them and there is no enforcement path.
2. Linux `tc`/eBPF for all limits: useful for process/instance shaping, but it
   cannot reliably identify logical proxy accounts after the Xray data path
   without an additional marking boundary.
3. A separate proxy layer: possible, but duplicates protocol ownership and
   adds another data-plane hop before the current fork is exhausted.

## Consequences

The Xray fork becomes a maintained dependency and must record its source
version, delta, generated protobuf procedure, focused tests, and release
artifact digest. XrayTool builds that do not use the local fork are invalid.

The first implementation is process-local. Multi-instance global account
limits remain a visible capability boundary rather than a silent false claim.

## Confidence and reversal

Confidence is high for process-local user and instance enforcement because the
dispatcher and pipe-level tests exercise the actual governed link.

The decision is reversible before production rollout by removing the local
replace and fork, but production artifacts must never be mixed across the two
data planes.
