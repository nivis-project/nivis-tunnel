---
# nivis-tunnel-ou1o
title: 03 Closure transport
status: completed
type: milestone
priority: normal
created_at: 2026-09-11T14:54:39Z
updated_at: 2026-09-14T21:28:04Z
---

Prove the tunnel carries what it exists to carry. Large closures go fine over AWS SSM, but that is Amazon's relay, not ours, and nobody has measured ours.

## Acceptance

Rung 1: nix-copy-closure pushes a full system closure through the tunnel without stalling.

## Summary of Changes

Complete, in a VM and then on real infrastructure.

16 MiB byte-identical in a second across a relay in Nuremberg and a machine in
Frankfurt; a repeated copy sending nothing; a dependency closure copied and
then executed on the target.

The number that matters for everything downstream: of the 698 paths in a real
live system, a running target already had 665. A deploy moves 17 MiB. The same
change delivered as an image moves 2649 MiB and a quarter of an hour.
