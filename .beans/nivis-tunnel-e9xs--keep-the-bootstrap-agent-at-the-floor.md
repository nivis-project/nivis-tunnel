---
# nivis-tunnel-e9xs
title: Keep the bootstrap agent at the floor
status: todo
type: feature
priority: normal
created_at: 2026-09-11T14:58:36Z
updated_at: 2026-09-11T14:58:36Z
parent: nivis-tunnel-khrj
---

**Target repo:** `terraform-aws-module-elastinix` (and this repo's `nix/module.nix`)

A rule worth writing down in both places, because violating it is silent until
it is expensive.

## The rule

**The agent in the boot image only has to be good enough to accept one push.**
A newer agent arrives through the live configuration like any other package.

## Why

The agent lives in the image, so every agent change is an image rebuild and
therefore a fleet-wide machine replacement. Holding it to "accepts one push"
means the image is only rebuilt on a genuine protocol break — which is also why
`proto.Version` exists from the first commit.

## Second reason

Magic rollback only rescues you if the *previous* generation had a working
agent. Generation one comes from the image, so the image's agent is the floor
you always fall back to. It must stay dull and reliable, not current.
