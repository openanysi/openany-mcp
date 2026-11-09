# Hermes plugin validation

Date: 2026-11-09. Repository branch `cursor/hermes-openany-plugin-beb6`.

## Default endpoint, 2026-11-09 follow-up

Package version `0.2.0` connects to `https://mcp.openany.si/mcp-lite`.

The full endpoint `https://mcp.openany.si/mcp` is not a reliable Hermes
default. Hermes 0.23.0 stops `tools/list` after 50 pages and, on that URL,
logs `tools pagination exceeded 50 pages; truncating at 2500 items`. An
unauthenticated probe continued past page 50. Tools after the cap never
reach the model.

Options checked with unauthenticated `initialize` and `tools/list` only:

| Endpoint | Hermes `mcp test` | Independent page walk | Catalog access |
| --- | --- | --- | --- |
| `/mcp-lite` | Exit 0, 2638ms, Auth none, 3 tools, no pagination warning | 1 page, 3 tools, no `nextCursor`. Server `OpenAny Lite` 0.1.0, protocol `2025-11-25`. Names: `get_api`, `run_api`, `search_apis`. None had `readOnlyHint`. | Search, read, and execute across the catalog through those three tools. |
| `/mcp/coingecko` | Exit 0, 2921ms, Auth none, 55 tools, no pagination warning | 2 pages, 55 unique tools, cursor finished. Includes `coingecko_get_api_v3_simple_price`. | That provider only. |
| `/mcp/not-a-real-provider` | not sent | HTTP 404 `404 page not found` | None. |
| `/mcp` | Earlier same-day run: exit 0, 16674ms, 2500 tools, then the 50-page warning | Page 51 and 52 still returned tools | Incomplete under Hermes. |

A user-configurable URL was not put in `mcp.json`. Hermes fails closed when a
`${VAR}` in a remote URL is unset, and the portable manifest has no separate
default. The working default is the literal lite URL. A provider server can
be added under another `mcp_servers` name, with `trust: untrusted` and the
same bearer placeholder. The name `openany` must not be reused.

### Owner-run authenticated session

Reported from a separate machine. This workspace did not repeat the call and
did not see the key.

- Hermes v0.23.0, `mcp==2.3.0`
- Model `deepseek-v4.1-flash` through `https://api.openany.si/v1`
- The installed portable plugin was used
- Native `mcp_servers.openany` skipped the plugin server, so that entry was removed
- The plugin URL for that session was `https://mcp.openany.si/mcp/coingecko`, not `/mcp` and not `/mcp-lite`
- Tool `coingecko_get_api_v3_simple_price` returned `{"bitcoin":{"usd":75190}}`
- Gateway request `420decad-3a6b-46f8-acff-53966bbd8b1c`
- Tool cost 0
- Model calls cost 807 microUSD total

That is one authenticated tool result on the provider endpoint. It does not
measure `run_api` on the lite server, and it does not set a price for any
other tool.

### Later owner-run on `/mcp-lite`

A later session the same day used the packaged `/mcp-lite` server. Hermes
called `search_apis`, then `get_api`, then `run_api` for
`coingecko_get_api_v3_simple_price`. The gateway request was
`362c623f-599e-472f-8c8b-6e2c79dd9ea9`. Tool cost was 0. The total run cost
2613 microUSD. This workspace did not repeat that call. The result reported
from that session was bitcoin at 74940 USD.

The catalog draft pin is the `main` commit that contains `plugins/openany`
version `0.2.1`, including `pyproject.toml`. `docs/hermes-catalog-draft.yaml`
records that full SHA. The commit that writes the pin does not change
`plugins/openany`.

## Streamable HTTP dependency

Hermes 0.23.0 skips MCP discovery when the `mcp` package is not importable.
The only log line is debug text: `mcp package not installed -- MCP tool support disabled`.
A stock install does not put `mcp` in `$HERMES_HOME/installs/.../venv`.
`hermes pm install --extra mcp` is a different command and can fail when
Node, npm, or ripgrep are absent.

