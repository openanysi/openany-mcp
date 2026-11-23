# Reviewer note

This note is for a Cursor Marketplace reviewer. It is not a submission, and it does not say the plugin is ready to publish.

## What this plugin is

The Cursor plugin is the repository root. `.cursor-plugin/plugin.json` names it `openany` (lowercase). `mcpServers` is `.mcp.json`. The Cursor plugin reference documents that field as a custom path that overrides default `mcp.json` discovery. This repository does not also ship a root `mcp.json`. It does not claim that Cursor prefers `.mcp.json` whenever both names exist.

`plugins/openany/` is a separate Hermes package. It is not this Cursor plugin.

The usage skill is `skills/openany/SKILL.md`. The manifest does not set `skills`, so Cursor's default skill discovery applies.

## Variable

`OPENANY_API_KEY` is declared under `variables` and is not in `required`. `.mcp.json` contains `Authorization: Bearer ${OPENANY_API_KEY}` and no key.

The plugin reference says that placeholder is filled from Plugins, then Configure. A manual Cursor MCP config uses a different syntax, `${env:OPENANY_API_KEY}`, documented on the MCP page. This repository has not installed the plugin in Cursor, so it has not observed either substitution. Please check whether Cursor connects when the plugin value is empty. Do not treat an empty value as tested.

Saving the key does not approve a charge. The skill tells the agent to get an explicit user approval before `run_api`. That is guidance. The plugin does not add its own enforcement layer.

## What was checked without Cursor

On 2026-11-23, a direct Streamable HTTP client called `https://mcp.openany.si` with no API key:

- `initialize` returned server `OpenAny` `0.1.0`, protocol `2025-11-25`.
- `tools/list` returned `get_api`, `openany_overview`, `run_api`, and `search_apis` in one page, with no `nextCursor`.
- The same list returned with `Authorization: Bearer not-a-real-key`. That fake value was not used as a credential for a business call. `tools/list` is public in this check, so a rejected-looking token on list is not by itself an authentication bug.
- `openany_overview`, `search_apis` (`query` `weather`, `kind` `api`, `limit` 1), and `get_api` for the returned `api` completed without `run_api`. `get_api` and `search_apis` and `openany_overview` were marked `readOnlyHint: true`. `run_api` was marked `readOnlyHint: false`. `get_api` included `input_schema`, `pricing`, and `price_summary`.

No `run_api` call was made. No real key was created or read. The literal placeholder `${OPENANY_API_KEY}` was not sent.

Those calls are not a Cursor client install. Enable, disable, restart, uninstall, and coexistence with a hand-written `~/.cursor/mcp.json` were not tested. This environment has no Cursor desktop or Cursor CLI.

## Marketplace name

On 2026-11-23, `https://cursor.com/marketplace/openany` displayed "Marketplace Plugin Not Found". A fetch of `https://cursor.com/marketplace?q=openany` did not contain the string `openany`. A web search for `openany` on `cursor.com/marketplace` did not show an OpenAny plugin page. A guessed slug that says not found is not a full registry export. Please search the marketplace admin index before treating the name as free.

## Publish gate, still closed

- Not submitted at https://cursor.com/marketplace/publish
- Not installed or enabled in Cursor
- Placeholder expansion in Cursor not observed
- Valid-key behavior not tested
- `run_api` not called
- Disable, restart, uninstall, and manual-config coexistence not tested
- Repository topic and marketplace settings not changed
