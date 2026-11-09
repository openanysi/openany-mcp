# OpenAny for Hermes

OpenAny (https://openany.si) is a marketplace for agents. This portable Hermes plugin connects to OpenAny's hosted MCP server over Streamable HTTP.

The default URL is `https://mcp.openany.si/mcp-lite`. That server exposes `search_apis`, `get_api`, and `run_api`, so an agent can search the catalog, read one API, and execute it. Hermes 0.23.0 stops `tools/list` after 50 pages. The full endpoint `https://mcp.openany.si/mcp` exceeds that cap and is truncated at 2500 tools, which hides the rest of the catalog. A provider path such as `https://mcp.openany.si/mcp/coingecko` fits under the cap but lists only that provider.

The package does not bundle a catalog and does not state how many models, APIs, or tools OpenAny offers.

## Package

Agent Plugins v1, loaded by Hermes as a portable plugin:

- `plugin.json` version `0.2.0`
- `mcp.json`
- `skills/openany/SKILL.md`

There is no Python entrypoint, hook, middleware, or Desktop script. The package does not download or replace its own files, and it does not send telemetry.

## Hermes surfaces

| Surface | Value |
| --- | --- |
| MCP server name | `openany` |
| Transport | `streamable-http` (not legacy SSE) |
| URL | `https://mcp.openany.si/mcp-lite` |
| Authorization | `Authorization: Bearer ${OPENANY_API_KEY}` |
| Secret | `config_schema.api_key`, stored by Hermes as `OPENANY_API_KEY` |
| Approval | `trust: untrusted` on that server |
| Skill | `openany:openany` |

`plugin.json` has no `requires_hermes` field. The draft catalog entry uses `>=0.23.0` because that is the Hermes release this package was checked against.

Hermes interpolates `${OPENANY_API_KEY}` from the active profile secret scope. The plugin does not read `.env`, `auth.json`, another tool's login, or any other credential file. Do not put a key in `mcp.json`.

Hermes refuses to connect when that variable is unset or empty, instead of sending the literal placeholder. Unauthenticated `tools/list` can still succeed against the URL with no Authorization header. This package does not connect until the secret is set.

A `mcp_servers.openany` entry in Hermes `config.yaml` replaces this server. Hermes then skips the packaged server, including its `trust: untrusted` gate. The manual example at `examples/hermes.config.yaml` uses that name. Remove it before enabling this plugin. On the owner-run live session below, that native entry was removed so the plugin server could load.

To add a provider-scoped server without disabling this one, use a different name:

```yaml
mcp_servers:
  openany_coingecko:
    url: "https://mcp.openany.si/mcp/coingecko"
    headers:
      Authorization: "Bearer ${OPENANY_API_KEY}"
    trust: untrusted
```

A missing provider id returns HTTP 404. Do not point this block at `/mcp` if you need Hermes to list every tool: Hermes 0.23.0 truncates that list at 50 pages.

## Network and billing

MCP traffic goes to OpenAny at `mcp.openany.si`, operated by Alephant AI LLC. After the secret is set, Hermes sends the bearer token on those requests.

Resource execution is pay-per-call against the caller's OpenAny prepaid balance. This package does not set `costs` or `spend_limit`. `trust: untrusted` asks before each tool that lacks `readOnlyHint: true`, and fails closed for unattended runs when nobody can answer. The lite tools observed in public discovery had no `readOnlyHint`.

`search_apis` and `get_api` are catalog reads. `run_api` forwards a business request and can charge. This package does not claim a new measurement of that billing split beyond the tool descriptions returned by `tools/list`.

No OAuth. No subscription is started by installing this plugin.

## Verified live session

On a separate machine, with Hermes v0.23.0, `mcp==2.3.0`, and model `deepseek-v4.1-flash` via `https://api.openany.si/v1`, the owner ran this plugin after two changes: the native `mcp_servers.openany` entry was removed, and the plugin server URL was set to `https://mcp.openany.si/mcp/coingecko` because the full `/mcp` list exceeds Hermes's 50-page cap.

That session called `coingecko_get_api_v3_simple_price`. The tool returned `{"bitcoin":{"usd":75190}}`. The gateway request id was `420decad-3a6b-46f8-acff-53966bbd8b1c`. The tool cost was 0. Model calls cost 807 microUSD in total.

That call used the CoinGecko provider endpoint, not `run_api` on `/mcp-lite`. It is an authenticated tool result from that session. It is not a new measurement of the lite server, and it is not a price list for other tools.

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

If a manual `mcp_servers.openany` entry remains, remove it with `hermes mcp remove openany`. Leaving it in place skips this plugin.

## Troubleshooting

| What you see | What it means |
| --- | --- |
| Plugin is installed but inactive | Portable packages install disabled. Run `hermes plugins enable openany`. |
| `${OPENANY_API_KEY} in url/headers is not set` | Hermes will not open the server until the profile secret is set. This is not a successful discovery. |
| Gateway HTTP 401 | The key was sent and OpenAny rejected it. Do not retry a paid call to fix authentication. |
| `Portable MCP server 'openany' conflicts with native config` | A manual `mcp_servers.openany` entry is replacing the package. Remove that entry. The owner-run session had to do this before the plugin server loaded. |
| Only some of the full catalog appears | The server is still `https://mcp.openany.si/mcp`, or Hermes stopped after 50 pages. This package's default is `/mcp-lite`. |
| Tools appear, then a call is blocked | `trust: untrusted` is waiting for approval, or the run is unattended and fails closed. |
| `tools/list` works | Discovery only. It does not prove authentication, balance, or a successful `tools/call`. |
| Long tool names look shortened | Hermes 0.23.0 clamps model-facing `mcp__openany__...` names to 64 characters. The lite tool names fit. The server tool name is unchanged. |

Logs must not include the API key. If a command prints `Authorization`, stop and redact it before sharing the output.

## Validation record

Static checks, Hermes commands, and the owner-run session are recorded in [docs/hermes-e2e.md](../../docs/hermes-e2e.md). Public discovery of `/mcp-lite` is not an authenticated tool call. The CoinGecko result above is a separate owner-run call.

## License

MIT, the same license as this repository. Copyright (c) 2026 Alephant AI LLC.
