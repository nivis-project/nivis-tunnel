---
# nivis-tunnel-up7u
title: 'Relay: rendezvous and splice'
status: todo
type: epic
priority: normal
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T15:03:09Z
parent: nivis-tunnel-xvjv
blocked_by:
    - nivis-tunnel-qtx9
---

The relay matches two connections by stream id and copies bytes between them. It
performs no cryptography and authenticates nobody.

## Why

Keeping the relay dumb is a design property, not laziness. Because Noise runs
end to end, a relay operator sees only bytes it cannot attribute — so hosting
one carries no trust, and anyone can run their own.

## Scope

- Listen, read a rendezvous frame, park the connection under its stream id.
- When both roles arrive for an id, splice them bidirectionally until either
  closes.
- Time out parked connections; never let an unmatched id pin memory.
- Reject a second claimant for an id already paired.
- Structured logs that never contain an unvalidated stream id.

## Acceptance

Unit tests for pairing, unmatched timeout, duplicate claim, and clean teardown
when one side disappears mid-stream. A measured note on throughput, since rung 1
depends on it.
