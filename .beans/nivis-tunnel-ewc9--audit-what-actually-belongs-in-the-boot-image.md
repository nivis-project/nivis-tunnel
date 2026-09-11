---
# nivis-tunnel-ewc9
title: Audit what actually belongs in the boot image
status: todo
type: feature
priority: normal
created_at: 2026-09-11T14:58:36Z
updated_at: 2026-09-11T14:58:36Z
parent: nivis-tunnel-khrj
---

**Target repo:** `terraform-aws-module-elastinix`
(`/home/pim/gh.wearetechnative/terraform-aws-module-elastinix`)

The AMI layer is rebuilt occasionally, and each rebuild means reconstructing the
whole machine. The list of things that genuinely force that is short.

## The insight

On NixOS the kernel, initrd **and bootloader** all live in the closure —
`switch-to-configuration` runs the bootloader installer when `boot.loader`
changes. So a kernel upgrade is a push plus a reboot, not a new image.

What truly forces an image rebuild:

```
partition layout          cannot be switched
filesystem properties     cannot be switched
boot mode (BIOS/UEFI)     cannot be switched
the bootstrap agent       only on a protocol break
```

Everything else — services, packages, network configuration, kernels — can move
down into the live configuration.

## Action

Audit the AMI layer against that list. If rebuilds happen more often than it
justifies, something is in the image that need not be. An afternoon's work, not
a project.
