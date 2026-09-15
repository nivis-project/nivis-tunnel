---
# nivis-tunnel-khjz
title: An activation says nothing until it fails
status: todo
type: feature
created_at: 2026-09-14T22:18:44Z
updated_at: 2026-09-14T22:18:44Z
---

**Target repo: terraform-provider-nivis-tunnel**, and half of it is nivis.

A successful activation prints nothing. The first real one took 51 seconds of
silence, and that was a small push: 33 paths, 17 MiB. A first deploy to a fresh
machine, or one carrying a language runtime, is minutes. The operator has no
way to tell a slow copy from a hang.

The output is not lost. `target.run` captures stdout and stderr and a failure
carries them, which is why the broken ProxyCommand was diagnosable at all. It
is simply never shown when things go well.

## What would be worth seeing

```
copying 33 paths (17 MiB) ...
setting profile /nix/var/nix/profiles/system -> generation 2
switch-to-configuration switch
  starting nginx.service
```

Those are three distinct steps with three distinct failure modes, and today
they are one opaque 51 seconds.

## The awkward part: there is no channel

`ApplyResourceChange` is a single unary RPC in tfplugin6. A provider cannot
report progress while it runs; that is why OpenTofu shows only "Still
creating... [51s elapsed]" from its own clock. The same constraint blocks a
progress bar for the S3 image upload in nivis-demos, so this is a property of
the protocol rather than of this provider.

What is possible:

1. **After the fact, as an attribute.** A computed attribute carrying the last
   activation output. Visible in state and in outputs, readable after the
   apply. Cheap, and it makes the copy size and the generation number
   inspectable rather than folklore.
2. **Diagnostics.** The provider can attach non-error diagnostics to its
   response. This costs nothing and is the natural place for "copied 33 paths,
   17 MiB".
3. **tflog.** Only useful if the caller surfaces it.

## Why 2 and 3 need nivis too

Nivis drops them. `normDiags` maps every non-ERROR severity to
`SeverityWarning` and nothing ever reads it; `DiagError` filters on
`SeverityError`. That is bean nixform2-8btx in the nivis repo, filed before
this project started, and it is the reason a provider that starts talking would
still not be heard.

So this is two beans wide: the provider has to say something, and nivis has to
show it. Doing only the first produces no visible change, which is worth
knowing before someone spends an afternoon on it.

## Todo

- [ ] Decide the shape: computed attribute, diagnostics, or both
- [ ] Report what was copied (paths and bytes) and which generation was set
- [ ] Report the three steps separately, since they fail separately
- [ ] Coordinate with nixform2-8btx, without which diagnostics stay invisible
- [ ] Test: a successful activation carries a summary; a failed one still
      carries the target output it carries today
