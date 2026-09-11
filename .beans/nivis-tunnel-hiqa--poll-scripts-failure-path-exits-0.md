---
# nivis-tunnel-hiqa
title: Poll script's failure path exits 0
status: todo
type: bug
priority: high
created_at: 2026-09-11T14:58:36Z
updated_at: 2026-09-11T14:58:36Z
parent: nivis-tunnel-khrj
---

**Target repo:** `terraform-aws-module-elastinix`
(`/home/pim/gh.wearetechnative/terraform-aws-module-elastinix`)
**File:** `instance/script/local_exec_test_machine_up.sh`

```bash
cleanup() { exit $!; }
```

`$!` is the PID of the most recent **background** job, not an exit code. No
background job runs in this script, so `$!` is empty and `exit` falls back to
the status of the last command executed.

In the failure path that last command is:

```bash
echo "Failed to poll for machine up status"
cleanup 1
```

`echo` succeeds, so **the failure path exits 0**. Terraform sees a successful
provisioner and proceeds to deploy to a machine that never came up. Almost
certainly `$1` was intended.

## Why it matters to this project

The same polling pattern is needed here (the agent is not reachable the instant
a server is created, which is exactly why elastinix polls). Worth not copying
the bug along with the pattern.
