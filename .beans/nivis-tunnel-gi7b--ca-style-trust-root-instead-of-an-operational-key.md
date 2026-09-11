---
# nivis-tunnel-gi7b
title: CA-style trust root instead of an operational key in the image
status: todo
type: feature
priority: high
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

Bake a long-lived trust root into the boot image and sign operational keys with
it, rather than baking the working orchestrator key directly.

## Why

With the operational public key in the image, **rotating it is an image
rebuild** — and an image rebuild is a fleet-wide machine replacement, the exact
cost this project exists to remove. With a trust root that signs short-lived
operational keys, rotation never touches the image.

Same distinction an ssh CA makes. Cheap to decide now, painful to retrofit,
because the image is precisely the thing you cannot cheaply change.
