---
name: sl-emails
description: Use the public sl CLI for Emails accounts, messages, threads, labels, drafts, and address tools.
---

# Emails with `sl`

Use the public `sl` CLI for mailbox and email API tasks. Start with `sl emails --help`, then inspect help for the exact resource and operation. Prefer typed commands; use `sl api emails <path>` only for a documented public endpoint without a typed command.

Emails commands cover accounts, messages, threads, labels, drafts, attachments, and discovery or verification tools. For `sl emails messages send`, `message.accountId` is the UUID of an Emails account, not the sender's email address. Sending mail changes the outside world: do it only when the user authorized the recipient and content. `sl` does not retry requests, so verify message state after an ambiguous send error before retrying.

Message and draft responses can contain private email content. Request and share only the fields needed. Use `--json` or `--jq` for structured output. Check command help for pagination; accounts and labels are not paginated like message lists.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Emails API overview](https://developers.saleslumen.com/en-US/emails/guides/overview.md.txt)
- [Send messages](https://developers.saleslumen.com/en-US/emails/guides/sending.md.txt)
- [Drafts](https://developers.saleslumen.com/en-US/emails/guides/drafts.md.txt)
- [Labels](https://developers.saleslumen.com/en-US/emails/guides/labels.md.txt)
- [Emails API reference](https://developers.saleslumen.com/en-US/emails/reference/apis/messages.md.txt)
