---
# nivis-tunnel-2oep
title: Provider warning diagnostics are discarded
status: todo
type: bug
priority: high
created_at: 2026-09-11T14:58:36Z
updated_at: 2026-09-11T14:58:36Z
parent: nivis-tunnel-khrj
---

**Target repo:** `nivis` (`/home/pim/gh.nivis-project/nivis`)
**Already filed there as:** `nixform2-8btx`

Recorded here because it cost real time during the investigation that produced
this project, and because this project will hit it too.

## The defect

`normDiags` maps every non-ERROR diagnostic to `provider.SeverityWarning`
(`internal/provider/v6/v6.go:421`, `internal/provider/v5/v5.go:414`). Nothing
ever reads that severity again — `grep -rn SeverityWarning` returns three hits:
those two assignments and the const declaration in
`internal/provider/provider.go:17`. `DiagError` filters to `SeverityError` only,
so warnings from Plan/Apply/Read/ReadDataSource/Destroy are dropped on every
path.

`--provider-log-level` does not cover it: that surfaces the provider's hclog
stream, and protocol diagnostics are a different channel.

## What it cost

Applying a Hetzner domain failed with an opaque 422 naming `assignee_id` and
`location`. The provider had already said exactly what was wrong, as a warning:

    The 'datacenter' attribute is marked for removal since 'v1.67.0',
    you must use the 'location' attribute instead.

nivis dropped it. Diagnosing a one-line deprecation took reading the schema out
of the provider binary with `strings`.

## Relevance here

This project ships a provider. Any warning it emits will be invisible until
this is fixed.
