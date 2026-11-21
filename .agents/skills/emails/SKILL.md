---
name: sl-emails
description: Use the public sl CLI for Emails accounts, messages, threads, labels, drafts, and address tools.
---

# Emails with `sl`

Use the public `sl` CLI for mailbox and email API tasks. Start with `sl emails --help`, then inspect help for the exact resource and operation. Prefer typed commands; use `sl api emails <path>` only for a documented public endpoint without a typed command.

Emails provides accounts, messages, threads, labels, drafts, attachments, discovery, and verification tools. `sl` does not retry requests; after an ambiguous send error, inspect message state before retrying.

Message and draft responses can contain private email content. Request and share only the fields needed. Use `--json` or `--jq` for structured output. Check command help for pagination; accounts and labels are not paginated like message lists.

## Examples

Find mailbox accounts and use their account IDs to filter message and thread lists. Messages can also be narrowed with the message query and label filters. Lists of accounts and labels are unpaginated:

```sh
sl emails accounts list --json
sl emails accounts get ACCOUNT_UUID --json
sl emails messages list --account ACCOUNT_ID --limit 20 --json
sl emails messages get MESSAGE_ID --json
sl emails threads list --account ACCOUNT_ID --limit 20 --json
sl emails threads list --query "project update" --limit 10 --json
sl emails threads get THREAD_ID --json
sl emails labels list --json
sl emails drafts list --limit 20 --json
```

Create a user label or draft with the request shape from child help and the linked docs:

```sh
sl emails labels create --name "Follow up" --json
sl emails drafts create --input draft.json --json
sl emails drafts update DRAFT_ID --input draft-update.json --json
```

Update mailbox accounts only when authorized to change those account settings:

```sh
sl emails accounts batch-update --input account-updates.json --json
```

Sending mail changes the outside world; send only when the user authorized the recipient and content. For `sl emails messages send`, `message.accountId` is the mailbox's Emails account UUID. Include the mailbox's `From` and at least one recipient header in `message.payload.headers`. Save the request as `send.json`; encode request body part data as base64url (`SGVsbG8=` is `Hello`). `sl` generates `requestId` when omitted.

```json
{
  "message": {
    "accountId": "EMAILS_ACCOUNT_UUID",
    "payload": {
      "headers": [
        { "name": "From", "value": "mailbox@example.com" },
        { "name": "To", "value": "recipient@example.com" },
        { "name": "Subject", "value": "Hello" }
      ],
      "parts": [
        { "mimeType": "text/plain", "body": { "data": "SGVsbG8=" } }
      ]
    }
  }
}
```

After confirming the recipient and content are authorized, send it with:

```sh
sl emails messages send --input send.json --json
```

Read an attachment or verify an address with the documented request body:

```sh
sl emails attachments get ATTACHMENT_ID --message MESSAGE_ID --json
sl emails tools verify --input verify.json --json
```

Find a person's work email by domain and name:

```sh
sl emails tools discover --domain example.com --name "Jane Doe" --json
```

Finder may return `pending` or `running`; poll the same discovery endpoint until it returns `success` or `not_found`. It charges 10 credits only when it finds an address. `not_found` results and status polling are not charged. See the Tools reference for response shapes and authentication requirements.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Emails API overview](https://developers.saleslumen.com/en-US/emails/guides/overview.md.txt)
- [Send messages](https://developers.saleslumen.com/en-US/emails/guides/sending.md.txt)
- [Drafts](https://developers.saleslumen.com/en-US/emails/guides/drafts.md.txt)
- [Labels](https://developers.saleslumen.com/en-US/emails/guides/labels.md.txt)
- [Emails API reference](https://developers.saleslumen.com/en-US/emails/reference/apis/messages.md.txt)
- [Emails Tools reference](https://developers.saleslumen.com/en-US/emails/reference/apis/tools.md.txt)
