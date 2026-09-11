---
# nivis-tunnel-8h0c
title: 04 Activation provider
status: todo
type: milestone
created_at: 2026-09-11T14:54:39Z
updated_at: 2026-09-11T14:54:39Z
---

The nixos_activation resource in the companion repo terraform-provider-nivis-tunnel. Read is what makes it a resource rather than a provisioner in disguise.

## Acceptance

Create, Read and Update work against a real target, and a manual nixos-rebuild on that target shows as drift in plan.
