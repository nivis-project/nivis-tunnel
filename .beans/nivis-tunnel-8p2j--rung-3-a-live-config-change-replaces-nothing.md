---
# nivis-tunnel-8p2j
title: 'Rung 3: a live-config change replaces nothing'
status: in-progress
type: epic
priority: normal
created_at: 2026-09-11T14:56:57Z
updated_at: 2026-09-14T21:08:56Z
parent: nivis-tunnel-c03l
blocked_by:
    - nivis-tunnel-e3ig
---

The acceptance test the entire project exists for.

## Why

Everything else is machinery. This is the claim:

> Changing a live NixOS configuration changes **only** the activation resource.
> No new image. No new snapshot. No replaced server.

Its absence is what made the agenix enrolment in nivis-demos non-convergent
(bean `nivis-demos-x98i`): the secret lived in the image, so re-keying replaced
the server, which regenerated the identity the secret was encrypted to, forever.

## Scope

- Apply. Record the snapshot id, the server id, and the machine's boot time.
- Change `liveSystem`. Apply again.
- Assert the plan touches **only** the activation resource.
- Assert afterwards: same snapshot id, same server id, same boot time, new
  system generation.

## Acceptance

The test asserts what did **not** happen. A test that only checks
`switch-to-configuration` ran has not tested the claim — the negative is the
claim. It must fail loudly if the server was replaced.

## Then

Rung 2 becomes available and is free evidence: point elastinix's
`instance/ssh.conf` at our `ProxyCommand` instead of SSM and run its existing
scripts unchanged. Same scripts, same machine, same closure, only the transport
differs — so anything that breaks is attributable to the tunnel and nothing
else.

## OpenSpec

`tunnel-target-activation`, in the `nivis` store, implemented in nivis-demos
where `040_tunnel_target` lives.

The fallback is already verified: the EC2 serial console reaches a login prompt
on the running target. That matters because this machine has no inbound port at
all, so a failed activation has no network route back, and magic rollback
(nivis-tunnel-wnkn) is deliberately not in this change.

