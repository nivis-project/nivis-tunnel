---
# nivis-tunnel-23mz
title: Persistent versus ephemeral connection policy
status: todo
type: feature
priority: normal
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

Decide and document what stays connected, and why.

## The constraint that settles it

You cannot have both **no inbound port** and **no persistent connection**:

```
no inbound port ⇒ the target must dial out
                ⇒ that connection must already exist when you want to deploy
                ⇒ persistent agent connection
```

SSM resolves this by splitting them, and that is the model to copy:

```
agent connection   persistent   (long-poll to the service, always up)
session            ephemeral    (established per connect, torn down after)
```

## What it costs

```
ephemeral sessions   relay load scales with deploys
persistent agents    relay load scales with FLEET SIZE
```

That is the difference between a service you run on the side and one you staff.
Tailscale is persistent, which is why their DERP fleet is what it is.

## What it does not cost

Drift detection. `refresh` happens during plan/apply, not continuously, so Read
only needs the machine during a run.
