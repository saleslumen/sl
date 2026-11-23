---
name: sl-workflows
description: Use the Saleslumen CLI to inspect workflows, revisions, executions, step runs, triggers, and event types; create or update workflows and revisions; and deploy, undeploy, signal, cancel, or manage triggers.
---

# Workflows

Use `sl workflows` for the typed Workflows commands. Creating a trigger requires user OAuth; sign in with `sl auth login`. Creating or signaling an execution accepts a user or an API key. Other typed commands accept the credentials allowed by the API route. A revision input's `connections` object is sent unchanged.

## Find and inspect

Search matches workflow display names. Search returns at most 20 results in one request and does not paginate. Without `--query`, `--limit 0` follows every list page.

Human execution rows show revision selection, cycle, and cycle start after the revision. Cycle `0` prints `0`. Human step-run rows show cycle, script version, and release id after the state. An absent script version is blank and script version `0` prints `0`. Human trigger rows show the signal and execution id from signal delivery; both are blank when the trigger has no signal delivery.

```sh
sl workflows list --query "lead follow-up" --limit 20 --json
sl workflows get WORKFLOW_ID --json
sl workflows revisions list --workflow WORKFLOW_ID --json
sl workflows executions list --workflow WORKFLOW_ID --filter 'state = "STATE_FAILED"' --json
sl workflows executions get EXECUTION_ID --workflow WORKFLOW_ID --json
sl workflows step-runs list --workflow WORKFLOW_ID --execution EXECUTION_ID --filter 'cycle = N' --json
sl workflows step-runs get SEQUENCE --workflow WORKFLOW_ID --execution EXECUTION_ID --json
sl workflows triggers list --workflow WORKFLOW_ID --json
sl workflows event-types list --json
```

Use the returned resource names and current `etag` values in later requests. Workflow and execution identifiers are the IDs used in their resource names.

`sl workflows step-runs list --filter` sends the filter unchanged. The only accepted form is `cycle = N`.

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

Omit `revision` to use the workflow's serving revision, or pass a valid revision resource name. `--request-id` is an optional idempotency key. The CLI does not retry writes; after an ambiguous response, inspect the resource before repeating the request.

```sh
sl workflows executions create --workflow WORKFLOW_ID --execution-id run-2026-01 --input execution.json --json
sl workflows executions get EXECUTION_ID --workflow WORKFLOW_ID --json
sl workflows executions watch EXECUTION_ID --workflow WORKFLOW_ID --json
sl workflows step-runs list --workflow WORKFLOW_ID --execution EXECUTION_ID --limit 100 --json
sl workflows step-runs get SEQUENCE --workflow WORKFLOW_ID --execution EXECUTION_ID --json
sl workflows executions cancel EXECUTION_ID --workflow WORKFLOW_ID --json
```

Watch emits complete snapshots as newline-delimited JSON and may coalesce intermediate changes. It ends after a terminal snapshot; if interrupted, rerun it to obtain the current snapshot.

Put the signal payload in the JSON body; `--request-id`, when supplied, is merged into that body. To target an observed open wait, supply both `--target-step-run` with its full step-run resource name and `--execution-uid` with the observed execution UID. Omitting both retains buffered signal delivery. Reuse the same request ID after an ambiguous response. `--start` creates the named execution when it is absent, then delivers the signal. `--start` alone sends an empty start object. `--start-revision` sets the revision resource name and `--start-input` sets the JSON execution input; both require `--start`. Do not combine `--start` with `--target-step-run` or `--execution-uid`.

```sh
sl workflows executions signal EXECUTION_ID --workflow WORKFLOW_ID --request-id REQUEST_UUID --input signal.json --json
sl workflows executions signal EXECUTION_ID --workflow WORKFLOW_ID --input signal.json --start --start-revision workflows/WORKFLOW_ID/revisions/REVISION_ID --start-input '{"x":1}' --json
```

## Manage triggers

Trigger creation requires user OAuth. Use the trigger's event type and body fields from the linked API contract. On create, `--signal` and `--execution-id` together set signal delivery: `--signal` is the signal name and `--execution-id` is the CEL expression for the execution id. Signal delivery is immutable, so update does not accept those flags. `--show-deleted` includes tombstoned triggers in list results.

```sh
sl workflows triggers create --workflow WORKFLOW_ID --trigger-id inbound --input trigger.json --signal payment_received --execution-id payload.customer --json
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
