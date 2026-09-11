---
# nivis-tunnel-1kpt
title: Agent and NixOS module
status: todo
type: epic
created_at: 2026-09-11T14:56:03Z
updated_at: 2026-09-11T14:56:03Z
parent: nivis-tunnel-xvjv
---

The agent runs on the target: dials out to the relay, waits, completes the Noise
handshake as responder, and splices the stream onto local ssh.

## Why

This is the piece that goes into the boot image, which gives it the strictest
change budget in the system. On NixOS the kernel, initrd and bootloader all live
in the closure, so `switch-to-configuration` can replace them; only partition
layout, filesystem, boot mode and this agent genuinely force an image rebuild.

**So the agent in the image only has to be good enough to accept one push.** A
newer agent arrives through the live configuration like any other package. Keep
it dull.

## Scope

- Dial out, reconnect with backoff, survive network changes and relay restarts.
- Accept only the configured orchestrator key.
- Splice to `localhost:22`, port configurable.
- Flesh out `nix/module.nix`: it declares the options already, it needs to be
  exercised by a NixOS VM test.

## Acceptance

A NixOS VM test boots a machine with the module enabled, points it at a local
relay, and an ssh session completes over the tunnel.
