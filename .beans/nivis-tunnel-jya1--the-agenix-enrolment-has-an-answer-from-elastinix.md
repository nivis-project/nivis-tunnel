---
# nivis-tunnel-jya1
title: The agenix enrolment has an answer from elastinix
status: todo
type: feature
priority: high
created_at: 2026-09-11T14:58:36Z
updated_at: 2026-09-11T14:58:36Z
parent: nivis-tunnel-khrj
---

**Target repo:** `nivis-demos` (`/home/pim/gh.nivis-project/nivis-demos`)
**Existing bean there:** `nivis-demos-x98i`

`x98i` records that the Hetzner agenix enrolment cannot converge, and lists three
candidate fixes. Elastinix has a fourth, already in production, and it is better
than all three.

## The answer

`instance/script/local_exec_upload_ssh_workloads_key.sh`:

```bash
SYS_SSH_KEY=$(agenix -d system_sshd_key.age --identity "${KEYFILE}")
echo "${SYS_SSH_KEY}" | ssh … ${TARGET} 'cat - > /tmp/system_sshd_key && chmod 600 …'
```

The host key is **not generated on the machine — it is pushed into it**,
decrypted on the orchestrator against the operator's identity. The secret is
encrypted to the *operator*, never to the box.

That removes the cycle entirely: there is no identity that comes into existence
only after boot, so nothing has to be re-keyed to it, so no server replacement
is needed, so nothing is invalidated.

## Corroboration

`instance/script/local_exec_test_machine_up.sh` carries the abandoned route,
commented out:

```bash
#echo "Add the the systems private key to agenix and run rekey (agenix -r -i PRIVATE_KEY)"
#cat /etc/ssh/ssh_host_ed25519_key.pub
```

They started where nivis-demos still stands, and walked away from it.

## Action

Update `x98i` with this, and prefer it over the volume-persistence and
`user_data` options recorded there.
