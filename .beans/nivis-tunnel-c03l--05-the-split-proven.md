---
# nivis-tunnel-c03l
title: 05 The split, proven
status: completed
type: milestone
priority: normal
created_at: 2026-09-11T14:54:39Z
updated_at: 2026-09-15T07:13:21Z
---

The milestone the project exists for. A disposable demo domain showing bootstrap image, server, and activation as separate concerns.

## Acceptance

Rung 3: change the live config, apply, and observe that ONLY the activation resource changed. No new snapshot. No replaced server.

## Summary of Changes

The milestone the whole PoC exists for, and it holds against real
infrastructure rather than in a test.

A machine in eu-central-1 that admits nothing on any of its 65535 ports had its
configuration changed twice. The plan each time:

```
~ nivis-tunnel.nixos_activation.live
= the nine other resources
```

No image. No snapshot. No AMI. No replaced server. The instance id, the
snapshot id, the AMI id and the boot time are the same before and after, and
three system generations have run on the one machine.

```
image route   2649 MiB   ~15 min   machine replaced
live-1          17 MiB       51s   same machine
live-2         343 KiB        4s   same machine
```

That is the split, and that ratio is the argument for everything downstream:
the relay as a service (nivis-tunnel-bcxz) is plausible because a deploy moves
kilobytes, and the agenix enrolment in nivis-demos stops being circular
(nivis-demos-x98i) because a secret no longer has to live in an image.

What it does not yet have is a net. A failed activation on a machine with no
inbound port has no network route back, and nivis-tunnel-wnkn is still open.
The fallback used here is the EC2 serial console, verified before it was needed
rather than during a failure, and written down in the demo's README.
