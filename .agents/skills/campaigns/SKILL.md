---
name: sl-campaigns
description: Use the public sl CLI for Saleslumen Campaigns, including people, rules, sequences, deliveries, schedules, and metrics.
---

# Campaigns with `sl`

Use the public `sl` CLI to inspect and manage Campaigns. Discover the exact command and flags from `sl campaigns --help` and the relevant child help; the API may have operations that are not exposed as typed commands.

The typed CLI covers campaigns, people, rules, sequences, schedules, deliveries, suppressions, tasks, operations, and metrics. Prefer typed commands. Use `sl api campaigns <path>` only when the needed public endpoint has no typed command, and follow the API reference for its request and response shape. `sl campaigns people run-script` requires user credentials when the request runs an installed app; its help describes this case.

Work within the user's authorized scope; ask when the target or effect is unclear. `sl` does not retry requests. After an ambiguous error, check the resulting resource before repeating the operation. Use `--json` or `--jq` for structured output.

## Examples

Inspect campaigns and their people:

```sh
sl campaigns list --limit 20 --json
sl campaigns list --query "spring follow-up" --visibility ALL --json
sl campaigns get CAMPAIGN_ID --json
sl campaigns people list --campaign CAMPAIGN_ID --limit 50 --json
sl campaigns people count --campaign CAMPAIGN_ID --json
sl campaigns sequences list --campaign CAMPAIGN_ID --limit 50 --json
sl campaigns rules list --campaign CAMPAIGN_ID --limit 50 --json
```

Create or change resources with the typed flags and request body shown by help:

```sh
sl campaigns create --display-name "Spring follow-up" --json
sl campaigns people import --campaign CAMPAIGN_ID --input people.json --json
sl campaigns sequences update SEQUENCE_ID --campaign CAMPAIGN_ID --input sequence.json --json
sl campaigns rules create --campaign CAMPAIGN_ID --input rule.json --json
```

Use lifecycle commands only when the user authorized the transition:

```sh
sl campaigns activate CAMPAIGN_ID --json
sl campaigns pause CAMPAIGN_ID --json
sl campaigns people stop PERSON_ID --campaign CAMPAIGN_ID --json
```

Inspect delivery state and performance for a campaign:

```sh
sl campaigns deliveries list --campaign CAMPAIGN_ID --status FAILED --limit 20 --json
sl campaigns metrics get --campaign CAMPAIGN_ID --time-range-start 2026-01-01T00:00:00Z --time-range-end 2026-02-01T00:00:00Z --reporting-timezone UTC --json
```

Delivery resolution changes delivery state. Use these commands only when the user authorized the specific resolution, and provide a request body valid for that resolution method:

```sh
sl campaigns deliveries resolve-acceptance DELIVERY_ID --input resolution.json --json
sl campaigns deliveries resolve-reconciliation DELIVERY_ID --input reconciliation.json --json
```

Use `--input FILE` for full request bodies; read the linked guides or child `--help` before preparing one. `--request-id` is generated where the CLI help says it is optional.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Campaigns getting started](https://developers.saleslumen.com/en-US/campaigns/guides/getting-started.md.txt)
- [Campaigns API reference](https://developers.saleslumen.com/en-US/campaigns/reference/apis/campaigns.md.txt)
- [People and suppression](https://developers.saleslumen.com/en-US/campaigns/guides/people-suppression.md.txt)
- [Deliveries and operations](https://developers.saleslumen.com/en-US/campaigns/guides/deliveries-operations.md.txt)
