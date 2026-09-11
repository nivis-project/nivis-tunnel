---
# nivis-tunnel-up7u
title: 'Relay: rendezvous and splice'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T15:30:26Z
parent: nivis-tunnel-xvjv
openspec-link: openspec/changes/archive/2026-09-11-tunnel-relay
blocked_by:
    - nivis-tunnel-qtx9
---

The relay matches two connections by stream id and copies bytes between them. It
performs no cryptography and authenticates nobody.

## Why

Keeping the relay dumb is a design property, not laziness. Because Noise runs
end to end, a relay operator sees only bytes it cannot attribute — so hosting
one carries no trust, and anyone can run their own.

## Summary of Changes

OpenSpec change `tunnel-relay`, capability `tunnel-relay`.

- `internal/relay` implements the server: accept, read a rendezvous frame under
  a deadline, park by (stream id, role), splice when both roles are present.
- `cmd/relay` is a real binary with flags for listen address, both deadlines,
  the parked bound and log level, plus deliberate shutdown on SIGINT/SIGTERM.
  A relay outage stops every deploy against it, so it drains live sessions
  rather than dropping them.

The engineering problem was never pairing — it was surviving clients it cannot
authenticate:

- **A taken role is never displaced.** Stream ids are cloud instance ids:
  guessable and not secret. Replacing an incumbent would let anyone who can
  reach the relay evict a live session, so a duplicate claim is refused and the
  incumbent is left carrying bytes.
- **Parked connections expire** and release their id, so a party whose
  counterpart never arrives cannot pin a slot.
- **A parked bound refuses rather than absorbs**, so an unmatched flood cannot
  take down the sessions that are working.
- **Pairing and expiry race by construction.** A `sync.Once` per parked party
  settles it: exactly one of the two claims the connection, and the loser closes
  nothing.
- **Rejected stream ids never reach a log record.** A test writes an id
  containing a forged `level=INFO msg=...` line and asserts it does not appear
  in the captured log output.

## Evidence

Twelve tests against a real listener and real TCP connections, all green under
`-race`:

- bytes flow both ways; closing one side ends the session; the id is reusable
  afterwards
- a duplicate claim is refused and the incumbent still carries bytes
- an unmatched party is released and unparked; a silent connection is closed
  without ever taking an id
- a flood beyond the bound is refused while parked connections stay usable
- a wrong-protocol connection is refused promptly rather than at the deadline
- 25 pair-and-close cycles leave the parked count at zero
- **end to end**: a real Noise handshake between two parties through a real
  relay, exchanging a secret, with the relay's copied bytes asserted not to
  contain it
