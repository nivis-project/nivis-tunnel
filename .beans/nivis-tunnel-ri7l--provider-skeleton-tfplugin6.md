---
# nivis-tunnel-ri7l
title: Provider skeleton (tfplugin6)
status: todo
type: epic
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T14:56:38Z
parent: nivis-tunnel-8h0c
---

**Target repo:** `terraform-provider-nivis-tunnel`

A tfplugin6 provider that nivis can exec, with the schema for `nixos_activation`
and nothing behind it yet.

## Why

nivis resolves providers by filesystem path, exactly as it does for
`terraform-provider-hcloudimage`, so no registry round-trip is needed. Getting a
provider that nivis can start and interrogate is a milestone on its own: it
proves the plumbing before any behaviour depends on it.

## Scope

- Schema: `target` (string), `closure` (string), computed `current_system`.
- GetProviderSchema, ValidateResourceConfig, and a PlanResourceChange that
  produces a sensible diff.
- Packaged by the flake so `nix build` yields a binary nivis can exec by path.

## Acceptance

`nivis gen --provider <path>` emits a constructor for `nixos_activation`, and a
nivis plan against a domain using it evaluates without error.
