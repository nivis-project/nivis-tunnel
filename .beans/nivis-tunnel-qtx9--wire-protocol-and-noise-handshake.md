---
# nivis-tunnel-qtx9
title: Wire protocol and Noise handshake
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T15:18:47Z
parent: nivis-tunnel-xvjv
openspec-link: openspec/changes/archive/2026-09-11-tunnel-wire-protocol
---

The protocol is the reason relay, agent and cli share one repository. Getting
its shape right early is worth more than any other decision here.

## Why

Two things must be true from the first release, because retrofitting either is
a fleet-wide image rebuild:

1. **Version negotiation.** Without it, every protocol change is a breaking
   change, and every breaking change rebuilds the boot image of every machine.
2. **End-to-end Noise.** The relay must never be able to read what it carries.
   That is what makes it safe to host, trivial to self-host, and honest to offer
   as a service.

## Summary of Changes

OpenSpec change `tunnel-wire-protocol`, capability `tunnel-wire-protocol`.

- **Rendezvous frame** in `internal/proto/frame.go`: fixed binary encoding of
  magic `NVTL`, uint16 version, role byte, id length and id. Positional rather
  than structured so the version can be read and checked before anything else
  is parsed — "your peer is too new" must be a legible error, not a parse
  failure at an arbitrary offset. The magic makes a connection to the wrong port
  fail immediately instead of hanging.
- **Version negotiation**: checked straight after the magic, before role or id.
  A refusal names both the version offered and the range supported. A test pins
  the ordering by sending a frame that is both too new and otherwise malformed.
- **Stream id validation** promoted into the protocol contract, and tightened:
  the refusal does not quote what it refused, because errors are what get
  logged, and echoing a hostile id is the injection the validation exists to
  prevent.
- **Noise handshake** in `internal/proto/handshake.go`:
  `Noise_XK_25519_ChaChaPoly_BLAKE2s`, with `ConnectAsAgent` /
  `ConnectAsOrchestrator` naming the parties by role rather than by Noise
  position. Returns a `net.Conn`, so callers splice bytes without knowing about
  Noise — which is what lets ssh run over it unchanged.
- **Tests** assert the claims rather than the implementation: that the relay
  cannot read the plaintext it carries (checked against what a stand-in relay
  actually observed), that tampering is detected, and that a payload spanning
  several Noise messages survives.
- `vendorHash` in `flake.nix` now pins the dependency tree, stated once and
  shared by the binaries and the test derivation. The `unit` check moved to
  `buildGoModule`: the Nix sandbox has no network, so a bare `go test` tried to
  fetch modules and failed.

## Design revision, and why it happened

`Noise_KX` with the orchestrator initiating was the original choice. It
authenticates just as strongly — an impostor learns nothing and can forge
nothing — but the responder gets no signal at handshake time. The agent would
complete its half believing it had a session and discover otherwise only when
transport failed to decrypt, leaving a small machine holding a connection that
could never carry a byte.

The spec says the agent "refuses it and the connection is closed without
carrying payload". `KX` could not honour that, and the test written from the
spec is what surfaced it. `Noise_XK` with the agent as initiator puts
verification on the side holding only a public key. Which party dials first is a
relay-layer concern — both dial outward — so the Noise roles were free.

The spec and design were revised to match; the implementation did not quietly
diverge from them.

## Follow-ups recorded elsewhere

- Target authentication is still absent by design: `nivis-tunnel-9t50`. The `X`
  half of XK already puts the agent's key on the wire, so verifying it is a
  configuration change rather than a protocol change.
- The orchestrator public key is baked into the image, so rotating it is an
  image rebuild: `nivis-tunnel-gi7b`.
