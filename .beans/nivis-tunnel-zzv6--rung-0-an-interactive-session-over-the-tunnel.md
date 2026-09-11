---
# nivis-tunnel-zzv6
title: 'Rung 0: an interactive session over the tunnel'
status: in-progress
type: epic
priority: normal
created_at: 2026-09-11T14:56:38Z
updated_at: 2026-09-11T15:58:15Z
parent: nivis-tunnel-xvjv
blocked_by:
    - nivis-tunnel-qtx9
    - nivis-tunnel-up7u
    - nivis-tunnel-1kpt
    - nivis-tunnel-ru8h
    - nivis-tunnel-mxtv
---

The acceptance test for milestone 02.

## Status: proven hermetically, not yet against a cloud

**The claim is proven.** `nix flake check` runs a NixOS VM test with three
machines — a relay, a target, and an operator — in which:

- the target's firewall admits nothing, and **direct ssh to it fails**
- `ssh -o ProxyCommand='tunnel connect <id> --relay ... --key ...'` opens a
  session and runs a command on it
- the machine is reachable again on a second session, with nothing restarted
- 32 MiB produced on the target arrives byte-identical at the operator
- an impostor key cannot open a session, and the agent survives the attempt

The negative — that direct ssh fails — is what makes this a proof rather than a
demonstration. Its first run caught exactly the mistake it exists to catch: the
target was reachable all along, because `services.openssh.openFirewall` defaults
to true and firewall port lists merge rather than override.

## What remains

The original acceptance names **a host on Hetzner or AWS**. That has not been
done, and cannot be done from here: it needs a cloud account, credentials and a
relay reachable from it.

Everything the cloud run would exercise beyond the VM test is environmental — a
real NAT, a real public relay, real latency, a real cloud firewall rather than
nftables in a VM. The transport itself is proven.

## To close this bean

1. A relay on a reachable address.
2. A Hetzner or AWS host with the agent enabled, its cloud firewall admitting
   nothing, and `services.nivis-tunnel-agent.streamId` set to the instance id.
3. `ssh -o ProxyCommand='nivis-tunnel connect <instance-id> ...' root@<instance-id> uptime`
4. A port scan of the host's public address showing nothing open. Record both;
   the negative is half the claim.
