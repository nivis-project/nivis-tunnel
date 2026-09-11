---
# nivis-tunnel-6u2u
title: 'Rung 1: a large closure over the tunnel'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T16:04:08Z
parent: nivis-tunnel-ou1o
openspec-link: openspec/changes/archive/2026-09-11-tunnel-closure-transport
blocked_by:
    - nivis-tunnel-zzv6
---

Push a full Nix closure through the tunnel with `nix-copy-closure`.

## Why

This was the one property nobody involved had data on. Large closures go fine
over AWS SSM — but that is Amazon's relay, engineered and operated by Amazon,
and ours is a few hundred lines old. A transport that carries an interactive
shell perfectly can still stall, deadlock or corrupt under sustained transfer,
and that failure shows up as a deploy that hangs rather than one that errors.

## Summary of Changes

OpenSpec change `tunnel-closure-transport`, capability
`tunnel-closure-transport`. **No production code changed.** The transport was
already built; this establishes it is fit for its payload. Had it needed code,
the transport would have been wrong.

`nix/rung-1-test.nix`: a three-machine NixOS VM test. The payload is created
inside the orchestrator with `nix-store --add` at test time, because NixOS test
nodes share the host store — copying any pre-existing path would transfer
nothing and prove nothing. The test asserts the target does not have it first.

## Evidence

```
rung 1 payload: /nix/store/rlkajqpr...-payload (16 MiB,
                sha256 ec390dd6d5adfe6f996afec2f53f7caf84ce8fefe8af78c715273b8241df2858)
rung 1: 16 MiB closure copied in 1s
```

- the target admits nothing directly (10s timeout proving it), so a successful
  copy can only have gone through the tunnel
- the path is absent from the target before the copy
- `nix-copy-closure --to` through the `ProxyCommand` succeeds
- the arrived path hashes to the same value as the source
- **a repeated copy sends nothing further** — what makes every deploy after the
  first small, and therefore what decides whether a relay is cheap or expensive
  to operate

## Figures, for when relay cost is reasoned about

16 MiB in roughly one second inside a VM on a local bridge. That is not a
throughput measurement — there is no real network here — but it establishes
there is no stall, no deadlock and no corruption in the path, which was the
question. Real figures need `nivis-tunnel-zzv6`'s cloud run.
