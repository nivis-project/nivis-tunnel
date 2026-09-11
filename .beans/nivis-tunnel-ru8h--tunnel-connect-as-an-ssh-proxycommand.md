---
# nivis-tunnel-ru8h
title: tunnel connect as an ssh ProxyCommand
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:03Z
updated_at: 2026-09-11T15:40:51Z
parent: nivis-tunnel-xvjv
openspec-link: openspec/changes/archive/2026-09-11-tunnel-proxycommand
blocked_by:
    - nivis-tunnel-qtx9
---

`nivis-tunnel connect <id>` dials the relay, completes the Noise handshake as
orchestrator, and pipes the stream to stdin/stdout.

## Why

This is the entire integration surface of the project. Because ssh speaks to a
`ProxyCommand` over stdio, everything above it — `nix-copy-closure`,
`switch-to-configuration`, `deploy-rs`, plain interactive ssh — works unchanged.

## Summary of Changes

OpenSpec change `tunnel-proxycommand`, capability `tunnel-proxycommand`.

- `internal/tunnel.Connect` dials, announces as orchestrator, handshakes, and
  returns a secured connection plus the agent's public key for later pinning.
- `internal/tunnel.Splice` copies between the stream and a reader/writer pair.
  It takes interfaces rather than reaching for `os.Stdin`/`os.Stdout`, so the
  copy loop is testable without a terminal.
- `cmd/tunnel` gained `connect` and `keygen`. `keygen` writes the private half
  0600 and prints the public half to stdout, so it can be piped straight into a
  NixOS configuration.
- Only the private half is stored; the public half is derived on read via
  `proto.DecodePrivateKey`. A stored pair can drift apart, and then someone is
  comparing two files to work out which half is stale.

**Failure reporting is the feature here.** An ssh session that fails with
"ProxyCommand failed" and nothing else is undiagnosable, and this client is the
only place the real cause is known. Three distinct errors: the relay was
unreachable (names the address), the announcement was refused, or the
counterpart never arrived (names the stream id and how long it waited). A
malformed id is refused before dialling, without echoing what it refused.

## Evidence

Green under `-race`, against a real relay and a real handshake:

- `Connect` reaches an agent through a real relay and round-trips bytes
- each failure mode produces its own error, with the address or stream id an
  operator needs
- `Splice` carries both directions and returns when the far end closes
- the key file is 0600, round-trips, and a second `keygen` refuses to overwrite
- **stdout discipline**: seven failing and informational paths run through the
  real `run()` with both streams captured, asserting stdout stayed empty and
  stderr said something. One stray byte on stdout corrupts ssh's protocol.

## Found while verifying

A client test took 60 seconds to tear down, which turned out to be a **relay
bug**: `Serve` waited out every parked connection's rendezvous deadline before
returning, so `cmd/relay` would hang up to five minutes on SIGTERM. With no
inbound port anywhere, a relay that will not restart is every deploy stopped.
`expireAfter` now selects on the server context, and `internal/relay` has a
regression test asserting shutdown does not wait out an hour-long deadline.
