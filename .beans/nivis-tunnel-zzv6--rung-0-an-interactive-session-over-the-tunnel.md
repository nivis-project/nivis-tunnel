---
# nivis-tunnel-zzv6
title: 'Rung 0: an interactive session over the tunnel'
status: todo
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T14:58:51Z
parent: nivis-tunnel-xvjv
blocked_by:
    - nivis-tunnel-qtx9
    - nivis-tunnel-up7u
    - nivis-tunnel-1kpt
    - nivis-tunnel-ru8h
    - nivis-tunnel-mxtv
---

The acceptance test for milestone 02, against a real cloud host rather than a
local pair.

## Why

Rung 0 is the first moment the claim is tested for real: a machine with **no
inbound port** that you can nevertheless reach.

## Scope

- A host on Hetzner or AWS with the agent enabled and its cloud firewall
  allowing no inbound traffic at all.
- A relay reachable by both sides.
- `ssh -o ProxyCommand='nivis-tunnel connect %h' root@<id> uptime`.

## Acceptance

The command returns the target's uptime, and a port scan of the host's public
address shows nothing open. Record both in the change's summary — the negative
is half the claim.
