---
# nivis-tunnel-xvjv
title: 02 Tunnel transport
status: completed
type: milestone
priority: normal
created_at: 2026-09-11T14:54:39Z
updated_at: 2026-09-14T21:28:04Z
---

The transport itself: wire protocol, Noise handshake, relay, agent, and the ssh ProxyCommand client. This is the only layer of the whole project that is genuinely unproven.

## Acceptance

Rung 0 of the test ladder: an interactive ssh session over the tunnel to a host with no inbound port.

## Summary of Changes

All six epics complete, and the milestone's claim now holds against real
infrastructure rather than only in a VM.

The protocol is `Noise_XK_25519_ChaChaPoly_BLAKE2s` with the agent as
initiator. It began as `Noise_KX`, which was revised because the responder got
no handshake-time signal of who it was talking to. The test written from the
spec is what surfaced that, before any of it was deployed.

Proven against a t3.micro in eu-central-1 with no ingress rules, through the
relay on durer:

```
nmap -Pn -p- 51.102.104.160
  Not shown: 65535 filtered tcp ports (no-response)

ssh -o ProxyCommand='nivis-tunnel connect poc-target-aws-01 ...' root@...
  poc-target-aws-01
```

The negative was recorded first, deliberately. A tunnel that works proves
nothing if you have not established that nothing else does.
