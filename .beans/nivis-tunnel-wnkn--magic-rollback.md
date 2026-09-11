---
# nivis-tunnel-wnkn
title: Magic rollback
status: todo
type: feature
priority: high
created_at: 2026-09-11T14:57:56Z
updated_at: 2026-09-11T14:58:51Z
parent: nivis-tunnel-q5cc
blocked_by:
    - nivis-tunnel-8p2j
---

Auto-revert an activation the deployer cannot confirm, deploy-rs style.

## Why this is the first thing after rung 3

A no-inbound-port design removes your own way back in. If a deploy breaks the
agent or networking, the machine is unreachable and there is nothing to fix it
with — during the PoC the public address is still the fallback, and that is the
only reason the gap is tolerable now.

deploy-rs already solves it and is worth copying exactly:

```
activate → target sets an inotify watcher under tempPath
        → deployer reconnects and confirms within confirmTimeout (30s default)
        → no confirmation ⇒ the machine rolls itself back
```

The machine tests whether you can still reach it, and heals itself when the
answer is no.

## Two subtleties

- **The agent floor.** Rollback only saves you if the *previous* generation had
  a working agent. Generation one comes from the boot image, which is why that
  agent must stay dull and always able to accept a push.
- **Reboots escape the net.** `switch-to-configuration switch` does not reboot,
  so a new kernel is not actually exercised before confirmation. Anything needing
  a reboot is outside the guarantee.

## Also

Measure `confirmTimeout` against our transport. 30s is comfortable for SSM;
re-establishing a persistent agent connection through our own relay may not fit
inside it.
