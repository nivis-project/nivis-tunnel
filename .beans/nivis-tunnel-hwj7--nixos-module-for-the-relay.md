---
# nivis-tunnel-hwj7
title: NixOS module for the relay
status: completed
type: epic
priority: high
created_at: 2026-09-14T14:48:32Z
updated_at: 2026-09-14T14:53:41Z
parent: nivis-tunnel-xvjv
openspec-link: openspec/changes/archive/2026-09-14-relay-nixos-module
---

The relay had a binary and a flake package but no NixOS module, so running one
meant hand-writing a systemd unit — what the agent module exists to avoid on the
other side.

## Summary of Changes

OpenSpec change `relay-nixos-module`.

- `nix/relay-module.nix`: `services.nivis-tunnel-relay` with package, address,
  port, both timeouts, parked bound and log level.
- Exported as `nixosModules.relay`; `nixosModules.default` stays the agent, so
  existing consumers are unaffected.
- Hardened to what the relay actually needs, which is nothing: `DynamicUser`,
  empty capability bounding set, `ProtectSystem=strict`, a syscall filter, and
  only `AF_INET`/`AF_INET6`. It holds no key material and reads no
  configuration; enforcing that beats relying on it staying true.
- Drains live sessions on stop rather than dropping them. A relay is the only
  route to a machine with no inbound port, so an outage stops every deploy
  against it.

## `openFirewall` defaults to false

The agent needs no inbound port at all; the relay is the single component in
this project that does, so opening one should be a decision an operator makes.

There is a worked example of why in this repo's own history.
`services.openssh.openFirewall` defaults to true and NixOS firewall port lists
**merge** rather than override — so a rung 0 target declaring
`allowedTCPPorts = [ ]` to admit nothing was reachable anyway, and the first run
proved it by connecting directly. A module that opens a port quietly is a module
that makes a closed machine open without anyone noticing.

## Evidence

Both VM tests now declare the relay through the module instead of a hand-written
unit, and both still pass — 13 subtests. A module only the tests never use would
be a module nobody has run.

Rung 0 also gained an assertion that the relay's port is open on the one machine
that asked for it, so the option's behaviour is checked rather than assumed.

## Downstream

Unblocks a relay on real infrastructure. `mipnix-ub5r` puts one on durer.
