---
# nivis-tunnel-e3ig
title: 'Demo domain: bootstrap image, server, activation'
status: in-progress
type: epic
priority: normal
created_at: 2026-09-11T14:56:57Z
updated_at: 2026-09-14T16:03:50Z
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

## Reversal: this lands in nivis-demos after all

The scope above says "in this repo, disposable — **not** in nivis-demos, that
would make the PoC inherit a whole production-shaped stack." Having now read
both repositories, that reasoning does not hold.

Catstack domains are independent. Each carries its own state key and its own
resources; a fourth domain alongside the Vaultwarden ones inherits nothing from
them. The Hetzner demo that prompted the worry has since been destroyed anyway.

What the alternative actually costs is rebuilding `stackctl`, `environments/`,
the S3 state backend and the AWS conventions inside nivis-tunnel — duplicating
infrastructure to honour a sentence. nivis-demos is described in its own
AGENTS.md as "a collection of self-contained demo stacks for nivis", which is
what this is.

So: the domain is `040_tunnel_target` in **nivis-demos**, and the image pattern
follows `020_vaultwarden_ec2`, which is a working image → S3 →
`aws_ebs_snapshot_import` → `aws_ami` → `aws_instance` chain in that repo
already.

This bean stays the tracking epic; the code and the OpenSpec change live where
they are useful.
