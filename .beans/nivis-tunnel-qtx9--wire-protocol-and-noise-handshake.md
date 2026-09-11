---
# nivis-tunnel-qtx9
title: Wire protocol and Noise handshake
status: todo
type: epic
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T14:56:02Z
parent: nivis-tunnel-xvjv
---

The protocol is the reason relay, agent and cli share one repository. Getting
its shape right early is worth more than any other decision here.

## Why

Two things must be true from the first release, because retrofitting either is
a fleet-wide image rebuild:

1. **Version negotiation.** `proto.Version` already exists; the handshake has to
   actually negotiate it. Without that, every protocol change is a breaking
   change, and every breaking change rebuilds the boot image of every machine.
2. **End-to-end Noise.** The relay must never be able to read what it carries.
   That is what makes it safe to host, trivial to self-host, and honest to offer
   as a service.

## Scope

- Rendezvous frame: version, stream id, role (agent or orchestrator).
- Noise handshake between orchestrator and agent, relay excluded.
- The agent accepts exactly one peer: the orchestrator public key baked into its
  configuration. The target proves nothing about itself — it is identified by
  the address the cloud API returned, the same trust model `ssh-keyscan`
  already relies on.
- Reject malformed stream ids before they reach a map key or a log line.

## Acceptance

Unit tests for framing, version mismatch, and a handshake refused when the
orchestrator key does not match. A recorded transcript proving the relay sees
only ciphertext.
