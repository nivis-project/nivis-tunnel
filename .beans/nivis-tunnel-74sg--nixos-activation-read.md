---
# nivis-tunnel-74sg
title: 'nixos_activation: Read'
status: todo
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T14:58:51Z
parent: nivis-tunnel-8h0c
blocked_by:
    - nivis-tunnel-ri7l
    - nivis-tunnel-zzv6
---

**Target repo:** `terraform-provider-nivis-tunnel`

Read the target's live system generation over the tunnel.

## Why

This is the entire justification for the resource existing rather than a
`null_resource` with `triggers`, or a shelled-out provisioner. `triggers` knows
only what it did last time; it cannot tell you that someone ran `nixos-rebuild`
on the box by hand. Read can:

```
readlink /run/current-system   →  the generation actually running
```

Implement Read before Create. It is the harder half, it is what makes the
resource honest, and building it first stops it being quietly dropped once
Create works.

## Scope

- ssh through the `ProxyCommand` to `readlink /run/current-system`.
- Unreachable target: a diagnostic that names the reason, not a nil state that
  silently recreates the resource.
- Retry with timeout — a machine that has just booted is not yet reachable, and
  elastinix polls for exactly this reason.

## Acceptance

`nivis refresh` reports drift after a manual `switch-to-configuration` on the
target, and reports none after an apply.
