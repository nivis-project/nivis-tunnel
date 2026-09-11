---
# nivis-tunnel-87qr
title: Derive push/pull from egress instead of configuring it
status: todo
type: feature
priority: low
created_at: 2026-09-11T14:57:57Z
updated_at: 2026-09-11T14:57:57Z
parent: nivis-tunnel-q5cc
---

`fastConnection` (push everything) versus letting the target substitute is
presented as a free choice. It is not:

```
private subnet, no egress ⇒ no substituter reachable ⇒ push is mandatory
```

The very property that makes this transport valuable — it works in closed
networks — is what forces push mode there. The provider can derive the value
from the target's reachability rather than asking an operator to know it.
