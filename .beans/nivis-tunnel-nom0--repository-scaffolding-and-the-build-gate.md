---
# nivis-tunnel-nom0
title: Repository scaffolding and the build gate
status: completed
type: epic
created_at: 2026-09-11T14:56:02Z
updated_at: 2026-09-11T14:56:02Z
parent: nivis-tunnel-hilr
---

Both repos scaffolded: Nix flake with systems enumerated in plain Nix (no
flake-utils), Go module, placeholder binaries, OpenSpec pointed at the `nivis`
store, beans initialised, jj colocated with git.

## Summary of Changes

- `flake.nix` in both repos: `systems` list + `nixpkgs.lib.genAttrs`, dev shell,
  packages, and a `checks` set that builds every binary, runs `go test ./...`
  and verifies Nix formatting.
- `internal/proto` with `StreamID.Validate` and `Version`, plus a table test
  covering path traversal, shell metacharacters, newline injection and length.
- `nix/module.nix`: NixOS module for the agent, outbound-only, `DynamicUser`.
- `openspec/config.yaml` declares `store: nivis`; the local `specs/` and
  `changes/` directories that `openspec init` creates were removed.
- `CLAUDE.md` in both repos records the working agreement.

`nix flake check` passes in both repos.

## Note

Nix reads the git index, not the jj working copy. A file added but not staged
fails the gate with "is not tracked by Git"; `git add -A` first.
