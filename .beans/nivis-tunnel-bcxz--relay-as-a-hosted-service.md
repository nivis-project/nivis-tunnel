---
# nivis-tunnel-bcxz
title: Relay as a hosted service
status: todo
type: feature
priority: low
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

Run the relay as a service for the Nivis Project, self-hostable.

## Why it is a credible offer

The relay is **untrusted by construction**. Noise runs end to end; it splices
bytes it cannot read and cannot attribute. So the pitch is "we cannot read your
closures, we do not hold your identities, and you can host it yourself" — a
conversation Tailscale cannot have, because their coordination server does hold
the key directory.

## Scope when it comes

Multi-tenancy, quotas, abuse handling, and an availability story: with no
inbound port, a relay outage means no deploys at all.
