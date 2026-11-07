---
name: sl-workflows
description: Use the public sl CLI to create, deploy, trigger, and monitor Saleslumen Workflows.
---

# Workflows with `sl`

Manage workflows, immutable revisions, triggers, executions, and step runs with `sl api workflows`. Read the linked API reference for request paths and JSON fields. Use `sl workflows --help` to see which typed commands the CLI provides.

Creating executions, signaling executions, and creating triggers require user OAuth. After creating an execution, read its state and step runs before reporting completion.

```sh
sl api workflows /v1/workflows --json
```

Work within the user's authorized scope; ask when the target or effect is unclear. `sl` does not retry requests. After an ambiguous response, inspect resource state before repeating a write.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Workflows API overview](https://developers.saleslumen.com/en-US/workflows/guides/overview.md.txt)
- [Design a definition](https://developers.saleslumen.com/en-US/workflows/guides/design-definitions.md.txt)
- [Create and monitor executions](https://developers.saleslumen.com/en-US/workflows/guides/create-and-monitor.md.txt)
- [Cancel and signal](https://developers.saleslumen.com/en-US/workflows/guides/cancel-and-signal.md.txt)
- [Workflows API reference](https://developers.saleslumen.com/en-US/workflows/reference/apis/workflows.md.txt)
- [Executions API reference](https://developers.saleslumen.com/en-US/workflows/reference/apis/executions.md.txt)
- [Triggers API reference](https://developers.saleslumen.com/en-US/workflows/reference/apis/triggers.md.txt)
