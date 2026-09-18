# sl — the Saleslumen CLI

`sl` is the command-line interface for the public Saleslumen APIs: Campaigns, Emails, Workflows, Apps Script, and Resources. One static binary, exclusive oauth or API-key profiles, and JSON output that pipes cleanly.

```sh
sl auth login
sl campaigns list
sl campaigns people import --campaign 8b1c... --input people.json
sl emails messages list --limit 20 --json --jq '.messages[].id'
sl emails messages send --input message.json
sl emails drafts list --limit 10
sl workflows executions start --workflow wf_9 --input execution.json
sl script run script_... --input run.json
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

A profile is exactly one mode: browser OAuth or an organization API key. Completing either login unsets the other mode in that profile.

```sh
sl auth login                         # browser OAuth (PKCE)
sl auth login --api-key               # hidden prompt for an organization API key
sl auth login --with-token < key      # API key from stdin, for scripts
sl auth status                        # profile, mode, and identifiers; never tokens or secrets
sl auth status --product campaigns    # one real request to confirm the credential works
sl auth logout                        # remove the profile
```

`--api-key` and `--with-token` are mutually exclusive. OAuth stores `auth_mode`, access and refresh tokens, expiry, `oauth_user_id`, and `organization_id` from claim `sl_organization_id`. API-key login writes locally and makes no request. `--organization` on API-key login stores the body-field projection.

Credentials live in `~/.sl/credentials` (directory `0700`, file `0600`) as TOML profiles:

```toml
[default]
auth_mode = "oauth"
oauth_access_token = "..."
oauth_refresh_token = "..."
oauth_expires_at = "2026-09-18T13:00:00Z"
oauth_user_id = "..."
organization_id = "0f3c..."
namespace_id = ""
api_domain = "saleslumenapis.com"

[staging]
auth_mode = "api_key"
api_key = "sl_key_..."
api_domain = "staging.saleslumenapis.com"
```

Select a profile with `--profile staging` or `SL_PROFILE=staging`. Edit `organization_id`, `namespace_id`, and `api_domain` with `sl config set`.

Precedence for every setting is flag, then environment, then profile, then default. `SL_API_KEY` applies only to `api_key` profiles; it never mixes into an `oauth` profile.

| Flag | Environment | Profile key |
| --- | --- | --- |
| `--profile` | `SL_PROFILE` | — |
| — | `SL_API_KEY` | `api_key` (api_key mode only) |
| `--organization` | `SL_ORGANIZATION_ID` | `organization_id` |
| `--namespace` | `SL_NAMESPACE_ID` | `namespace_id` |
| — | `SL_API_DOMAIN` | `api_domain` |
| — | `SL_CONFIG_DIR` | — |

An expired oauth access token is refreshed automatically. Refresh failure is an error; the stale token is not sent and there is no API-key fallback.

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
- `sl script run`, `sl workflows executions start`, `sl workflows executions resume`, and `sl workflows triggers create` require user OAuth (`sl auth login`). An API-key profile exits 4 with `user OAuth login required`.

Run `sl <product> --help` for the full tree.

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
| 4 | No credentials, user OAuth required, or the API returned 401/403 |

API errors are one line: `sl: <product> <METHOD> <path>: <CODE>: <message>`. Missing credentials print `sl: no credentials found; run 'sl auth login'`. A user-only command on an API-key profile prints `sl: user OAuth login required`.

## Headers on the wire

A request sends exactly one credential header: `Authorization: Bearer <access-token>` for oauth, or `sl-api-key` for api_key. Never both, and never a fallback between modes. When configured, it also sends `sl-namespace-id`. `--organization` only fills request bodies that the API requires it in; it is never a header.

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
