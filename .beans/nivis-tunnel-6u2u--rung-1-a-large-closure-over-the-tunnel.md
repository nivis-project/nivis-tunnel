---
# nivis-tunnel-6u2u
title: 'Rung 1: a large closure over the tunnel'
status: todo
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T14:58:51Z
parent: nivis-tunnel-ou1o
blocked_by:
    - nivis-tunnel-zzv6
---

Push a full NixOS system closure through the tunnel with `nix-copy-closure`.

## Why

This is the one property nobody involved has data on. Large closures go fine
over AWS SSM — but that is Amazon's relay, engineered and operated by Amazon,
not ours. A stream that carries an interactive shell perfectly can still stall,
deadlock or corrupt under a sustained multi-gigabyte transfer.

It is also the cheapest possible test: one command, no new code, run before any
provider is built around it.

## Scope

- `NIX_SSHOPTS="-F poc.conf" nix-copy-closure root@<id> <path>` where `poc.conf`
  carries the `ProxyCommand`.
- Measure: wall time, throughput, memory on the relay.
- Repeat with the path already present, to confirm `nix copy` sends only what is
  missing — after the first deploy this is what real traffic looks like.
- Behaviour on a mid-transfer relay restart: a clean error is acceptable, silent
  truncation is not.

## Acceptance

A full system closure lands on the target and `nix-store --verify --check-contents`
passes there. Figures recorded in the change summary so the relay's cost as a
hosted service can be reasoned about later.
