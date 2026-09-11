# AGENTS.md — nivis-tunnel

## Overview

`nivis-tunnel` is a **public** proof of concept for a cloud-neutral deploy
transport: a replacement for AWS SSM Session Manager that works the same way on
Hetzner, on-prem and at the edge.

**The point of this project is not the tunnel.** It is the **image/live-config
split**. nivis today can only rebuild a whole machine image: every change to a
NixOS configuration produces a new image, a new snapshot, and a replaced
server. That single fact is the root of a class of problems — secrets baked
into an image cannot be rotated without replacing the machine, and any identity
the machine generates at first boot is destroyed by the next deploy.

The [elastinix](https://github.com/wearetechnative/terraform-aws-module-elastinix)
module already has the split: a thin bootstrap AMI, plus a live closure pushed
with `nix-copy-closure` and activated with `switch-to-configuration`. It works,
and it is AWS-only, because the transport is SSM. This project is that split
with a transport that runs anywhere.

```
target has no inbound port
        │
        ├─ agent dials out ─────┐
        │                       ├──► relay: splices two streams by id,
        ├─ orchestrator dials ──┘         sees only bytes it cannot read
        │
        └─ Noise runs end to end; ssh runs inside the stream
```

Because ssh runs inside the stream, `nix-copy-closure`, `switch-to-configuration`
and `deploy-rs` all work **unchanged**. The only integration point is ssh's
`ProxyCommand`. That seam is the design: everything above it is already proven
in production elsewhere, so this project only has to earn the layer below it.

The companion repo `terraform-provider-nivis-tunnel` holds the
`nixos_activation` resource. This repo holds the transport — relay, agent, cli,
NixOS module and the wire protocol — together, because they share that protocol
and splitting them would mean three lockstep releases per protocol change.

## Commands

```bash
# Beans (issue tracker) — source of truth for what to work on next
beans prime                  # read the workflow before touching beans
beans roadmap                # milestones -> epics -> tasks
beans list --json --ready    # what is ready to start
beans show --json <id>       # full bean
beans check                  # validate config and bean integrity

# OpenSpec — planning lives in the shared `nivis` store, NOT in ./openspec
openspec context             # show which root/store resolves here
openspec list                # active changes
openspec doctor              # store registration health
/opsx:propose "<idea>"       # new proposal (Claude Code)
/opsx:apply                  # implement a change
/opsx:archive                # archive a completed change

# Nix — the gate
nix flake check              # build + unit tests + formatting; must stay green
nix develop                  # dev shell: go, gopls, golangci-lint, jj, ssh
nix fmt                      # format Nix files
nix build .#relay .#agent .#tunnel

# Go
go test ./...
go test ./test/e2e -tags=e2e    # end-to-end, needs a target (see Testing)

# Version control: jj, colocated with git
jj st
jj describe -m "<subject>"
jj new                       # start the next change
jj git push
```

## OpenSpec lives in a store

This repo declares `store: nivis` in `openspec/config.yaml`, so there is **no
local `openspec/specs` or `openspec/changes`** — proposals and specs live in
`/home/pim/gh.nivis-project/ospecs` (`git@github.com:nivis-project/ospecs.git`),
shared with the other nivis repos. Two consequences:

- `openspec archive` writes into the **store's** repo, not this one.
- Never re-create `openspec/specs` or `openspec/changes` here. `openspec init`
  creates them by default; if `openspec doctor` starts warning that the store
  declaration is ignored, it is because one of those directories came back —
  delete it.

## Beans

Beans is the internal ticket system, and **Claude Code administers it**. The
hierarchy is milestone → epic → task.

- **Milestone titles start with an incrementing two-digit number**, beginning at
  `01`. Example: `01 Foundation`, `02 Tunnel transport`.
- **Epics are the unit that becomes an OpenSpec proposal.** One epic, one
  proposal.
- Tasks inside an epic are tracked in the OpenSpec change's `tasks.md`, not as
  separate beans. Beans carry the *why* and the acceptance criteria; OpenSpec
  carries the work breakdown.

Status transitions Claude Code owns:

| When | Action |
|--------------------------------------|----------------------------------------|
| starting work on an epic             | bean status → `in-progress`            |
| creating the proposal                | link the bean id in `proposal.md`      |
| archiving the change                 | add `openspec-link:` to the frontmatter, status → `completed` |
| deciding not to do it                | status → `scrapped` + `## Reasons for Scrapping` |

Beans that belong to **another repository** live here too, but their body must
open with `**Target repo:** <name>` and carry enough context — file paths, line
numbers, the finding itself — to be acted on or moved without re-deriving it.

## Version control: jj

`jj` (jujutsu) is colocated with git in this repo, so both work, but use `jj`.

- **Commit after every OpenSpec change archival.** One archived change, one
  commit.
- Commits are authored by **Pim Snel** alone. Never add `Co-authored-by`,
  `Generated with`, or any other attribution trailer.
- Commit messages: a short imperative subject, then a body explaining *why* the
  change was needed and what evidence proves it works. Match the style already
  in the nivis repos.
- Nix reads the **git index**, not the jj working copy. If `nix flake check`
  reports a file "is not tracked by Git", run `git add -A` before re-running.

## Nix conventions

- **Do not use `flake-utils`.** Supported systems are enumerated in plain Nix
  and mapped with `nixpkgs.lib.genAttrs`. One `genAttrs` is the whole
  abstraction flake-utils provides, and keeping it explicit means a reader sees
  exactly which systems are built.
- `nix flake check` is the gate. It must be green before any change is
  archived. It builds all three binaries, runs the Go unit tests, and checks
  Nix formatting.
- `vendorHash = null` holds only while `go.mod` has no external dependencies.
  The moment one is added, `nix build` prints the expected hash — put it in.

## Testing

Thorough testing is a requirement of this PoC, not a nicety. A proof of concept
that cannot demonstrate its own claim has proven nothing.

**Unit tests** live beside the code and run in `nix flake check`. Every task
that changes behaviour names the test that proves it.

**End-to-end tests** live in `test/e2e`, behind the `e2e` build tag so they
never run in the sandboxed Nix gate (they need a network and a target). Each one
states an *observable outcome*, not an implementation detail.

The e2e suite is organised around the test ladder, and each rung is a real
acceptance test:

```
rung 0   ssh -o ProxyCommand='nivis-tunnel connect %h' root@<id> uptime
         → the tunnel carries an interactive session

rung 1   NIX_SSHOPTS="-F poc.conf" nix-copy-closure root@<id> <path>
         → the tunnel carries a large closure without stalling
           (the one property nobody has measured; SSM does it fine,
            but that is AWS's relay, not ours)

rung 3   nivis apply → bootstrap image → server → activation
         then change liveSystem and apply again
         → ONLY the activation resource changes.
           No new snapshot. No replaced server.
           THIS is the claim the PoC exists to prove.
```

Rung 2 is optional and free: point elastinix's `instance/ssh.conf` at our
`ProxyCommand` instead of SSM and run its existing scripts unchanged. That is a
true A/B — same scripts, same machine, same closure, only the transport differs
— so any failure is attributable to the tunnel and nothing else.

## Scope

The PoC builds exactly this:

1. **transport** — relay (splices on stream id, no crypto of its own), agent
   (dials out, Noise responder, splices to local ssh), cli (`nivis-tunnel
   connect <id>`, usable as a `ProxyCommand`), NixOS module for the agent.
2. **provider** — `nixos_activation` with Create/Read/Update, in the companion
   repo.
3. **demo domain** — in this repo, disposable, demonstrating the split.

The trust model is deliberately thin: the orchestrator's public key is baked
into the image and the agent accepts only that. The target proves nothing; it is
identified by the address the cloud API returned. This is not good enough for
production and is exactly good enough to prove the point.

**Everything else is a bean, not a task.** The backlog already records magic
rollback, a CA-style trust root, target attestation, NAT traversal, relay
multi-tenancy and the rest. Resist folding them in: the PoC's value is that it
proves one layer, quickly, with the layers above it untouched.

## Boot image discipline

The agent goes into the boot image, which gives it the strictest change budget
in the system. On NixOS the kernel, initrd and bootloader all live in the
closure, so `switch-to-configuration` can replace them; the only things that
genuinely force an image rebuild are partition layout, filesystem, boot mode,
and this agent.

So: **the agent in the image only has to be good enough to accept one push.**
A newer agent arrives through the live configuration like any other package.
Keep `nix/module.nix` dull, and make protocol-breaking changes expensive to
need — which is why `proto.Version` exists from the first commit.
