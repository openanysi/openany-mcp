# OpenAny for Dify

The B2A (business to agent) marketplace. One MCP endpoint for 3,200+ APIs (social, SEO, crypto, company, finance, legal) and 168+ AI models (GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo, Kling). Browse free; pay per call.

This is a Dify **tool** plugin. It is not a model provider. A Dify agent or workflow can discover OpenAny APIs, read a schema and the current price, and run one selected call with the caller's own OpenAny API key.

Maker: Pistis Wang / OpenAny. Operator named on the public privacy policy: Alephant AI LLC. Support: team@openany.si. Website: https://openany.si. Docs: https://openany.si/docs/. Source: https://github.com/openanysi/openany-mcp.

The manifest `author` is `openanysi` because the current SDK requires `^[a-zA-Z0-9_-]{1,64}$`. That slug is the GitHub organization of this repository. It is not a claim that a Dify Marketplace publisher account exists or is verified.

## Tools

| Tool | What it does | Key | Charge |
| --- | --- | --- | --- |
| `openany_overview` | Live catalog counts, categories, and model modalities. No arguments. | No | No |
| `search_apis` | Keyword and filter search. Summaries, not full schemas. | No | No |
| `get_api` | Input schema, public examples, pricing, and `api_version` for one result. | No | No |
| `run_api` | One API or model call. Needs `api`, `api_version`, and `input` from `get_api`. | Yes | Pay per call |

Call order: `openany_overview` when the user asks what OpenAny offers, then `search_apis`, then `get_api`. Confirm the price shown by `get_api` before `run_api`. A displayed price is not a price lock. Live counts come from `openany_overview`. The figures 3,200+ and 168+ are product copy, not a substitute for that response.

The plugin speaks MCP Streamable HTTP to the fixed URL `https://mcp.openany.si` (the root, not `/mcp` or `/mcp-lite`). There is no separate public REST route for these four tools. The plugin does not scrape internal APIs.

## Credentials

The OpenAny API key is the provider secret `openany_api_key` (`secret-input`, `required: false`). It is not a tool argument.

Leave the secret empty to browse, search, and check prices. Create a key at https://openany.si/dashboard?page=api-keys only when you want to run a call, and save it in the Dify provider form. Do not paste it into the chat.

Saving the secret checks its shape on the plugin. That check does not call OpenAny and does not spend money. An empty secret is accepted. A placeholder such as `${OPENANY_API_KEY}`, `$OPENANY_API_KEY`, `<key>`, or `YOUR_API_KEY` is rejected. The gateway is the authority for whether a real key is valid, expired, or revoked, and that answer arrives only when `run_api` is invoked.

Some Dify builds have kept asking for a credential even when the field is optional (marketplace discussion langgenius/dify-plugins#1365, closed over issue language, not over a confirmed product fix). If a build will not enable the tools until the form is saved, save it with the secret empty. If that build still demands a non-empty value, the catalog tools in this package still omit the key, and validation still does not call the paid API. That UI requirement would be a Dify limitation, not a charge from this plugin.

## Install the package

From the repository root, with the Dify plugin CLI:

```sh
dify plugin package ./plugins/dify -o openany-0.1.0.difypkg
```

CLI used for the package in this change: `dify-plugin-linux-amd64` 0.6.15 from https://github.com/langgenius/dify-plugin-daemon/releases/tag/0.6.15.

In Dify, install that `.difypkg` from the workspace plugin page. This repository change did not click through Dify Community or Dify Cloud, so a menu label was not re-checked here. The smoke procedure below is the reproducible check until that runtime install is done.

Plugin distribution is free. There is no monthly fee for the plugin. OpenAny bills `run_api` per call on the caller's own key. This package does not claim a Dify revenue share.

## Debug against a Dify workspace

The debug key below is the Dify plugin daemon key from the workspace plugin debug card. It is not an OpenAny API key.

```sh
cd plugins/dify
python3 -m pip install -r requirements.txt
cp .env.example .env
```

Set `INSTALL_METHOD=remote`, `REMOTE_INSTALL_URL` to the host:port from the debug card, and `REMOTE_INSTALL_KEY` to that debug key. Then:

```sh
python3 -m main
```

Do not commit `.env`.

Python 3.12 and `dify-plugin==0.11.1` are the versions this package declares. `dify-plugin>=0.9.0` is the Marketplace floor. `meta.version` is the manifest schema version `0.0.1`. `minimum_dify_version` is omitted because this change did not establish a minimum Dify product version.

## Security

- The only request URL is `https://mcp.openany.si`.
- HTTP proxies are disabled for these requests. Redirects are not followed, so the API key is not sent to another host.
- The key is attached only as `Authorization: Bearer <key>` on the `run_api` `tools/call`. Initialize, the initialized notification, and catalog tools do not send it.
- Credential checks and argument checks that fail locally do not open a connection.
- `run_api` is sent once per invocation. A timeout after that request is sent is `execution_state=unknown`. The plugin does not retry it.
- Errors and returned JSON have the key replaced with `[REDACTED]` when the key is at least 8 characters.
- Catalog timeout is 30 seconds. `run_api` timeout is 120 seconds.

Details of what leaves the workspace are in [PRIVACY.md](PRIVACY.md). Marketplace submission steps are in [PUBLISHING.md](PUBLISHING.md).

## Tests and what they do not prove

From `plugins/dify`:

```sh
python3 -m unittest discover -s tests -v
```

Those tests load the manifest with `dify-plugin` 0.11.1 and mock the MCP HTTP exchange. They cover an optional and a required key, JSON arguments, tool errors, timeouts, non-JSON bodies, SSE, redirect and proxy refusal, and secret redaction. They do not install the plugin into Dify.

An optional live check, with no API key and no `run_api`, is:

```sh
python3 scripts/smoke_catalog.py
```

That script calls `openany_overview` on the public endpoint. It does not read `OPENANY_API_KEY`.
