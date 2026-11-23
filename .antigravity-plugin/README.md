# OpenAny for Google Antigravity

The B2A (business to agent) marketplace. This plugin connects Antigravity to the hosted MCP server at `https://mcp.openany.si`.

It wraps that remote server only. It does not install a local process, change the OpenAny service, or start a subscription.

## What you get

One Streamable HTTP server, `openany`, at `https://mcp.openany.si`. Do not point this plugin at `https://mcp.openany.si/mcp`. That path is the advanced per-API catalog, not this package.

Antigravity namespaces the server as `openany_openany`. The lite server exposes:

- `openany_overview`, `search_apis`, and `get_api` read the catalog. They do not call the paid gateway.
- `run_api` runs one selected API or model call. It is pay per call, with no monthly fee. Read the price from `get_api` before you approve it.

The catalog has 4,400+ APIs across social, SEO and search, crypto, company and leads, finance, and legal, plus 168+ AI models from families including GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo, and Kling. Live counts come from `openany_overview`.

Website: https://openany.si

Docs: https://openany.si/docs/

Contact: team@openany.si

Maker: Pistis Wang, Founder, OpenAny. Alephant AI LLC.

## Install

The plugin root is this repository. The manifest is `.antigravity-plugin/plugin.json` because the repo also ships other clients. MCP servers stay in `mcp_config.json` at the repository root.

From a local checkout:

```sh
agy plugin validate .
agy plugin install .
```

From GitHub, once this packaging is on the default branch:

```sh
agy plugin install https://github.com/openanysi/openany-mcp
```

`agy plugin validate --strict` is for a plugin that contains only Antigravity files. This repository also has `.mcp.json`, `gemini-extension.json`, `.cursor-plugin/plugin.json`, and other client packages. Antigravity warns on those and does not use them. The [build-plugins guide](https://antigravity.google/docs/build-plugins/) says Marketplace packaging does not fail on those warnings. Validate without `--strict`, or filter the documented other-tool warnings.

There is no OAuth sign-in. OpenAny does not collect the API key inside the MCP session.

## API key

`mcp_config.json` declares `OPENANY_API_KEY` and sends `Authorization: Bearer ${OPENANY_API_KEY}`. That value is a placeholder. The repository does not contain a key.

Antigravity treats every declared MCP variable as required. Installing the plugin leaves it disabled until you enter `OPENANY_API_KEY` in the setup dialog (Customizations > Installed, or `/plugin` in the CLI). The official schema has no optional-variable field. The committed header is only that placeholder. Antigravity expands it from the dialog.

Create a key at https://openany.si/dashboard?page=api-keys and store it in that dialog. Do not paste it into chat, commit it, or put it in a prompt.

Catalog calls do not charge. `run_api` needs the key and can charge the caller's OpenAny account.
