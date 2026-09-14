---
# nivis-tunnel-8h0c
title: 04 Activation provider
status: completed
type: milestone
priority: normal
created_at: 2026-09-11T14:54:39Z
updated_at: 2026-09-14T21:28:04Z
---

The nixos_activation resource in the companion repo terraform-provider-nivis-tunnel. Read is what makes it a resource rather than a provisioner in disguise.

## Acceptance

Create, Read and Update work against a real target, and a manual nixos-rebuild on that target shows as drift in plan.

## Summary of Changes

All three epics complete. The provider speaks tfplugin6 and implements Create,
Read and Update for `nixos_activation`.

Read is what makes it a resource rather than a provisioner: `current_system` is
followed from `/run/current-system` on the machine instead of being remembered
from the last apply, and `ModifyPlan` turns a difference between what is
running and what is configured into a planned update. A manual `nixos-rebuild`
on the target therefore shows up as drift. Delete leaves the machine alone,
because a machine is not the activation's to destroy.

Proven in its own VM test: apply, read back, drift becomes a plan exit 2,
converge, a failure carries the target's own output, and destroy touches
nothing.

The remaining question is not whether it works but whether it is safe to use
without a net, which is `nivis-tunnel-wnkn`.
