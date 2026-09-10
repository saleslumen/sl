# sl — the Saleslumen CLI

`sl` is the command-line interface for the public Saleslumen APIs: Campaigns, Emails, Workflows, Apps Script, and Resources. One static binary, API-key authentication, and JSON output that pipes cleanly.

```sh
sl auth login
sl campaigns list
sl campaigns people import --campaign 8b1c... --input people.json
sl emails messages list --limit 20 --json --jq '.messages[].id'
sl emails messages send --input message.json
sl emails drafts list --limit 10
sl workflows triggers update trg_123 --workflow wf_9 --update-mask enabled --input trigger.json
sl script versions restore 7 --project 4f2e...
sl script libraries lookup lib_...
sl resources organizations get org_...
sl resources namespaces list --organization org_...
```

## Install

```sh
curl -fsSL https://saleslumen.com/sl/install.sh | sh
```

The script downloads the latest GitHub Release into `/usr/local/bin` or `~/.local/bin`. Set `SL_INSTALL_DIR` to choose the directory, or `SL_INSTALL_VERSION=v0.1.0` to pin a tag.

Build from source (Go 1.25+):

```sh
git clone https://github.com/saleslumen/sl
cd sl
make build          # writes bin/sl
```

Or `go install github.com/saleslumen/sl@latest`.

## Authenticate

Create an organization API key in the Saleslumen console, then:

```sh
sl auth login                      # hidden prompt
sl auth login --with-token < key   # from stdin, for scripts
sl auth status                     # shows the key identifier, never the secret
sl auth status --product campaigns # one real request to confirm the key works
```

Keys are stored in `~/.sl/credentials` (mode `0600`) as TOML profiles:

```toml
[default]
api_key = "sl_key_..."
organization_id = "0f3c..."
namespace_id = ""
api_domain = "saleslumenapis.com"

[staging]
api_key = "sl_key_..."
api_domain = "staging.saleslumenapis.com"
```

Select a profile with `--profile staging` or `SL_PROFILE=staging`. Edit non-secret fields with `sl config set namespace_id <id>`.

Precedence for every setting is flag, then environment, then profile, then default:

| Flag | Environment | Profile key |
| --- | --- | --- |
| `--profile` | `SL_PROFILE` | — |
| — | `SL_API_KEY` | `api_key` |
| `--organization` | `SL_ORGANIZATION_ID` | `organization_id` |
| `--namespace` | `SL_NAMESPACE_ID` | `namespace_id` |
| — | `SL_API_DOMAIN` | `api_domain` |
| — | `SL_CONFIG_DIR` | — |

## Commands

```
sl <product> [<resource>] <verb> [ID] [flags]
```

- The primary resource ID is positional; a parent is a flag (`--campaign`, `--workflow`, `--project`, `--message`).
- A product's root resource attaches directly: `sl campaigns get ID`, `sl workflows publish ID`.
- `list` takes `--limit N` and follows pages for you.
- Request bodies come from `--input FILE` or `--input -` (stdin) as JSON, passed through unchanged. The CLI fills only the control fields the API requires (idempotency `request_id`, `etag`, `organization`) when your input omits them.
- Destructive verbs ask for confirmation; pass `--yes` in scripts.
- Streams (`sl campaigns tasks stream`, `sl emails tools verify`) print one JSON object per line.

Run `sl <product> --help` for the full tree. Endpoints that require a signed-in user rather than an API key (for example running scripts, starting workflow executions, or listing organizations) are intentionally absent; the parent command's help says so.

### Raw access

Anything the typed commands do not cover:

```sh
sl api campaigns /v1/campaigns
sl api workflows /v1/workflows/wf_9/executions --paginate
sl api emails /v1/labels -X POST --input label.json
sl api script /v1/projects -F pageSize=5
```

## Output

- Default: a table on a terminal; tab-separated rows without headers when piped.
- `--json`: the raw response body.
- `--jq EXPR`: filter the response with jq syntax (built in; no jq binary needed).
- `--verbose`: log `METHOD host path → status` to stderr. Never headers or bodies.

Data goes to stdout, diagnostics to stderr.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | API error, I/O error, or declined confirmation |
| 2 | Usage error: bad flags, arguments, `--jq`, or input JSON |
| 4 | No credentials, or the API returned 401/403 |

API errors are one line: `sl: <product> <METHOD> <path>: <CODE>: <message>`.

## Headers on the wire

Every request sends `sl-api-key` and, when configured, `sl-namespace-id`. Nothing else identifies you: the organization is derived server-side from the key. `--organization` only fills request bodies that the API requires it in.

## Development

```sh
make fmt      # gofmt
make test     # go test ./...
make check    # gofmt -l + go vet + go test
make build    # bin/sl with the git version embedded
```

Tests run against in-process HTTP servers and temporary config directories; they never touch your home directory or the network.

## Shell completion

```sh
sl completion bash > /etc/bash_completion.d/sl
sl completion zsh > "${fpath[1]}/_sl"
sl completion fish > ~/.config/fish/completions/sl.fish
```