`plugins/openany/pyproject.toml` declares `mcp==2.3.0`, the same pin as the
Hermes 0.23.0 `mcp` extra. Enabling the plugin prepares that package in the
Hermes dependency environment. No separate pip install is required. Accept
the enable prompt, or pass `--yes-deps` with `--enable`, then start a new
Hermes process.

Clean check on 2026-11-09, fresh `HERMES_HOME`, Hermes v0.23.0, install of
this plugin with `--yes-deps --enable`:

- Install exit 0. Hermes listed the dependency `mcp==2.3.0` and prepared it.
- The selected environment venv imported `mcp` 2.3.0. The PM runtime
  interpreter did not, which is expected.
- A new process using that environment connected to
  `https://mcp.openany.si/mcp-lite` and registered 3 tools:
  `get_api`, `run_api`, `search_apis`.
- The check used a throwaway profile value for `OPENANY_API_KEY` and did not
  record it. No paid `tools/call` was made.

Commands against this lite tree, Hermes v0.23.0 (`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`), Python 3.14.10, `mcp==2.3.0`:

- `python3 scripts/test_hermes_plugin.py`: 5 tests, OK
- `python3 scripts/test_public_docs.py`: 5 tests, OK
- `hermes plugins validate /workspace/plugins/openany --install-deps`: exit 0, `Validation passed.`, JSON `"ok": true`, `"warnings": []`, security scan `safe`
- `hermes plugins doctor /workspace/plugins/openany --ci`: exit 0, `manifest: openany 0.2.0`, `registrations: 0 tool(s), 0 hook(s)`

Install from the git checkout of this lite commit is recorded with the catalog pin.

`hermes plugins install "file:///workspace#plugins/openany" --no-enable --yes-deps` into a throwaway `HERMES_HOME` exited 0. Hermes warned that `file://` is an unreviewed local source. The installed copy was version `0.2.0`, URL `https://mcp.openany.si/mcp-lite`, header placeholder `Bearer ${OPENANY_API_KEY}`, trust `untrusted`. `hermes plugins enable openany` exited 0. With `OPENANY_API_KEY` unset, the connect probe raised `missing_credentials`: `MCP server 'openany': ${OPENANY_API_KEY} in url/headers is not set in this profile's .env or secret source`. No request was sent. `hermes plugins disable openany` and `hermes plugins remove openany` both exited 0. The installed directory was removed, and no file in that home contained `OPENANY_API_KEY`.

## Earlier package record

Catalog pin before the lite default:
`b9bb897d6b0e6ce62da8226030854d92821927c1`.

The first package commit was `4b607c41973613070267654cba125be0d8219eb3`.
Those commits point the plugin at `https://mcp.openany.si/mcp`.

This record separates static checks, public discovery, and authenticated
execution. Public `tools/list` is not an authenticated `tools/call`. No
OpenAny API key was available. No `tools/call` was sent. No production
service, account, balance, or release was changed.

## Package

Portable Agent Plugins v1 at `plugins/openany`:

- `plugin.json` name `openany`, version `0.1.0`
- `mcp.json` server `openany`, `streamable-http`, `https://mcp.openany.si/mcp`
- header placeholder `Authorization: Bearer ${OPENANY_API_KEY}`
- Hermes extension: secret `OPENANY_API_KEY`, server trust `untrusted`
- skill `skills/openany/SKILL.md`

`examples/hermes.config.yaml` is still a manual `mcp_servers` example. A
same-name `openany` entry there replaces the plugin server and its trust
setting.

The package does not state a model, API, or tool count. The figures below are
observations from this run.

## Environment

| Item | Value |
| --- | --- |
| Hermes | Agent v0.23.0 (2026.9.24), git `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19` (tag `v0.23.0`) |
| Python | 3.14.10 |
| Install | `uv sync --frozen` from that tag, then `uv sync --frozen --extra mcp` |
| MCP client | `mcp==2.3.0` from the `mcp` extra |

