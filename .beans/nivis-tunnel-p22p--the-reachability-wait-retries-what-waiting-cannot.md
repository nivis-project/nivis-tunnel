---
# nivis-tunnel-p22p
title: The reachability wait retries what waiting cannot fix
status: todo
type: bug
created_at: 2026-09-14T22:15:37Z
updated_at: 2026-09-14T22:15:37Z
---

**Target repo: terraform-provider-nivis-tunnel** (`internal/provider/target.go`,
`waitReachable`, around line 147).

`waitReachable` retries `ssh ... true` every 5 seconds for 3 minutes and treats
every failure identically. The reasoning above it is sound:

> A machine created moments ago is not yet reachable. A provider that gave up
> on the first refused connection would be unusable against a freshly created
> server, the only kind this project creates.

What the loop cannot do is tell a condition that time will fix from one that
time will never fix.

## Observed twice in one evening

Applying `040_tunnel_target` in nivis-demos:

```
Activation failed: target poc-target-aws-01 did not become reachable within 3m0s
fish: Unknown command: connect
```

```
Activation failed: target poc-target-aws-01 did not become reachable within 3m0s
fish: Unknown command: /nix/store/d8vj6.../bin/nivis-tunnel
```

The first was an empty `tunnel_command`; the second a path that had never been
built. Neither could ever have succeeded, and each got 36 attempts.

The diagnosis was present in the very first attempt. It surfaced three minutes
late, wrapped in a timeout message, which reads like a network problem when it
is a configuration error.

## The distinction to draw

Operator-side facts are already true or false when the operation starts.
Waiting changes none of them:

```
tunnel_command empty              -> fail now
tunnel_command does not exist     -> fail now
tunnel_command not executable     -> fail now
key_file missing or unreadable    -> fail now

agent not parked yet              -> wait
connection refused                -> wait
machine still booting             -> wait
```

A preflight, once, before the loop. Retry only what time can repair.

## Not the alternative

Sniffing ssh stderr for "Unknown command" or "No such file" would catch these
two and nothing else, and would break on a different shell or locale. The
inputs are checkable directly, which is both cheaper and exact.

## Related

The empty value came from nivis (nixform2-1mk0: schema defaults never reach a
provider). A preflight would have named it in one second instead of hiding it
behind a timeout, which is the point: this bean is about how the failure reads,
not about whose fault the value was.

## Todo

- [ ] Preflight the operator-side inputs before entering waitReachable
- [ ] Each failure names the attribute and what was wrong with it
- [ ] Test: an unset tunnel_command fails immediately, not after the timeout
- [ ] Test: a tunnel_command naming a nonexistent path fails immediately
- [ ] Test: a refused connection still retries to the deadline
