---
# nivis-tunnel-6f4m
title: Adopt deploy-rs as the activation engine
status: todo
type: feature
priority: normal
created_at: 2026-09-11T14:57:56Z
updated_at: 2026-09-11T14:57:56Z
parent: nivis-tunnel-q5cc
---

Replace the three hand-rolled activation steps with deploy-rs.

## Why

deploy-rs already has the parts that are subtle to get right: profiles and
generations, auto-rollback on failed activation, and magic rollback. Two of its
options matter directly here:

- **`sshOpts`** — "An optional list of arguments that will be passed to SSH."
  Our `ProxyCommand` slots straight in, so deploy-rs over the tunnel needs no
  changes to deploy-rs at all. Same lever as elastinix's `ssh.conf`.
- **`fastConnection`** — "If this is true, copy the whole closure instead of
  letting the node substitute. This defaults to `false`." That is exactly the
  push/pull switch, already built.

## Tension to resolve

A provider shelling out to a CLI looks like local-exec in a jacket. The
difference is real — typed config, state, diff, retry and Read survive — but it
should be a deliberate choice, not a drift. Decide whether to shell out or
reimplement just the activation call.