`uv sync --frozen --no-dev` does not install Streamable HTTP support.
`hermes mcp test` then failed with `mcp.client.streamable_http is not available`.
After the `mcp` extra, the same command connected. `hermes plugins validate`
and `hermes plugins doctor` had already passed before that extra was installed.

## Static checks

`python3 scripts/test_hermes_plugin.py`: 5 tests, OK.

`python3 scripts/test_public_docs.py`: 5 tests, OK.

`hermes plugins validate /workspace/plugins/openany --install-deps`: exit 0.

```text
✓ portable manifest, plugin.json parses (Agent Plugins v1)
✓ manifest fields, name present
✓ config schema, shape valid
✓ server trust: openany, untrusted (Hermes asks before every write-capable tool call)
✓ security scan, safe
◆ Isolation, portable plugin (no Python): runs anywhere
Validation passed.
```

`--json` reported `"ok": true` and `"warnings": []`.

`hermes plugins doctor /workspace/plugins/openany --ci`: exit 0.

```text
Plugin Doctor: /workspace/plugins/openany
  manifest: openany 0.1.0 (standalone)
  OK: runtime discovery, manifest parsing, import, and registration passed
  registrations: 0 tool(s), 0 hook(s)
```

Zero registered tools is expected. Tools come from the remote server at
connect time, not from `register()`.

## Install, enable, disable, remove

Throwaway `HERMES_HOME=/tmp/hermes-home-install`. `OPENANY_API_KEY` was unset.
No profile `.env` was written.

```text
hermes plugins install "file:///workspace#plugins/openany" --no-enable --yes-deps
```

Exit 0. Hermes warned that the source is a custom unreviewed catalog source
and that `file://` is a local URL scheme. Installed copy:

- `/tmp/hermes-home-install/plugins/openany`
- files: `plugin.json`, `mcp.json`, `README.md`, `skills/openany/SKILL.md`
- header remained the placeholder `Bearer ${OPENANY_API_KEY}`

`hermes plugins list --plain --no-bundled` before enable showed
`not enabled` in the full table, then:

```text
hermes plugins enable openany
✓ Plugin openany enabled.
hermes plugins show openany
openany v0.1.0
Status: enabled
Source: user
Key: openany
Runs in: Hermes process, portable plugin (no Python): runs anywhere
```

The full list view, before enable, appended `provenance: drift` on this
`file://` subdirectory install. `plugins show` after enable did not repeat
that word. A catalog install checks out a pinned SHA; this local install is
not that path.

```text
hermes plugins disable openany
⊘ Plugin openany disabled.
hermes plugins remove openany
✗ Plugin openany removed from /tmp/hermes-home-install/plugins
```

The directory was gone. `hermes plugins list --plain --no-bundled` printed no
user plugins. No file under that `HERMES_HOME` contained `OPENANY_API_KEY`.

## Enabled plugin without a key

Before removal, with the plugin enabled, Hermes loaded the portable server
through its MCP config merge:

- server present: `openany`
- url: `https://mcp.openany.si/mcp`
- trust: `untrusted`
- `strict_redirect_headers`: set
- Authorization still contained the unresolved placeholder

Hermes also logged:

```text
Plugin openany config: plugins.entries.openany.settings.api_key is required by the plugin's config_schema but is not set
```

The connect probe raised:

```text
ValueError
failure_class: missing_credentials
MCP server 'openany': ${OPENANY_API_KEY} in url/headers is not set in this profile's .env or secret source
```

No request was sent. This is not tool discovery and not an authenticated call.

## Public discovery through Hermes

Separate `HERMES_HOME=/tmp/hermes-home-public`, no Authorization header, no
plugin. The hand-written `config.yaml` had no `_config_version`, so Hermes
logged a config-migration warning. That warning is from the throwaway home,
not from the plugin.

