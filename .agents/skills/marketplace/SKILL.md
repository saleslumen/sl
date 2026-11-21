---
name: sl-marketplace
description: Use the public sl CLI to browse Marketplace apps, publish releases, and manage installations and connections.
---

# Marketplace with `sl`

Use `sl marketplace` for Marketplace catalog, publishing, review, installation, and connection operations. Discover the exact command and flags with `sl marketplace --help`. Prefer typed commands; use `sl api marketplace <path>` only for a documented endpoint without a typed command.

Only `sl marketplace list` accepts an organization API-key profile; other Marketplace commands require user OAuth. Do not switch credentials to bypass this restriction. Review the app and its current permission digest before installing. Work within the user's authorized scope; ask when the organization, namespace, app, or effect is unclear.

Installed Marketplace operations run through Apps Script and have no typed `sl` command. Run one with the Script host's raw API command and a JSON body containing the operation key and input:

```sh
sl api script /v1/installations/INSTALLATION_ID:run -X POST --input run.json
```

This call requires user OAuth. `sl` does not retry requests; inspect operation or installation state after an ambiguous response before repeating a write.

## Examples

Browse the public catalog and inspect an app:

```sh
sl marketplace list --limit 20 --json
sl marketplace get APP_ID --json
sl marketplace publisher-apps list --json
```

Inspect installations and their connections:

```sh
sl marketplace installations list --limit 20 --json
sl marketplace installations get INSTALLATION_ID --json
sl marketplace connections list --installation INSTALLATION_ID --json
```

Review the app's permission digest and request shape before installing it:

```sh
sl marketplace installations create --input install.json --json
```

Publisher workflows use submissions and releases. Check their help and the publishing guide before creating or changing a release:

```sh
sl marketplace submissions --help
sl marketplace releases create --help
sl marketplace submissions list --app APP_ID --json
sl marketplace releases list --app APP_ID --json
```

After the user approves the release details, create the release and submit it for review with the documented request bodies:

```sh
sl marketplace releases create --app APP_ID --input release.json --json
sl marketplace submissions create --app APP_ID --input submission.json --json
```

Read these docs for the API contract:

- [sl CLI](https://developers.saleslumen.com/en-US/sdk/sl.md.txt)
- [Marketplace overview](https://developers.saleslumen.com/en-US/marketplace/guides/overview.md.txt)
- [Publish an app](https://developers.saleslumen.com/en-US/marketplace/guides/publish-an-app.md.txt)
- [Catalog and install](https://developers.saleslumen.com/en-US/marketplace/guides/catalog-and-install.md.txt)
- [Install Marketplace apps with `sl`](https://developers.saleslumen.com/en-US/sdk/sl/install-marketplace-apps.md.txt)
