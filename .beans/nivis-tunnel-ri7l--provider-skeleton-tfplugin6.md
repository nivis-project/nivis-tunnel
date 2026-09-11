---
# nivis-tunnel-ri7l
title: Provider skeleton (tfplugin6)
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T16:13:08Z
parent: nivis-tunnel-8h0c
openspec-link: openspec/changes/archive/2026-09-11-activation-provider-skeleton
---

**Target repo:** `terraform-provider-nivis-tunnel`

A tfplugin6 provider that nivis can exec, with the schema for `nixos_activation`
and nothing behind it yet.

## Why

Getting a provider that an external tool can start and interrogate is a
milestone on its own: it proves the plumbing before any behaviour depends on it,
the same reason the build gate was made green before a line of protocol existed.

## Summary of Changes

OpenSpec change `activation-provider-skeleton`, capability
`activation-provider`.

- `internal/provider` on `terraform-plugin-framework` v1.19.0, the version
  `terraform-provider-hcloudimage` uses, so the two providers in this
  organisation read alike.
- `nixos_activation` with required `closure`, `stream_id`, `relay`, `key_file`
  and computed `current_system`. `stream_id` carries `RequiresReplace`: a
  different target is a different resource, not an update to this one.
- Connection settings sit on the resource rather than the provider, because one
  orchestrator routinely reaches targets behind different relays and hoisting
  the relay would force a provider alias per relay for nothing.
- **Every operation refuses.** Create, Read, Update and Delete return an error
  naming themselves. A provider that reports success while doing nothing writes
  state describing a machine nobody configured, and the next plan believes it.

## Evidence

Nine unit tests, plus a check in which **a real OpenTofu starts the built binary
and reads its schema over tfplugin6**:

```
--- the resource is offered over tfplugin6 ---
  closure: ok
  stream_id: ok
  relay: ok
  key_file: ok
  current_system: ok
--- required attributes are required ---
--- a missing required attribute is refused by name ---
```

The unit tests assert what the Go code intends; the OpenTofu check asserts what
an external tool actually receives, which is a different claim and the one that
matters. It uses a filesystem mirror rather than a dev override, because
`tofu providers schema` needs an initialised lock file and dev overrides bypass
init — and a mirror needs no network, which is what lets it run in the sandbox.

## Found while verifying

OpenTofu resolves `resource "nixos_activation"` by looking up the local name
`nixos` in `required_providers`. A configuration declaring it as `nivis-tunnel`
sends OpenTofu to the public registry hunting for `hashicorp/nixos`. The local
name must be the resource prefix — the same shape `google-beta` uses, whose
resources are `google_*`.
