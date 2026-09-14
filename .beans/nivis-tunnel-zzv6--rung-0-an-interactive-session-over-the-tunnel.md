---
# nivis-tunnel-zzv6
title: 'Rung 0: an interactive session over the tunnel'
status: completed
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-14T20:53:00Z
parent: nivis-tunnel-xvjv
blocked_by:
    - nivis-tunnel-qtx9
    - nivis-tunnel-up7u
    - nivis-tunnel-1kpt
    - nivis-tunnel-ru8h
    - nivis-tunnel-mxtv
---

The acceptance test for milestone 02.

## Proven, hermetically and against a cloud

`nix flake check` runs a NixOS VM test with three machines in which the
target admits nothing, direct ssh to it fails, and a session opens through
the tunnel anyway. That was the transport. What remained was the environment:
a real NAT, a real public relay, real latency, a real cloud firewall rather
than nftables in a VM.

That has now been done.

## The cloud run

- **Relay**: durer (`nuremberg.pimsnel.com:7843`), via the mipnix module
  `networking-nivis-tunnel-relay`.
- **Target**: `i-01415faf5b5b44b99`, a t3.micro in eu-central-1 built from
  `040_tunnel_target` in nivis-demos. Security group with no ingress rules at
  all, host firewall with empty `allowedTCPPorts`, no instance profile, no DNS
  record.

The agent dialled out on its own after boot, from the relay log:

```
22:44:48  parked  stream=poc-target-aws-01 role=agent
22:48:27  paired  stream=poc-target-aws-01 role=orchestrator with=agent
22:48:27  session ended
22:48:27  parked  role=agent
```

## The negative, recorded first

Deliberately before the session, because a tunnel that works proves nothing if
you did not first establish that nothing else does.

```
ssh root@51.102.104.160          Connection timed out
TCP connect to 51.102.104.160:22 no connection
security group                   zero ingress rules

nmap -Pn -p- 51.102.104.160
  All 65535 scanned ports are in ignored states.
  Not shown: 65535 filtered tcp ports (no-response)
```

Every port, not a sample.

## Rung 0

```
$ ssh -o ProxyCommand="nivis-tunnel connect poc-target-aws-01 --relay nuremberg.pimsnel.com:7843 --key ..." root@poc-target-aws-01
poc-target-aws-01
 20:48:27  up   0:03,  0 users
active
```

## Rung 1, also on real infrastructure

16 MiB of random data, a path the machine could not already have:

- copied in 1s, sha256 identical on both ends
- a second copy sent nothing: `copying 0 paths...`
- a real closure with dependencies (`hello`, 5 paths, 2 missing) copied and
  then executed on the target: `Hello, world!`

## Summary of Changes

Nothing in this repo changed to make this work. The transport, the agent
module, the relay module and the CLI were all already proven in the VM tests;
this bean was open on an environmental claim, and the environment now agrees.

What it took was in nivis-demos and nivis, and is recorded there: a truncated
image upload that nothing was positioned to notice (nivis-demos-u4x9,
nixform2-sqco, nixform2-xqy1), an S3 key that did not carry the image so a
rebuilt image never reached the machine (nivis-demos-1wj0), and two
replacement defects in nivis (nixform2-nwnf, nixform2-aur6).
