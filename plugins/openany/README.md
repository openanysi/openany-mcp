# OpenAny for Hermes

OpenAny (https://openany.si) is a marketplace for agents. This portable Hermes plugin connects to OpenAny's hosted MCP server over Streamable HTTP.

The package does not bundle a catalog, does not call OpenAny's model API, and does not state how many models, APIs, or tools OpenAny offers. Use the current `tools/list` response.

## Package

Agent Plugins v1, loaded by Hermes as a portable plugin:

- `plugin.json`
- `mcp.json`
- `skills/openany/SKILL.md`

There is no Python entrypoint, hook, middleware, or Desktop script. The package does not download or replace its own files, and it does not send telemetry.

## Hermes surfaces

| Surface | Value |
| --- | --- |
| MCP server name | `openany` |
| Transport | `streamable-http` (not legacy SSE) |
| URL | `https://mcp.openany.si/mcp` |
| Authorization | `Authorization: Bearer ${OPENANY_API_KEY}` |
| Secret | `config_schema.api_key`, stored by Hermes as `OPENANY_API_KEY` |
| Approval | `trust: untrusted` on that server |
| Skill | `openany:openany` |

`plugin.json` has no `requires_hermes` field. The draft catalog entry uses `>=0.23.0` because that is the Hermes release this package was checked against. It is not a claim that older releases were tested.

Hermes interpolates `${OPENANY_API_KEY}` from the active profile secret scope. The plugin does not read `.env`, `auth.json`, another tool's login, or any other credential file. Do not put a key in `mcp.json`.

Hermes refuses to connect when that variable is unset or empty, instead of sending the literal placeholder. The OpenAny server's own `tools/list` can be public; this package still does not connect until the secret is set.

A `mcp_servers.openany` entry in Hermes `config.yaml` replaces this server, including its `trust` setting. The manual example at `examples/hermes.config.yaml` uses that same name. Remove or rename it when you want this package's approval gate.

## Network and billing

MCP traffic goes to OpenAny at `mcp.openany.si`, operated by Alephant AI LLC. After the secret is set, Hermes sends the bearer token on those requests.

Resource execution is pay-per-call against the caller's OpenAny prepaid balance. This package does not set `costs` or `spend_limit`: prices are chosen by OpenAny per resource and were not frozen in the package. `trust: untrusted` asks before each tool that lacks `readOnlyHint: true`, and fails closed for unattended runs (cron, non-interactive sessions) when nobody can answer.

No OAuth. No subscription is started by installing this plugin.

`https://mcp.openany.si/mcp-lite` answered a public `initialize` and `tools/list` during packaging and exposed `search_apis`, `get_api`, and `run_api`. It is not this package's server. Point Hermes at it only with a separate server entry, and do not enable both endpoints unless you intend to load both catalogs. `run_api` can charge. This package does not claim that `search_apis` or `get_api` are free; that behavior was not re-checked with `tools/call` here.

## Install

From a git checkout of this repository:

```sh
hermes plugins install "file://$(pwd)#plugins/openany" --no-enable --yes-deps
hermes plugins enable openany
```

A catalog install, after the entry exists, is `hermes plugins install openany`. Portable packages stay disabled until enabled.

Set `OPENANY_API_KEY` through Hermes:

- Desktop: Settings → Plugins → OpenAny → OpenAny API key
- Or the active profile's secret scope under the name `OPENANY_API_KEY`

Do not paste the key into a prompt, a shell trace, a screenshot, or this repository.

## Uninstall

```sh
hermes plugins disable openany
hermes plugins remove openany
```

`remove` is also available as `uninstall`. Removing the plugin does not close the OpenAny account or refund a balance. It also does not delete `OPENANY_API_KEY` from the Hermes profile. Remove that secret in Hermes settings if you do not want it kept.

If you also added a manual `mcp_servers.openany` entry, remove that separately with `hermes mcp remove openany`.

## Troubleshooting

| What you see | What it means |
| --- | --- |
| Plugin is installed but inactive | Portable packages install disabled. Run `hermes plugins enable openany`. |
| `${OPENANY_API_KEY} in url/headers is not set` | Hermes will not open the server until the profile secret is set. This is not a successful discovery. |
| Gateway HTTP 401 | The key was sent and OpenAny rejected it. Do not retry a paid call to "fix" authentication. |
| `Portable MCP server 'openany' conflicts with native config` | A manual `mcp_servers.openany` entry is replacing the package. The manual entry's trust setting wins. |
| Tools appear, then a call is blocked | `trust: untrusted` is waiting for approval, or the run is unattended and fails closed. |
| `tools/list` works | Discovery only. It does not prove authentication, balance, or a successful `tools/call`. |
| Long tool names look shortened | Hermes 0.23.0 clamps model-facing `mcp__openany__...` names to 64 characters. The server tool name is unchanged. |

Logs must not include the API key. If a command prints `Authorization`, stop and redact it before sharing the output.

## Validation record

Static checks and the Hermes commands run for this package are recorded in [docs/hermes-e2e.md](../../docs/hermes-e2e.md). A passing manifest check is not an authenticated tool call.

## License

MIT, the same license as this repository. Copyright (c) 2026 Alephant AI LLC.
