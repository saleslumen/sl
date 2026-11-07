---
name: sl
description: Use the Saleslumen `sl` CLI to read or operate Campaigns, Emails, Workflows, Apps Script, Resources, and Marketplace APIs.
---

# Saleslumen CLI

Use the installed `sl` command as the client for Saleslumen's public APIs. It covers several API products, so discover the exact resource, verbs, and flags from the command tree instead of guessing:

```sh
sl --help
sl <product> --help
sl <product> <resource> --help
```

Supported typed products are `campaigns`, `emails`, `workflows`, `script` (Apps Script), `resources`, and `marketplace`. Root resources such as campaigns and workflows put verbs directly below the product; other resources keep their noun. IDs are usually positional, while parent resources use flags such as `--campaign`, `--workflow`, or `--project`.

Load a product skill for that product's commands, request examples, authorization rules, and API docs:

```sh
sl skill campaigns
sl skill emails
sl skill workflows
sl skill script
sl skill resources
sl skill marketplace
```

## Authentication and scope

Check the selected profile with `sl auth status`. Do not print, copy into prompts, or expose API keys, access tokens, or refresh tokens. If no profile is ready, tell the user to run `sl auth login` and wait before making authenticated calls. OAuth login opens a browser; API-key login reads a secret through a hidden prompt or stdin.

Profiles use one mode at a time: OAuth or an organization API key. A command's help may require user OAuth; an API-key profile cannot satisfy that requirement. Do not switch profiles or credentials to get around an authorization error.

`--profile` selects a profile. `--organization` supplies an organization value only where a command's request body or resource path requires it; it does not set an organization header. `--namespace` supplies the `sl-namespace-id` request header.

## Make requests

Prefer the typed command shown by `--help`. Supply JSON bodies with `--input FILE` or `--input -` for stdin. The CLI adds documented control fields such as `requestId` when needed. If a request ID is present in the input, do not also pass `--request-id`; the CLI treats that as a conflict.

The CLI generates documented control fields such as `requestId` when the command supports it. Use the matching product skill for request-body examples and product-specific field requirements.

Use `--limit` only on list commands that expose it; the CLI follows the service's page tokens. Some lists are unpaginated and have no `--limit` flag.

Common CLI commands:

```sh
sl --help
sl auth status
sl skill <product>
```

Use `--json` for machine-readable response JSON and `--jq EXPR` for built-in jq filtering. Results go to stdout; diagnostics go to stderr. Message and draft output can contain private email content, so only request or forward the fields needed for the task.

## Raw API

Use `sl api` when the typed command tree does not cover the needed endpoint. It accepts the same six product host labels, an API path, `-X METHOD`, `-F key=value` query fields, and `--input FILE|-` for a JSON body. `--paginate` is GET-only and follows list pagination; it is not a way to bypass service authorization.

```sh
sl api emails /v1/messages -F maxResults=10 --json
```

## Documentation

- [Public `sl` source and complete command tree](https://github.com/saleslumen/sl)
- [Saleslumen product index](https://www.saleslumen.com/llms.txt)
- [Developers documentation index](https://developers.saleslumen.com/llms.txt)

## Writes and failures

Only send, create, update, delete, or otherwise change data when the user has authorized that action and its target. Destructive commands prompt for confirmation; `--yes` bypasses the prompt but does not grant authorization.

The CLI does not automatically retry requests. If a write returns an ambiguous transport or server error, inspect the response and request ID and verify the resource state before considering another write. A validation error is not fixed by retrying: correct the request using the service contract and the CLI's help output.

`--verbose` reports the HTTP method, host, path, and status to stderr. It does not print headers or bodies. Do not paste credentials or private message content into logs, prompts, or reports.

## Install this skill

This file is bundled into the `sl` executable. `sl skill` prints the general CLI skill. `sl skill <product>` prints a product skill for `campaigns`, `emails`, `workflows`, `script`, `resources`, or `marketplace`. Each command prints the exact Markdown packaged in that CLI version.

To install the general skill in a repository-local agent skills directory:

```sh
mkdir -p .agents/skills/sl
sl skill > .agents/skills/sl/SKILL.md
```

For example, install the Emails product skill:

```sh
mkdir -p .agents/skills/sl-emails
sl skill emails > .agents/skills/sl-emails/SKILL.md
```
