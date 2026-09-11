---
# nivis-tunnel-5x9w
title: 'nixos_activation: Create and Update'
status: in-progress
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T16:14:30Z
parent: nivis-tunnel-8h0c
blocked_by:
    - nivis-tunnel-74sg
    - nivis-tunnel-6u2u
---

**Target repo:** `terraform-provider-nivis-tunnel`

Push a closure and activate it.

## Why

Three steps, deliberately kept explicit rather than delegated to `deploy-rs` for
now, so the PoC owns its failure modes. Adopting deploy-rs as the engine is a
separate, later decision — see the hardening milestone.

## Scope

```
nix copy --to ssh://<target> <closure>
nix-env --profile /nix/var/nix/profiles/system --set <closure>
<closure>/bin/switch-to-configuration switch
```

- The closure arrives as a store path nivis already realised from a `__build`
  leaf, so the diff is free: *the store path changed* **is** the change.
- Root is a trusted user, so store path signature checking does not apply. It
  would if a non-root deploy user were ever used — note it, do not build for it.
- Activation failure must surface the target's stderr, not a generic error.

## Acceptance

An end-to-end test changes a live configuration, applies, and asserts the new
generation is live by reading it back. A deliberately broken configuration fails
the apply with the target's own error message visible.

## Known gap

No rollback. A configuration that breaks networking or the agent strands the
machine, and in the PoC the public address is the only way back. Magic rollback
is tracked separately and is the highest-priority item after rung 3.
