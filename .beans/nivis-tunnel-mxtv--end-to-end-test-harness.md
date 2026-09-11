---
# nivis-tunnel-mxtv
title: End-to-end test harness
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T16:04:35Z
parent: nivis-tunnel-hilr
---

A PoC that cannot demonstrate its own claim has proven nothing.

## What was delivered, and why not as originally scoped

This bean specified a `test/e2e` package behind a build tag, configured from the
environment and skipping when no target is available. That shape exists to keep
network-dependent tests out of the sandboxed Nix gate without making them tests
nobody runs.

**A NixOS VM test solves the same problem better**, and is what was built. It
gets a real network, real machines and a real firewall *inside* the gate, so the
end-to-end proof runs on every `nix flake check` rather than on whoever
remembers to set the environment variables. A skipped test proves nothing; these
cannot be skipped.

So the harness landed in two pieces:

- **In-package fixtures.** `internal/agent`, `internal/tunnel` and
  `internal/relay` tests each start a real relay on a real listener and speak
  the real protocol over TCP. No mocks of the transport anywhere.
- **VM tests.** `nix/rung-0-test.nix` and `nix/rung-1-test.nix`, three machines
  each, both wired into `checks`.

## Summary of Changes

No separate `test/e2e` package. The empty directory should be removed.

Evidence the harness produces, on every gate run:

- rung 0: ssh reaches a target whose firewall admits nothing, and direct ssh to
  that target fails
- rung 1: a 16 MiB store path copied with `nix-copy-closure` arrives intact, and
  a repeat copy sends nothing

## What it still cannot reach

A real cloud target. That needs an account and credentials, and is the open half
of `nivis-tunnel-zzv6`. If that run ever becomes routine rather than a one-off,
the environment-configured tagged suite this bean originally described is the
right way to hold it.
