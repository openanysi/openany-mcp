# Hermes plugin validation

Date: 2026-11-09. Repository branch `cursor/hermes-openany-plugin-beb6`.

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
✓ portable manifest — plugin.json parses (Agent Plugins v1)
✓ manifest fields — name present
✓ config schema — shape valid
✓ server trust: openany — untrusted (Hermes asks before every write-capable tool call)
✓ security scan — safe
◆ Isolation — portable plugin (no Python): runs anywhere
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
Runs in: Hermes process — portable plugin (no Python): runs anywhere
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
- No authenticated end-to-end tool call, and no claim that one succeeded.
