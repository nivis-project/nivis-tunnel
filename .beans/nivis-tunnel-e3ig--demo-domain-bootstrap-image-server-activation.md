---
# nivis-tunnel-e3ig
title: 'Demo domain: bootstrap image, server, activation'
status: todo
type: epic
priority: normal
created_at: 2026-09-11T14:56:57Z
updated_at: 2026-09-11T14:58:51Z
parent: nivis-tunnel-c03l
blocked_by:
    - nivis-tunnel-5x9w
---

A disposable nivis domain in this repo demonstrating the split. **Not** in
nivis-demos — that would make the PoC inherit a whole production-shaped stack.

## Why

The split has to be expressible in nivis before it can be proven. The shape it
takes:

```nix
osImage    = mkResource { ... };              # bootstrap, changes almost never
liveSystem = mkLiveSystem { ... };            # the real configuration

activation = mkResource {
  provider = "nivis-tunnel";
  type     = "nixos_activation";
  config = {
    target  = server.refAttr "id";   # a ref -> ordering after the server
    closure = drv liveSystem;        # a __build leaf -> nivis realises it
  };
};
```

Nothing in nivis needs to change for this. `--build` already realises `__build`
leaves before apply, so the diff falls out of the store path.

## Scope

- A minimal bootstrap NixOS configuration: boot, network, the agent, nothing
  else. Everything a machine could otherwise need goes in `liveSystem`.
- A live configuration carrying something visibly changeable, so rung 3 has an
  observable effect.
- Documented in the README as the worked example.

## Acceptance

`nivis apply` produces a booted server reachable over the tunnel with the live
configuration active.
