# Publishing checklist

Status on 2026-11-17: **not published, not listed, not merged into an upstream provider catalog.** No npm token, PyPI token, registry account, or 2FA enrollment was created for this work. No package was uploaded.

The names below were not present in the public registries during this change (`npm view` and PyPI both returned 404). A 404 means the name was unused at that time. It does not reserve the name, and it does not prove the `@openany` npm scope exists or that this operator owns it. `openany-cli` is already published on npm by another maintainer account. Confirm the name with that publishing owner before uploading.

## Names in this repository

| Registry | Name in the manifest | Status |
| --- | --- | --- |
| npm | `openany-ai-sdk-provider` | Not published. Name returned 404. |
| PyPI | `langchain-openany` | Not published. Name returned 404. |

Alternative npm name if a scope is approved later: `@openany/ai-sdk-provider`. That scope was not verified. Do not rename in a published release without a changelog note.

Versions in the manifests are `0.1.0`. The dependencies that were verified are stable releases, not prereleases:

- `ai` 7.0.173
- `@ai-sdk/provider` 4.0.38
- `@ai-sdk/provider-utils` 5.0.81
- `@ai-sdk/openai-compatible` 3.0.92
- `undici` 8.12.4. Its published `engines.node` is `>=22.19.0`, and this package declares the same Node floor. npm warns on an older Node and refuses the install only when the installer enables `engine-strict`.
- Language Model Specification V4, which current AI SDK 7 docs call the provider interface
- `langchain-core` 1.8.1
- `langchain-tests` 1.1.9

## Still required before npm publish

1. Decide the final package name and confirm it is available and owned by the operator.
2. Create or use an npm account that is allowed to publish that name. Enable 2FA. npm requires 2FA for publishing.
3. Create a granular access token with publish rights. Store it in the CI secret store or a local password manager. Do not commit it and do not paste it into chat.
4. From `providers/js/openany-ai-sdk-provider`, run `npm ci`, `npm test`, and `npm pack --dry-run`. Inspect the tarball for `dist`, `README.md`, `LICENSE`, and `examples` only.
5. Run `npm publish --access public` only after the name and token are approved. The `prepublishOnly` script runs the tests again.
6. Provenance and a dedicated public GitHub repository are separate from this monorepo. Vercel's community-provider guide says to publish the provider in your own GitHub repository and as an npm package. This monorepo path does not satisfy that sentence by itself.

## Still required before PyPI publish

1. Confirm `langchain-openany` (or the approved rename) is available.
2. Create a PyPI account, verify the email address, and enable 2FA. PyPI requires 2FA before an API token can be created.
3. Create an API token scoped to that project. Store it outside the repository.
4. From `providers/python/langchain-openany`, run `uv sync --all-groups --frozen`, `uv run pytest`, and `uv build`.
5. Install the wheel in a clean virtual environment and import `ChatOpenAny`.
6. Upload with the project's chosen publisher (`uv publish` or `twine upload`) only after the token exists. Do not upload from this change.
7. LangChain's publish guide says the package should live in its own repository under the provider's GitHub organization. A later publishing step is to split `providers/python/langchain-openany` into that repository. Do not describe this monorepo as that repository.

## Still required before a Vercel community-provider listing

Official guide: https://ai-sdk.dev/providers/community-providers/custom-providers

1. Publish the npm package first. You host and maintain it.
2. Open a pull request to the AI SDK repository that adds a Community Providers page. The guide names the OpenRouter page as the template.
3. Use the draft in `providers/listing-drafts/vercel-ai-sdk-community-provider.md` only as local review material. It has not been submitted.
4. Do not claim the package is an official or community-listed provider until that pull request is merged.

## Still required before a LangChain listing

Official guides:

- https://docs.langchain.com/oss/python/contributing/integrations-langchain
- https://docs.langchain.com/oss/python/contributing/implement-langchain
- https://docs.langchain.com/oss/python/contributing/publish-langchain

1. Publish the PyPI package from the provider's own repository. New integrations are not accepted as pull requests to `langchain-ai` code repositories.
2. Keep `ChatModelUnitTests` passing. Do not point `ChatModelIntegrationTests` at the paid API unless a maintainer has a key and has agreed to the spend.
3. File an Integration listing issue in the LangChain docs repository (`langchain-ai/docs`). A maintainer applies `integration-run`. Automation opens the listing pull request. Do not open a manual listing pull request or add an MDX page. External manual listing pull requests are closed.
4. The default listing is an external YAML row. A hosted guide requires 50,000 monthly downloads or a maintainer featuring the package. Do not request a hosted guide now.
5. Suggested issue fields, for a human to copy later, are in `providers/listing-drafts/langchain-integration-issue.md`. That file is not an open issue.
6. Capability flags must stay accurate: stream is implemented; tool calling is not.

## Operator

Alephant AI LLC. Contact: team@openany.si. Project: https://openany.si. Repository: https://github.com/openanysi/openany-mcp.
