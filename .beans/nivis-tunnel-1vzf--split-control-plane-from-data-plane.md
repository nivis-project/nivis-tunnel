---
# nivis-tunnel-1vzf
title: Split control plane from data plane
status: todo
type: feature
priority: low
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

Let the relay carry only the control channel while closure bulk comes from a
substituter straight to the target.

## Why

It decides whether a hosted relay is cheap or expensive to run:

```
everything over the relay       control-plane split
────────────────────────        ───────────────────
relay carries the closure       relay carries "switch to /nix/store/abc…"
first deploy ~1-2 GB            bulk comes from a binary cache → target
                                relay traffic stays negligible
```

Not a v1 requirement. It is a requirement that the protocol does not make it
impossible later.
