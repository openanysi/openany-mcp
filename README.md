# OpenAny MCP

One MCP for 3,200+ APIs and 168+ AI models: search APIs, inspect schemas and pricing, and run calls with one OpenAny key. Browsing is free; pay per call.

OpenAny (https://openany.si) is a remote MCP server. The recommended endpoint is the lite server.

**Endpoint:** https://mcp.openany.si

**Transport:** Streamable HTTP, not legacy SSE.

**Tools:** `openany_overview`, `search_apis`, `get_api`, `run_api`.

**Operator:** Alephant AI LLC.

**Pricing:** Pay per call, no subscription. Browsing, search, and price checks do not need a key. Check the current resource price and billing unit before approving a paid call.

Live catalog counts come from `openany_overview`. The separate model API base URL is https://api.openany.si/v1.

## Configuration

Add this remote server in a client that accepts a top-level `mcpServers` object. `Authorization: Bearer <OpenAny API key>` is optional. Only `run_api` needs it. Browsing, search, and pricing work without a key. `${OPENANY_API_KEY}` is the environment placeholder for that key. Do not hardcode a key. If a client refuses to connect while the variable is unset, omit the header until you execute a call.

```json
{
  "mcpServers": {
    "openany": {
      "url": "https://mcp.openany.si",
      "headers": {
        "Authorization": "Bearer ${OPENANY_API_KEY}"
      }
    }
  }
}
```

This block is the lite server. `https://mcp.openany.si/mcp-lite` is the same server. The header above is an optional example.

## Tools

`https://mcp.openany.si` is the lightweight server. `https://mcp.openany.si/mcp-lite` is the same server. It exposes four tools. `openany_overview` reports live catalog counts. `search_apis` finds models and APIs. Execution is pay per call, no monthly fee, with one API key.

- `openany_overview`: Return the current data API count, AI model count, APIs grouped by category, and models grouped by modality. No arguments, and it does not call the paid gateway.
- `search_apis`: Find OpenAny models and APIs by keywords and optional kind (api, model, or all; default all), provider, category, data_platform, data_product or capability.
- `get_api`: Read one model or API selected from search_apis.
- `run_api`: Execute one selected OpenAny API, or send one selected model to chat completions, with the caller's current credentials.

`https://mcp.openany.si/mcp` is the advanced full-catalog endpoint. It exposes 3,268 per-API tools across 45 providers, led by tikhub (1,009), anyapi (337), justoneapi (301), scrapecreators (193), and qcc (170). It does not include models. The list is in [TOOLS.md](TOOLS.md). Use it only when a workflow already has an exact tool name. It is not the default.

## Recommended server

When enabled and routed by the deployment, `https://mcp.openany.si` and
`https://mcp.openany.si/mcp-lite` use the same Streamable HTTP transport and expose four tools:
`openany_overview`, `search_apis`, `get_api` and `run_api`. One search covers the current model
catalog and the API catalog. `get_api` reads the selected model or API.
`run_api` executes an API through the gateway, or posts a model call to
`/v1/chat/completions` on the model API origin, using the caller's key.
Model ids, modalities, context length and pricing are taken from the live
`GET /v1/models` response and are omitted when that response does not provide
them. Pay per call, no monthly fee, one API key.

The implementation enables this interface by default; operators can disable
it with `MCP_LITE_ENABLED=false`. Public deployment availability is a separate
check. `openany_overview`, `search_apis` and `get_api` read metadata; `run_api` forwards a business
request and may incur charges, requiring the caller's gateway credentials and
spending authorization. Configure the recommended endpoint. Use the advanced
`/mcp` catalog only when a workflow needs one tool per API. See the [lightweight guide](docs/mcp.md).

## Authentication

- `openany_overview`, `search_apis`, `get_api`, and `tools/list` are public. Browsing, search, and pricing do not require an OpenAny API key.
- `run_api` requires an **OpenAny API key**. The gateway remains responsible for authentication, billing and execution. On the advanced `/mcp` catalog, each per-API tool also needs that key.
- **No OAuth:** this server does not implement an OAuth sign-in flow. It also does not collect a key with MCP form elicitation. URL elicitation is not offered, because OpenAny has no elicitation completion page. Create a key at https://openany.si/dashboard?page=api-keys and store it in the client's secure secret or connector settings. A `run_api` call with no key is rejected locally (`openany_mcp_credentials_missing`) and does not reach the gateway. A key the gateway rejects keeps its code, including the live `openany_key_invalid` and `openany_authentication_invalid`, plus the expired and revoked siblings. Never ask the user to paste the key into chat. A raw Authorization value with no scheme is sent as `Bearer <key>`. An unexpanded placeholder such as `${OPENANY_API_KEY}` is rejected locally as `openany_mcp_credentials_placeholder`.
- **Confirmed authentication header:**
  `Authorization: Bearer <OPENANY_API_KEY>`. The header is optional. Supply an OpenAny API key through
  `OPENANY_API_KEY` when executing; the examples include this confirmed header format.

The header format is confirmed by the maintainer for this documentation
update. This is separate from validating a particular account, client version
or business request. No new authenticated live call was performed during this
update; see the [verification record](docs/verification.md) for test scope.

Provide the key through `OPENANY_API_KEY` in the client's private environment
or secret settings when you execute a call. Never hardcode a key in the repository, paste it into an
agent prompt, or include it in a URL, screenshot, log or support message.
Ensure the client process can read that environment variable; a desktop app
does not necessarily inherit a terminal's environment.

## Client setup

Merge the appropriate snippet into existing settings rather than replacing
unrelated servers. The files below contain environment references, not real
keys. Every example uses `https://mcp.openany.si` and the confirmed Bearer header as an optional example; none is a claim
of an end-to-end paid test of every client.

| Client | Example | Destination |
| --- | --- | --- |
| Claude Code | [claude-code.sh](examples/claude-code.sh) | Run from a private shell. The header is added only when `OPENANY_API_KEY` is set |
| Cursor | [cursor.mcp.json](examples/cursor.mcp.json) | `~/.cursor/mcp.json` |
| Codex | [codex.config.toml](examples/codex.config.toml) | `~/.codex/config.toml` |
| Pi | [pi.mcp.json](examples/pi.mcp.json) | `~/.pi/agent/mcp.json` or `.pi/mcp.json` |
| Hermes | [hermes.config.yaml](examples/hermes.config.yaml) | `~/.hermes/config.yaml` |
| OpenClaw | [openclaw.json](examples/openclaw.json) | `~/.openclaw/openclaw.json`, under `mcp.servers` |
| Other clients | [generic.md](examples/generic.md) | Client-specific remote MCP settings |

The Hermes row is a manual `mcp_servers` example for the lite server.
The installable plugin is [plugins/openany](plugins/openany/README.md). Its
default is `https://mcp.openany.si`, because Hermes 0.23.0 stops
`tools/list` after 50 pages and the advanced `/mcp` catalog is truncated at 2500 tools.
A same-name `openany` server in `config.yaml` makes Hermes skip the plugin
server. The owner-run session, the CoinGecko tool result, request
`420decad-3a6b-46f8-acff-53966bbd8b1c`, tool cost 0, and 807 microUSD of model
calls are recorded in [docs/hermes-e2e.md](docs/hermes-e2e.md).

JSON does not support comments. The JSON examples use a top-level `$comment`
annotation for the warning and CLI notes; only the `mcpServers` or `mcp` block
is connection configuration. If your client rejects annotation fields, copy
that configuration block without `$comment`, retaining the warning separately.

The requested Claude Code command expands the environment variable before
invoking its CLI when the key is set. The resulting header can be visible to local process
inspection and may be stored in personal client configuration. Do not run it
with shell tracing or commit the generated configuration. The Pi/OpenClaw CLI
alternatives in annotations are alternatives, not additional commands to run
as a batch. OpenClaw's CLI alternative sets the endpoint and transport only;
the optional environment-backed header still needs to be merged from the JSON example when you execute a call.

Client reference documentation: [Claude Code](https://code.claude.com/docs/en/mcp),
[Cursor](https://cursor.com/docs/mcp),
[Codex](https://developers.openai.com/codex/mcp/),
[Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md),
[Hermes](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/),
[OpenClaw](https://docs.openclaw.ai/cli/mcp/transports).
Client versions and configuration support can change; consult the matching
client documentation if a field or command is rejected.

## Gemini CLI

Install this repository as a Gemini CLI extension:

```sh
gemini extensions install https://github.com/openanysi/openany-mcp
```

[`gemini-extension.json`](gemini-extension.json) names the extension `openany`.
It connects with `httpUrl` to `https://mcp.openany.si` and can send
`Authorization: Bearer ${OPENANY_API_KEY}`. The header is optional. The key is the sensitive setting
`OPENANY_API_KEY`. Gemini CLI asks for it during install and stores it outside
this repository. [`GEMINI.md`](GEMINI.md) is the extension context file.

## Cursor

[`.mcp.json`](.mcp.json) is the root file cursor.directory reads for an MCP
server. [`.cursor-plugin/plugin.json`](.cursor-plugin/plugin.json) is the
Cursor plugin manifest. It declares `OPENANY_API_KEY` as an optional string
variable and sets `mcpServers` to `.mcp.json`. The header in that file is
`Authorization: Bearer ${OPENANY_API_KEY}`. Cursor fills the placeholder from
the plugin setting when it is set. The file does not contain a key.

A manual merge into `~/.cursor/mcp.json` uses
[`examples/cursor.mcp.json`](examples/cursor.mcp.json), which expands
`${env:OPENANY_API_KEY}` and points at `https://mcp.openany.si`.

There is no root `mcp.json`. Cursor's plugin loader reads `.mcp.json` before
`mcp.json` when both exist, so the manifest points at the one file.

## OpenClaw

[`plugins/openclaw/openany/SKILL.md`](plugins/openclaw/openany/SKILL.md) is an
OpenClaw skill. Its front matter names `OPENANY_API_KEY` as `primaryEnv` and sets the
homepage to https://openany.si. The key is optional for browsing. The skill tells the agent to add
`https://mcp.openany.si` under `mcp.servers` in
`~/.openclaw/openclaw.json` with `transport` set to `streamable-http`, then
to call `search_apis`, `get_api`, confirm the returned price, and only then
`run_api`. See [the skill README](plugins/openclaw/openany/README.md).

[`examples/openclaw.json`](examples/openclaw.json) is the manual example
for the same lite URL.

## Which URL the install files use

The Gemini CLI extension, the Cursor `.mcp.json` plugin, the OpenClaw
skill, and the files under `examples/` use `https://mcp.openany.si`. Public `tools/list` on that URL
returns `openany_overview`, `search_apis`, `get_api`, and `run_api` in a single page and does not
send `nextCursor`. The advanced `https://mcp.openany.si/mcp` URL returns one tool
per API, 3,268 tools, 50 tools per page, with `nextCursor` until the last page.

That split matters for these clients. Cursor staff wrote in July 2026 and
again in September 2026 that the desktop MCP client registers only the first
`tools/list` page and does not follow `nextCursor`. Gemini CLI shortens a
tool name longer than 63 characters. The first page of `/mcp` is 50 tools
and is not the search, inspect, and run workflow. The recommended URL keeps that
workflow on four short names.
[`server.json`](server.json) uses `https://mcp.openany.si`.

Discovery for this choice was an unauthenticated `tools/list`. It was not an
authenticated `tools/call`.

## Verify without accidentally spending

1. Connect to https://mcp.openany.si using Streamable HTTP.
2. List tools and inspect the intended tool's current schema. Discovery alone
   does not prove authentication, balance access or successful execution.
3. Before a live call, review the current price, parameters and total spending
   limit, and obtain explicit approval. Configure the key privately.
4. Confirm the result of the authorized `tools/call`, its gateway status and
   available usage or billing record. Do not equate HTTP 200 with business
   success. Never repeat an uncertain paid request automatically.

No new authenticated live `tools/call` was performed during this documentation update.

## Registry metadata

[`server.json`](server.json) is the descriptor for the official MCP
registry's [2025-12-11 schema](https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json).
Its Authorization header is optional. The value is `Bearer {OPENANY_API_KEY}`,
`isRequired` is false, and `OPENANY_API_KEY` is a secret variable that is not required.
Browsing does not need the key. `run_api` does.

The registry description is limited to 100 characters, so its wording is
shorter than the full description above. Version `0.1.2` is the descriptor in this repository. Version `0.1.1` is published on the
official MCP Registry as `si.openany/openany`. Local setup does not run
`mcp-publisher publish`.

[`glama.json`](glama.json) names the Glama maintainer for this repository.

## Development

The existing Go server implementation, configuration, migrations and tests are
preserved. See [development and operator notes](docs/development.md),
[the lightweight connection guide](docs/mcp.md) and
[existing verification records](docs/verification.md). The recommended server
is the lite endpoint. The advanced `/mcp` catalog is the per-API surface.

## Links

- [OpenAny](https://openany.si)
- [Agent instructions](https://openany.si/AGENT.md)
- [Documentation](https://openany.si/docs)
- [Document index](https://openany.si/llms.txt)
- [Model API base URL](https://api.openany.si/v1)
- [Agent instruction snippets](AGENTS-snippet.md)

## License

[MIT](LICENSE). Copyright (c) 2026 Alephant AI LLC.
