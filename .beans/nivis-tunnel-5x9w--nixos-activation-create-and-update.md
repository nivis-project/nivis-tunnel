---
# nivis-tunnel-5x9w
title: 'nixos_activation: Create and Update'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T18:22:02Z
parent: nivis-tunnel-8h0c
openspec-link: openspec/changes/archive/2026-09-11-activation-read-and-apply
blocked_by:
    - nivis-tunnel-74sg
    - nivis-tunnel-6u2u
---

**Target repo:** `terraform-provider-nivis-tunnel`

Push a closure and activate it.

## Summary of Changes

OpenSpec change `activation-read-and-apply`, shared with `nivis-tunnel-74sg`.

Three steps, deliberately explicit rather than delegated to deploy-rs, so the
PoC owns its failure modes:

```
nix-copy-closure --to ssh://<target> <closure>
nix-env --profile <profile> --set <closure>
<closure>/bin/switch-to-configuration switch
```

Plus: retry until a timeout when reaching the target, because a machine created
moments ago is not yet reachable and elastinix polls for exactly this reason. A
`profile` attribute, because generations are what make rollback possible. And
**Delete does nothing to the machine** — removing a resource from state says
what is managed, not what should be torn down.

## Evidence

Ten subtests in a VM test with a relay, a target whose firewall admits nothing,
and an operator running `tofu`. Every step asserted on the machine itself:

- the target is not reachable directly, so everything below went through the tunnel
- after apply: the closure is present, the profile resolves to it, and the
  observable file the generation adds is there
- the machine is **still reachable afterwards** — a switch restarts units whose
  definitions changed, and restarting the agent would cut the tunnel the
  activation is travelling over
- changing the closure activates the other generation
- a failing activation **names the step and carries the target's own output**
- destroy leaves the generation running and the agent active

## Known gaps, recorded rather than hidden

The three steps are **not atomic**: the profile is set before the switch, so a
failed switch leaves the profile naming a generation that is not running. And a
deploy that changed the agent's unit would sever its own connection. Both are on
`nivis-tunnel-wnkn`, which is why magic rollback is the first thing after rung 3.
