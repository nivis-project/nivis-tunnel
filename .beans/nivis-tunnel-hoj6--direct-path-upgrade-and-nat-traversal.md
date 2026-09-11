---
# nivis-tunnel-hoj6
title: Direct path upgrade and NAT traversal
status: todo
type: feature
priority: low
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

Relay-only is v1. Later, upgrade to a direct path where both ends can reach each
other, with the relay as fallback — the shape Tailscale uses with DERP.

## Why it is deliberately not in the PoC

Where the target has a public address, the orchestrator could dial it directly
over a single silent UDP port; a Noise-style listener does not answer packets
without a valid handshake, so it is invisible to scanning. That removes the
relay entirely for the easy topology.

But the orchestrator is often the constrained end — a laptop on a hotel network,
a CI runner in a corporate VPC — where outbound TCP/443 is reliable and outbound
UDP to an arbitrary port is not. Relay-over-443 is simply more dependable, and
one code path is cheaper than two while the idea is still being proven.

## Cost this eventually addresses

Relay bandwidth. `nix copy` sends only missing paths, so after the first deploy
traffic is small — but the first deploy is a full system closure.
