---
# nivis-tunnel-hyey
title: Land the image/live-config split in nivis itself
status: todo
type: feature
priority: high
created_at: 2026-09-11T14:58:36Z
updated_at: 2026-09-11T14:58:36Z
parent: nivis-tunnel-khrj
---

**Target repo:** `nivis` (`/home/pim/gh.nivis-project/nivis`)

The PoC proves the split with an external provider. Whether it should become a
first-class nivis concept is a separate question.

## The case for leaving it external

Nothing in nivis needs to change. `--build` already realises `__build` leaves
before apply — "realise Nix build outputs (drv leaves) referenced by resources"
— so a provider receiving an already-realised store path gets its diff for free.
The provider route also works in plain OpenTofu.

## The case for making it native

A first-class activation node could expose a real diff and a real Read in
`plan`, rather than a provider's approximation. And if the *transport* were a
nivis capability, every provider could use it — reading state from a private
host, health checks — where a provider cannot share its own transport.

## Decide after rung 3

Building it natively first would mean forking nivis's core to discover whether
the idea holds.
