---
# nivis-tunnel-hilr
title: 01 Foundation
status: completed
type: milestone
priority: normal
created_at: 2026-09-11T14:54:39Z
updated_at: 2026-09-14T21:28:04Z
---

Repository scaffolding, the Nix build gate, and the end-to-end test harness that every later milestone is measured against.

## Acceptance

A green `nix flake check` from the first commit, and an e2e harness that can point at a real target without running in the sandboxed gate.

## Summary of Changes

Both epics complete. The repository builds on plain Nix flakes with explicit
supported systems and no flake-utils, `nix flake check` is the gate, and the
end-to-end harness is NixOS VM tests rather than scripts against a live
machine.

That harness is the reason the rest of this project can be trusted. Its first
run of rung 0 caught the mistake it existed to catch: the target was reachable
all along, because `services.openssh.openFirewall` defaults to true and NixOS
firewall port lists merge rather than override. Every other assertion in that
test passed while proving nothing.
