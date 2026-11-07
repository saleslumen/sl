---
name: sl-script
description: Use the public sl CLI for Apps Script projects, content, versions, deployments, and function runs.
---

# Apps Script with `sl`

Use `sl script` for the public Apps Script API. Discover supported commands and flags with `sl script --help`; use the child command's help for the exact request shape. Prefer typed commands, using `sl api script <path>` only when a documented endpoint is not represented by a typed command.

Apps Script supports projects and content, versions, deployments, libraries, and function execution. A run executes the specified function with the supplied parameters; check its process or execution result rather than treating request acceptance as completion. `sl script run` requires user OAuth. Do not switch credentials to bypass that requirement.

Work within the user's authorized scope; ask when the project, target, or effect is unclear. `sl` does not retry requests. After an ambiguous response, inspect state before repeating a write.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Apps Script overview](https://developers.saleslumen.com/en-US/apps-script/guides/overview.md.txt)
- [Manage project content](https://developers.saleslumen.com/en-US/apps-script/guides/manage-content.md.txt)
- [Versions and deployments](https://developers.saleslumen.com/en-US/apps-script/guides/versions-and-deployments.md.txt)
- [Run functions](https://developers.saleslumen.com/en-US/apps-script/guides/run-functions.md.txt)
