---
name: sl-campaigns
description: Use the public sl CLI for Saleslumen Campaigns, including people, sequences, deliveries, schedules, and metrics.
---

# Campaigns with `sl`

Use the public `sl` CLI to inspect and manage Campaigns. Discover the exact command and flags from `sl campaigns --help` and the relevant child help; the API may have operations that are not exposed as typed commands.

The typed CLI covers campaigns, people, sequences, schedules, deliveries, suppressions, tasks, operations, and metrics. Prefer typed commands. Use `sl api campaigns <path>` only when the needed public endpoint has no typed command, and follow the API reference for its request and response shape. `sl campaigns people run-script` requires user credentials when the request runs an installed app; its help describes this case.

Work within the user's authorized scope; ask when the target or effect is unclear. `sl` does not retry requests. After an ambiguous error, check the resulting resource before repeating the operation. Use `--json` or `--jq` for structured output.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Campaigns getting started](https://developers.saleslumen.com/en-US/campaigns/guides/getting-started.md.txt)
- [Campaigns API reference](https://developers.saleslumen.com/en-US/campaigns/reference/apis/campaigns.md.txt)
- [People and suppression](https://developers.saleslumen.com/en-US/campaigns/guides/people-suppression.md.txt)
- [Deliveries and operations](https://developers.saleslumen.com/en-US/campaigns/guides/deliveries-operations.md.txt)
