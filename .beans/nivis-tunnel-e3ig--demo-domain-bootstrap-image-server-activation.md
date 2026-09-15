---
# nivis-tunnel-e3ig
title: 'Demo domain: bootstrap image, server, activation'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:57Z
updated_at: 2026-09-15T07:13:02Z
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

## Status: the image and the server half is done, the activation half is not

`040_tunnel_target` exists in nivis-demos and is applied. A t3.micro
(`i-01415faf5b5b44b99`) boots the bootstrap image, the agent dials out to the
relay on durer by itself, and the machine is reachable over the tunnel while
admitting nothing: direct ssh to its public address times out and its security
group has no ingress rules.

Both rungs hold there. Rung 0 is an interactive session; rung 1 is a closure
pushed over the tunnel, 16 MiB byte-identical in 1s, a repeat sending nothing,
and a real dependency closure copied and then executed on the target.

What the acceptance still asks for is "with the live configuration active", and
that is the half that does not exist yet. `/etc/tunnel-target-generation` on
the machine still reads `bootstrap`, which is the marker put there so a later
activation has something observable to change.

So what remains is the `nixos_activation` resource in the domain, which is
nivis-tunnel-8p2j (rung 3). This bean closes when a changed `liveSystem`
produces a plan in which only the activation resource differs: no new snapshot,
no new AMI, no replaced server.

### Worth carrying into that work

The image chain cost about a quarter of an hour per change and broke in three
separate ways before it came up. That is not incidental to the PoC, it is the
thing the PoC exists to remove: after the activation resource, only the
partition layout, the filesystem, the boot mode and the agent itself should
ever force that chain to run again.


## Summary of Changes

Complete. The domain is `040_tunnel_target` in nivis-demos, and the acceptance
this bean asked for, a booted server reachable over the tunnel with the live
configuration active, holds: `/etc/tunnel-target-generation` reads `live-2`
where it read `bootstrap`, and nginx serves that value on the machine.

The OpenSpec change is `tunnel-target-activation`, in the `nivis` store,
following `tunnel-target-domain` which built the machine.

The measurements are on nivis-tunnel-8p2j, which was the other half of this and
closes with it. The one line worth repeating here: a live change moved 343 KiB
in 4 seconds, where the image route moves 2649 MiB and a quarter of an hour.
