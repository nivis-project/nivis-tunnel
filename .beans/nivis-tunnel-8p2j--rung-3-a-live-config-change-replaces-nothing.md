---
# nivis-tunnel-8p2j
title: 'Rung 3: a live-config change replaces nothing'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:57Z
updated_at: 2026-09-15T07:13:02Z
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


## Summary of Changes

Proven against `i-01415faf5b5b44b99`, a t3.micro in eu-central-1 built from
`040_tunnel_target` in nivis-demos, reached through the relay on durer.

The instance id, the snapshot id, the AMI id and the machine's boot time were
recorded before anything was applied. Then a live configuration change:

```
plan:  ~ nivis-tunnel.nixos_activation.live
       = the nine other resources
       1 change(s) across 10 resource(s)
```

Afterwards all four recorded values are unchanged, the machine has not
rebooted, and a third system generation is running. Two activations, two new
generations, the same machine.

### The figures

```
image route   2649 MiB   ~15 min   machine replaced
live-1          17 MiB       51s   same machine
live-2         343 KiB        4s   same machine
```

The 4s is the activation itself; the surrounding apply took 46s, of which 42
were nivis refreshing nine AWS resources.

### The negative, after the fact

```
nmap -Pn -p- 51.102.104.160
  Not shown: 65535 filtered tcp ports (no-response)
```

Every port, scanned after both activations. The changes arrived and nothing was
opened to let them.

### What the run taught

The agent stayed up throughout, and the session that verified it ran through
that same agent. That is `live.nix` importing the image's configuration rather
than standing beside it: the unit is the same store path, so a switch leaves it
alone. Had the live system been assembled independently it would have stopped
the agent and severed the connection the activation arrived over, on a machine
with no second route in.

Three defects surfaced getting there, all of them outside this repo:

- nixform2-1mk0: nivis sends unset optional-computed attributes as unknown
  where Terraform sends null, so a provider's schema defaults never fire. Two
  applies lost to an empty `tunnel_command`.
- nivis-tunnel-p22p: the reachability wait retries what waiting cannot fix, so
  both of those took three minutes to report a fault visible in the first
  second.
- A store path in an IR is a string nothing realises. Fixed in nivis-demos by
  making the client a `__build` leaf, which removes the class rather than the
  instance.

Still open and now more clearly needed: nivis-tunnel-wnkn (magic rollback).
Nothing went wrong on the machine this time, but nothing would have caught it
if it had.
