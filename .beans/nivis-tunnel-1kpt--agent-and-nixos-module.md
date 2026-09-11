---
# nivis-tunnel-1kpt
title: Agent and NixOS module
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:03Z
updated_at: 2026-09-11T15:57:49Z
parent: nivis-tunnel-xvjv
openspec-link: openspec/changes/archive/2026-09-11-tunnel-agent
blocked_by:
    - nivis-tunnel-qtx9
---

The agent runs on the target: it dials out to the relay, waits, completes the
Noise handshake as the verifying party, and splices the stream onto local ssh.

## Why

This is the piece that goes into the boot image, which gives it the strictest
change budget in the system. **The agent in the image only has to be good enough
to accept one push.** A newer agent arrives through the live configuration like
any other package.

## Summary of Changes

OpenSpec change `tunnel-agent`, capability `tunnel-agent`.

- `internal/agent` dials out, announces, handshakes, splices, and returns to
  waiting. One session at a time, which is what a deploy needs and what keeps
  the state small enough to reason about in a boot image.
- Reconnect with exponential backoff capped at 30s. The cap is the interesting
  number: a relay coming back must be found promptly, and a machine that waits
  ten minutes to notice is a machine nobody can deploy to.
- Refusing a peer is not a fault to back off from. A machine that could be taken
  out of reach by anyone who connects to it would be worse than one with an open
  port.
- The agent generates its own keypair per run. The orchestrator does not verify
  it, so there is nothing to persist and nothing for an operator to manage.
- `cmd/agent` with flags matching the module's options; `nix/module.nix`
  finished and actually exercised.

## Evidence

Nine unit tests green under `-race` against a real relay and a real client, plus
**a NixOS VM test with three machines** — a relay, a target, and an operator —
in which all seven subtests pass:

- the agent holds no listening socket (checked against the service's PID)
- its unit carries the orchestrator PUBLIC key and not the private one
- **the target is NOT reachable by direct ssh** — the load-bearing negative
- **rung 0: `ssh -o ProxyCommand='tunnel connect ...'` runs a command on it**
- it is reachable again on a second session, with nothing restarted
- 32 MiB produced on the target arrives byte-identical at the operator
- an impostor key cannot open a session, and the agent survives the attempt

## What this does and does not prove

The claim — a host whose firewall admits nothing is still reachable — is proven
hermetically, in VMs, and the negative assertion makes it a real proof rather
than a machine that was open all along. What it does not prove is the same
against a **real cloud target**, which needs an account and credentials. That
remains the open half of `nivis-tunnel-zzv6`.

## Found while verifying

The first VM run reached the target directly: `services.openssh.openFirewall`
defaults to true, and firewall port lists MERGE rather than override, so an
empty `allowedTCPPorts` closed nothing. Every later subtest would have passed
while proving nothing. Also fixed: Go's `flag` swallowed the flags in
`connect <id> --relay ...`, the documented ProxyCommand form; and two VM
assertions that were written to pass rather than to test.
