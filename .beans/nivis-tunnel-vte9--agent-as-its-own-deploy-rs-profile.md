---
# nivis-tunnel-vte9
title: Agent as its own deploy-rs profile
status: todo
type: feature
priority: low
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

Deploy the agent independently of the system closure.

## Why

deploy-rs supports several profiles per node, not just `system`:

```
/nix/var/nix/profiles/system        the OS
/nix/var/nix/profiles/nivis-agent   the agent, own generations, own rollback
```

Updating the agent would then not require a new system generation, reducing how
often the boot image is the bottleneck.

## Why it is not PoC work

On NixOS the agent is normally a systemd service defined in the system config,
so it is part of that one closure. Making it a separate profile means lifting it
out: its own unit, its own package, its own activation script. Real work, and an
unusual shape. Right tool, wrong moment.
