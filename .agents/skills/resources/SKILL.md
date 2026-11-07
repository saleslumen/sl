---
name: sl-resources
description: Use the public sl CLI to manage organizations and namespaces in Saleslumen.
---

# Organizations and namespaces with `sl`

Use `sl resources` for organization and namespace commands. Discover the exact syntax with `sl resources --help` and the relevant child help. Prefer typed commands; use `sl api resources <path>` only for a documented endpoint without a typed command.

Organization list and create commands require user OAuth. Namespace commands and organization get accept either OAuth or an organization API-key profile. Do not change profiles to work around an authorization error. `--organization` supplies an organization value to request paths or bodies that define it; it is not an HTTP header. `--namespace` sets the namespace context for the request.

Organization or namespace creation and deletion can have effects beyond the immediate response. Work within the user's authorized scope; ask when the target or effect is unclear. Read the operation or resource state until it reaches the documented result before reporting completion. After an ambiguous response, inspect state before retrying.

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Resources API overview](https://developers.saleslumen.com/en-US/management/guides/resources/overview.md.txt)
- [Resources getting started](https://developers.saleslumen.com/en-US/management/guides/resources/getting-started.md.txt)
- [Organizations API reference](https://developers.saleslumen.com/en-US/management/reference/apis/organizations.md.txt)
- [Namespaces API reference](https://developers.saleslumen.com/en-US/management/reference/apis/namespaces.md.txt)
