# nivis-tunnel

A cloud-neutral deploy transport for NixOS closures — what AWS SSM Session
Manager does, on any cloud.

> **The point is not the tunnel.** It is the image/live-config split.

## The problem

nivis can only rebuild a whole machine image. Every change to a NixOS
configuration produces a new image, a new snapshot, and a replaced server. That
one fact causes a class of problems:

- a secret baked into an image cannot be rotated without replacing the machine
- an identity the machine generates at first boot is destroyed by the next
  deploy, so anything encrypted to it becomes undecryptable
- a one-line configuration change costs a full image build, upload and reboot

The [elastinix](https://github.com/wearetechnative/terraform-aws-module-elastinix)
module already solves this with a thin bootstrap AMI plus a pushed live closure.
It works — and it is AWS-only, because the transport is SSM.

## The approach

```
target has no inbound port
        │
        ├─ agent dials out ─────┐
        │                       ├──► relay: splices two streams by id.
        ├─ orchestrator dials ──┘         Noise runs end to end, so the relay
        │                                 only ever sees bytes it cannot read.
        └─ ssh runs inside that stream
```

Because ssh runs inside the stream, `nix-copy-closure`, `switch-to-configuration`
and `deploy-rs` work **unchanged**. The only integration point is ssh's
`ProxyCommand`:

```sh
ssh -o ProxyCommand='nivis-tunnel connect %h' root@i-0abc123def456789
```

Two properties fall out. The relay needs no trust — it cannot read what it
carries, so it is safe to host and trivial to self-host. And both ends dial out,
so this works on a host with no public address at all: private subnets, on-prem,
edge.

## Layout

| Path | |
|--------------------|--------------------------------------------------|
| `cmd/relay`        | the rendezvous relay |
| `cmd/agent`        | runs on the target; dials out, waits, splices to local ssh |
| `cmd/tunnel`       | `nivis-tunnel connect <id>` — usable as an ssh `ProxyCommand` |
| `internal/proto`   | the wire protocol — why these live in one repo |
| `nix/module.nix`   | NixOS module for the agent |
| `test/e2e`         | end-to-end tests, `e2e` build tag |

The `nixos_activation` resource lives in
[`terraform-provider-nivis-tunnel`](https://github.com/nivis-project/terraform-provider-nivis-tunnel).
Planning for both repos lives here: see `beans roadmap`.

## Status

Proof of concept. The trust model is deliberately thin — the orchestrator's
public key is baked into the image and the agent accepts only that; the target
proves nothing and is identified by the address the cloud API returned. Good
enough to prove the point, not good enough for production. The backlog records
what production would need.

## Getting started

```sh
nix develop          # go, gopls, golangci-lint, jj, ssh
nix flake check      # the gate: builds, unit tests, formatting
beans roadmap        # what to work on
```

## Licence

See [LICENSE](LICENSE).
