# OpenAny MCP

OpenAny (https://openany.si) is the marketplace for agents: one key for 168+ AI
models and 3,200+ APIs. No subscriptions, no per-provider signups. Top up once,
pay per call. This repo is OpenAny's remote MCP server (Streamable HTTP).

**Endpoint:** https://mcp.openany.si/mcp

**Transport:** Streamable HTTP, not legacy SSE.

**Operator:** Alephant AI LLC.

**Pricing:** Pay per call, no subscription. Check the current resource price
and billing unit before approving a paid call.

OpenAny provides model access and callable APIs under one API key and prepaid
balance. The supplied catalog snapshot is approximately **168+ models**,
**3,201 APIs** and **3,220 MCP tools**. These are approximate, changing catalog
figures supplied for this documentation, not independently verified live counts
or a guarantee that every model is exposed as an MCP tool. Use the current
`tools/list` response and tool schemas to determine MCP availability. The
separate model API base URL is https://api.openany.si/v1.

## Optional lightweight endpoint

When enabled and routed by the deployment, `https://mcp.openany.si/mcp-lite`
uses the same Streamable HTTP transport and exposes just three tools:
`search_apis`, `get_api` and `run_api`. It lets an agent search the API catalog,
inspect one API's schema and pricing, then execute the selected API without
loading the full independent-tool catalog into context.

The implementation enables this interface by default; operators can disable
it with `MCP_LITE_ENABLED=false`. Public deployment availability is a separate
check. `search_apis` and `get_api` read metadata; `run_api` forwards a business
request and may incur charges, requiring the caller's gateway credentials and
spending authorization. Configure one endpoint, not both, unless your workflow
explicitly needs both catalogs. See the [lightweight guide](docs/mcp.md).
The examples and registry descriptor in this repository continue to use `/mcp`.

## Authentication

- `tools/list` is public; discovery does not require an OpenAny API key.
- Resource execution through `tools/call` requires an **OpenAny API key**.
  The gateway remains responsible for authentication, billing and execution.
- **No OAuth:** this server does not implement an OAuth sign-in flow.
- **Confirmed authentication header:**
  `Authorization: Bearer <OPENANY_API_KEY>`. Supply an OpenAny API key through
  `OPENANY_API_KEY`; the examples use this confirmed header format.

The header format is confirmed by the maintainer for this documentation
update. This is separate from validating a particular account, client version
or business request. No new authenticated live call was performed during this
update; see the [verification record](docs/verification.md) for test scope.

Provide the key through `OPENANY_API_KEY` in the client's private environment
or secret settings. Never hardcode a key in the repository, paste it into an
agent prompt, or include it in a URL, screenshot, log or support message.
Ensure the client process can read that environment variable; a desktop app
does not necessarily inherit a terminal's environment.

## Client setup

Merge the appropriate snippet into existing settings rather than replacing
unrelated servers. The files below contain environment references, not real
keys. Every example uses the confirmed Bearer header format; none is a claim
of an end-to-end paid test of every client.

| Client | Example | Destination |
| --- | --- | --- |
| Claude Code | [claude-code.sh](examples/claude-code.sh) | Run from a private shell with `OPENANY_API_KEY` available |
| Cursor | [cursor.mcp.json](examples/cursor.mcp.json) | `~/.cursor/mcp.json` |
| Codex | [codex.config.toml](examples/codex.config.toml) | `~/.codex/config.toml` |
| Pi | [pi.mcp.json](examples/pi.mcp.json) | `~/.pi/agent/mcp.json` or `.pi/mcp.json` |
| Hermes | [hermes.config.yaml](examples/hermes.config.yaml) | `~/.hermes/config.yaml` |

The Hermes row is a manual `mcp_servers` example. The installable plugin is
[plugins/openany](plugins/openany/README.md). A same-name `openany` server in
`config.yaml` replaces that plugin's server.
| OpenClaw | [openclaw.json](examples/openclaw.json) | `~/.openclaw/openclaw.json`, under `mcp.servers` |
| Other clients | [generic.md](examples/generic.md) | Client-specific remote MCP settings |

JSON does not support comments. The JSON examples use a top-level `$comment`
annotation for the warning and CLI notes; only the `mcpServers` or `mcp` block
is connection configuration. If your client rejects annotation fields, copy
that configuration block without `$comment`, retaining the warning separately.

The requested Claude Code command expands the environment variable before
invoking its CLI. The resulting header can be visible to local process
inspection and may be stored in personal client configuration. Do not run it
with shell tracing or commit the generated configuration. The Pi/OpenClaw CLI
alternatives in annotations are alternatives, not additional commands to run
as a batch. OpenClaw's CLI alternative sets the endpoint and transport only;
the environment-backed header still needs to be merged from the JSON example.

Client reference documentation: [Claude Code](https://code.claude.com/docs/en/mcp),
[Cursor](https://cursor.com/docs/mcp),
[Codex](https://developers.openai.com/codex/mcp/),
[Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md),
[Hermes](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/),
[OpenClaw](https://docs.openclaw.ai/cli/mcp/transports).
Client versions and configuration support can change; consult the matching
client documentation if a field or command is rejected.

## Verify without accidentally spending

1. Connect to the full endpoint URL using Streamable HTTP.
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
Its required Authorization header describes the confirmed authenticated-call
configuration, not a credential requirement for public `tools/list`.

The registry description is limited to 100 characters, so its wording is
shorter than the full description above. Version `0.1.0` is published on the
official MCP Registry as `si.openany/openany`. Local setup does not run
`mcp-publisher publish`.

## Development

The existing Go server implementation, configuration, migrations and tests are
preserved. See [development and operator notes](docs/development.md),
[the lightweight connection guide](docs/mcp.md) and
[existing verification records](docs/verification.md). The optional lightweight
interface is separate from the full `/mcp` endpoint documented here.

## Links

- [OpenAny](https://openany.si)
- [Agent instructions](https://openany.si/AGENT.md)
- [Documentation](https://openany.si/docs)
- [Document index](https://openany.si/llms.txt)
- [Model API base URL](https://api.openany.si/v1)
- [Agent instruction snippets](AGENTS-snippet.md)

## License

[MIT](LICENSE). Copyright (c) 2026 Alephant AI LLC.
