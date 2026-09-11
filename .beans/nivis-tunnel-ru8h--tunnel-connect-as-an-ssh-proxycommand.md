---
# nivis-tunnel-ru8h
title: tunnel connect as an ssh ProxyCommand
status: todo
type: epic
priority: normal
created_at: 2026-09-11T14:56:03Z
updated_at: 2026-09-11T15:03:09Z
parent: nivis-tunnel-xvjv
blocked_by:
    - nivis-tunnel-qtx9
---

`nivis-tunnel connect <id>` dials the relay, completes the Noise handshake as
initiator, and pipes the stream to stdin/stdout.

## Why

This is the entire integration surface of the project. Because ssh speaks to a
`ProxyCommand` over stdio, everything above it — `nix-copy-closure`,
`switch-to-configuration`, `deploy-rs`, plain interactive ssh — works unchanged.
Elastinix proves the point from the other direction: its whole deploy path hangs
off one `ProxyCommand` line pointing at AWS SSM.

## Scope

- Strictly stdio: nothing on stdout that is not stream data, diagnostics to
  stderr only.
- Non-zero exit and a legible message when the relay is unreachable, the id is
  unknown, or the handshake is refused.
- Honour a timeout; ssh hanging forever on a dead relay is a bad failure.

## Acceptance

`ssh -o ProxyCommand='nivis-tunnel connect %h' root@<id> uptime` succeeds against
a local relay+agent pair.
