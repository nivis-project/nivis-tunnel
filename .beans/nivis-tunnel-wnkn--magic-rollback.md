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

## Found while implementing the activation provider

**The three steps are not atomic**, and the middle one is the problem:

```
1. nix-copy-closure          harmless on its own
2. nix-env --profile --set   the profile now points at the new generation
3. switch-to-configuration   ...which is not running yet
```

If step 3 fails, the machine is left with a system profile naming a generation
it is not running. Nothing is broken — the old system is still live — but the
profile now lies, and a later rollback reasoning from generations would reason
from a wrong one.

Reordering does not fix it: the profile has to be set before the switch, because
that is what makes the generation rollback-able in the first place. What fixes
it is knowing whether the switch took, which is exactly what magic rollback
establishes.

So this is not a separate defect to file; it is another reason this bean is the
first thing after rung 3.

## A second reason, from the same work

`switch-to-configuration` restarts every unit whose definition changed. If a
deploy ever changes the agent's unit, the switch restarts the agent — cutting
the tunnel that the activation is travelling over, mid-activation.

Real deploys over ssh have the same shape and survive it because sshd does not
kill established sessions on restart. The agent has no such property today, and
the VM test avoids the question by making its two generations differ in a file
rather than in the agent's configuration.

Whatever rollback does has to account for this: the connection carrying the
deploy can be severed by the deploy.
