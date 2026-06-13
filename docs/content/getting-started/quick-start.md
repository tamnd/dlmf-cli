---
title: "Quick start"
description: "Run your first dlmf command."
weight: 30
---

Once `dlmf` is on your `PATH`:

```bash
dlmf --help       # see the command tree
dlmf version      # build info
```

This is a fresh scaffold, so the command tree is just `version` for now. Add
your first real command in `cli/`, build on the `dlmf` library package,
and document it here.

A good first command usually fetches one thing and prints it as JSON, so the
output pipes straight into `jq` and the rest of your tools.
