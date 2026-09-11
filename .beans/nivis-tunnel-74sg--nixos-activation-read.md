---
# nivis-tunnel-74sg
title: 'nixos_activation: Read'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T18:22:01Z
parent: nivis-tunnel-8h0c
openspec-link: openspec/changes/archive/2026-09-11-activation-read-and-apply
blocked_by:
    - nivis-tunnel-ri7l
    - nivis-tunnel-zzv6
---

**Target repo:** `terraform-provider-nivis-tunnel`

Read the target's live system generation over the tunnel.

## Why

This is the entire justification for the resource existing rather than a
`null_resource` with `triggers`. `triggers` knows only what it did last time; it
cannot tell you that someone ran `nixos-rebuild` on the box by hand.

## Summary of Changes

OpenSpec change `activation-read-and-apply`, shared with `nivis-tunnel-5x9w`.
**Two beans, one change, deliberately**: Read cannot be demonstrated without
something to read, and shipping it alone would have meant archiving it on unit
tests — the kind of proof this project keeps refusing.

- `readlink /run/current-system` over the tunnel, reported as `current_system`.
- An unreachable target is an **error, not an absent resource**. Reporting the
  resource as gone would provoke a recreation and activate a closure on a
  machine that may already have it, for no better reason than a reboot.

## The gap Read alone would have left

`current_system` is computed, so a refresh discovering a different generation
would have updated state quietly and the next plan would have reported no
changes. The resource would have *known* about the drift and done nothing —
precisely `null_resource` behaviour, and the opposite of why this is a resource.

`ModifyPlan` closes it: if what is running differs from what is configured, for
any reason, an apply is owed. That comparison is the whole convergence rule.

## Evidence

In a VM test where a real OpenTofu drives the provider against a target reached
only through the tunnel:

- `current_system` reads back as the generation just activated
- a second plan is **empty** when the machine matches the configuration
- **switching the target by hand makes `tofu plan` exit 2** — drift becomes a
  planned change rather than a silent overwrite
- applying it converges the machine back
