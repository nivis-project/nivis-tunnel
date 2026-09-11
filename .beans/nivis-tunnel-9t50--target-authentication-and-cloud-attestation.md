---
# nivis-tunnel-9t50
title: Target authentication and cloud attestation
status: todo
type: feature
priority: normal
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

In the PoC the target proves nothing: it is whoever claimed the stream id, and
trust flows one way — the agent authenticates the orchestrator, not the reverse.

## Why it is tolerable now

It matches the trust model `ssh-keyscan` already relies on: the machine is
identified by the address the cloud API returned, and that API is trusted.

## Why it is not enough later

Anyone who can reach the relay can claim an id and receive a closure push
intended for another machine.

## Options, and their asymmetry

- AWS publishes a signed instance identity document.
- Hetzner has no equivalent.

So attestation is uneven across clouds, which cuts against the project's whole
selling point of uniformity. A one-time join token delivered through `user_data`
(a first-class attribute on both `hcloud_server` and EC2) is the uniform option,
and is how Tailscale auth keys already work.