`hermes mcp test openany_lite_public` against `https://mcp.openany.si/mcp-lite`:

```text
Transport: HTTP → https://mcp.openany.si/mcp-lite
Auth: none
✓ Connected (2959ms)
✓ Tools discovered: 3
get_api
run_api
search_apis
```

Exit 0. This server is not the one packaged in `mcp.json`.

`hermes mcp test openany_public` against `https://mcp.openany.si/mcp`:

```text
Transport: HTTP → https://mcp.openany.si/mcp
Auth: none
✓ Connected (16674ms)
✓ Tools discovered: 2500
```

Exit 0. Hermes `agent.log` then recorded:

```text
WARNING tools.mcp_tool: MCP server 'openany_public': tools pagination exceeded 50 pages; truncating at 2500 items
```

Hermes 0.23.0 `_MCP_LIST_MAX_PAGES` is 50. An independent unauthenticated
`tools/list` (no Authorization, no `tools/call`) used the same 50-tool pages.
Page 50 still returned `nextCursor`. Page 51 and page 52 each returned 50
more tools and another cursor. The probe stopped there. 2500 is Hermes's page
cap for this run, not a complete catalog size and not a product count.

Names actually returned:

- `/mcp` page 1, server order: `accurlex_post_law_search_deep_accurlex_law_search`, `accurlex_post_law_search_expert_accurlex_law_search`, `accurlex_post_qa_deep_accurlex_legal_qa`
- Hermes display order began: `aihot_get_api_v1_agent_codex_resets`, `aihot_get_api_v1_agent_daily_date`, `aihot_get_api_v1_agent_hot`
- `/mcp-lite`: `get_api`, `run_api`, `search_apis`

On the first 2500 `/mcp` definitions, none had an annotations object or
`readOnlyHint`. Pages after 50 were not checked for annotations. Those 2500
names did not include a general model-invocation tool (`openai`, `anthropic`,
or `completion` in the name). Later pages were not scanned. This package does
not call `https://api.openany.si/v1`.

`initialize` on both URLs returned HTTP 200, protocol `2025-11-25`. Server
info was `OpenAny` 0.1.0 on `/mcp` and `OpenAny Lite` 0.1.0 on `/mcp-lite`.

## Authenticated call

Not run. Blocked.

| Need | Detail |
| --- | --- |
| Secret name | `OPENANY_API_KEY` |
| Where | Hermes profile secret for this plugin (`config_schema.api_key`, env `OPENANY_API_KEY`). Not a file in the repository. |
| Why it blocks discovery inside the plugin | Hermes will not open the packaged server while the header placeholder is unresolved. |
| Why it blocks one real `tools/call` | The gateway expects `Authorization: Bearer <OPENANY_API_KEY>` for execution. |
| Price | Not measured. No `tools/call` was made, so this run cannot quote a USD amount or a safe budget. |
| Approval still required | Resource execution is pay-per-call on the caller's OpenAny balance. `trust: untrusted` asks before tools that lack `readOnlyHint: true`. The first 2500 `/mcp` tools had no such hint. |

Existing `docs/mcp.md` describes `/mcp-lite` `search_apis` and `get_api` as
catalog reads and `run_api` as able to charge. This run did not re-check that
with `tools/call`.

## Deadline check

The attached prep note says 2026-11-11 17:00 America/New_York equals
2026-11-12 06:00 UTC+8. US daylight saving time in 2026 ends on Sunday
November 1, so November 11 is EST (UTC-5). 17:00 EST is 22:00 UTC, which is
06:00 on November 12 at UTC+8. The arithmetic matches. This run did not
re-read the X announcement.

## Not done

- No pull request, including to this repository or to NousResearch/hermes-agent.
- The catalog draft in `docs/hermes-catalog-draft.yaml` was not submitted.
- No release, Typeform, X post, Discord message, or production deploy.
- This workspace did not send an authenticated `tools/call`. The owner-run CoinGecko result is recorded in the follow-up section above.
