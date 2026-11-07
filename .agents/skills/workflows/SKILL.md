---
name: sl-workflows
description: Use the Saleslumen CLI to inspect workflows, revisions, executions, step runs, triggers, and event types; create or update workflows and revisions; and deploy, undeploy, signal, cancel, or manage triggers.
---

# Workflows

Use `sl workflows` for the typed Workflows commands. Creating executions, signaling executions, and creating triggers require user OAuth; sign in with `sl auth login`. Other typed commands accept the credentials allowed by the API route.

## Find and inspect

Search matches workflow display names. Search returns at most 20 results in one request and does not paginate. Without `--query`, `--limit 0` follows every list page.

```sh
sl workflows list --query "lead follow-up" --limit 20 --json
sl workflows get WORKFLOW_ID --json
sl workflows revisions list --workflow WORKFLOW_ID --json
sl workflows executions list --workflow WORKFLOW_ID --filter 'state = "STATE_FAILED"' --json
sl workflows executions get EXECUTION_ID --workflow WORKFLOW_ID --json
sl workflows step-runs list --workflow WORKFLOW_ID --execution EXECUTION_ID --json
sl workflows triggers list --workflow WORKFLOW_ID --json
sl workflows event-types list --json
```

Use the returned resource names and current `etag` values in later requests. Workflow and execution identifiers are the IDs used in their resource names.

## Create and deploy

Pass JSON object bodies with `--input FILE` or `--input -`. Caller-selected IDs are optional. Updating a resource uses its current `etag`; update masks are optional and name the fields being changed.

```sh
sl workflows create --workflow-id lead-follow-up --display-name "Lead follow-up" --json
sl workflows revisions create --workflow WORKFLOW_ID --revision-id first --input revision.json --json
sl workflows deploy WORKFLOW_ID --revision workflows/WORKFLOW_ID/revisions/first --json
sl workflows undeploy WORKFLOW_ID --json
```

Create a revision from the current workflow definition contract. Read the workflow before editing so the submitted `etag` is current. Use `sl workflows update WORKFLOW_ID --input workflow.json` for updates and `sl workflows delete WORKFLOW_ID` for deletion; deletion prompts for confirmation unless `--yes` is supplied.

## Run and inspect executions

Execution creation requires user OAuth. Omit `revision` to use the workflow's serving revision, or pass a valid revision resource name. `--request-id` is an optional idempotency key. The CLI does not retry writes; after an ambiguous response, inspect the resource before repeating the request.

```sh
sl workflows executions create --workflow WORKFLOW_ID --execution-id run-2026-01 --input execution.json --json
sl workflows executions get EXECUTION_ID --workflow WORKFLOW_ID --json
sl workflows step-runs list --workflow WORKFLOW_ID --execution EXECUTION_ID --limit 100 --json
sl workflows step-runs get SEQUENCE --workflow WORKFLOW_ID --execution EXECUTION_ID --json
sl workflows executions cancel EXECUTION_ID --workflow WORKFLOW_ID --json
```

Signaling requires user OAuth. Put the signal payload in the JSON body; `--request-id`, when supplied, is merged into that body.

```sh
sl workflows executions signal EXECUTION_ID --workflow WORKFLOW_ID --request-id REQUEST_UUID --input signal.json --json
```

## Manage triggers

Trigger creation requires user OAuth. Use the trigger's event type and body fields from the linked API contract. `--show-deleted` includes tombstoned triggers in list results.

```sh
sl workflows triggers create --workflow WORKFLOW_ID --trigger-id inbound --input trigger.json --json
sl workflows triggers list --workflow WORKFLOW_ID --show-deleted --json
sl workflows triggers update TRIGGER_ID --workflow WORKFLOW_ID --input trigger-update.json --update-mask enabled --json
sl workflows triggers rotate-webhook-token TRIGGER_ID --workflow WORKFLOW_ID --json
sl workflows triggers delete TRIGGER_ID --workflow WORKFLOW_ID
```

For the update example, `trigger-update.json` includes the current `etag` and `enabled` value. Valid update-mask fields depend on trigger kind: webhook supports `enabled`, schedule supports `enabled`, `cron`, and `timeZone`, and event supports `enabled` and `eventType`.

Anonymous webhook invocation is documented in the Triggers API reference; it is outside the typed resource command surface.

Read these references for resource fields, definition and execution semantics, trigger schemas, and the complete HTTP contract:

- [Workflows API overview](https://developers.saleslumen.com/en-US/workflows/guides/overview.md.txt)
- [Getting started](https://developers.saleslumen.com/en-US/workflows/guides/getting-started.md.txt)
- [Design a definition](https://developers.saleslumen.com/en-US/workflows/guides/design-definitions.md.txt)
- [Create and monitor executions](https://developers.saleslumen.com/en-US/workflows/guides/create-and-monitor.md.txt)
- [Cancel and signal](https://developers.saleslumen.com/en-US/workflows/guides/cancel-and-signal.md.txt)
- [Workflows API reference](https://developers.saleslumen.com/en-US/workflows/reference/apis/workflows.md.txt)
- [Executions API reference](https://developers.saleslumen.com/en-US/workflows/reference/apis/executions.md.txt)
- [Triggers API reference](https://developers.saleslumen.com/en-US/workflows/reference/apis/triggers.md.txt)
- [SL CLI reference](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
