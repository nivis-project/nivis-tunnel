---
# nivis-tunnel-mxtv
title: End-to-end test harness
status: todo
type: epic
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T14:56:02Z
parent: nivis-tunnel-hilr
---

A PoC that cannot demonstrate its own claim has proven nothing. Every later
milestone ends in an acceptance test, so the harness has to exist before them.

## Why

The e2e tests need a network and a real target, which the Nix sandbox has
neither of. They must sit outside `nix flake check` without becoming tests
nobody runs.

## Scope

- `test/e2e` behind the `e2e` build tag, so `go test ./...` and the Nix gate
  stay clean while `go test ./test/e2e -tags=e2e` runs the real thing.
- Configuration from the environment: relay address, target stream id, ssh
  identity. Skip with a clear message when unset rather than failing.
- A fixture that brings up a relay and an agent locally, so rung 0 can be
  exercised on one machine before any cloud is involved.
- Helpers that assert on *observable outcomes* — a command ran on the target, a
  generation changed — never on internal state.

## Acceptance

`go test ./test/e2e -tags=e2e` runs green against a local relay+agent pair, and
skips cleanly with an explanatory message when no target is configured.
