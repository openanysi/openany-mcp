# Reviewer note

This note is for a Cursor Marketplace reviewer. It is not a submission, and it does not say the plugin is ready to publish.

## What this plugin is

The Cursor plugin is the repository root. `.cursor-plugin/plugin.json` names it `openany` (lowercase). `mcpServers` is `.mcp.json`. The Cursor plugin reference documents that field as a custom path that overrides default `mcp.json` discovery. This repository does not also ship a root `mcp.json`. It does not claim that Cursor prefers `.mcp.json` whenever both names exist.

`plugins/openany/` is a separate Hermes package. It is not this Cursor plugin.

The usage skill is `skills/openany/SKILL.md`. The manifest does not set `skills`, so Cursor's default skill discovery applies.

## Variable

`OPENANY_API_KEY` is declared under `variables` and is not in `required`. `.mcp.json` contains `Authorization: Bearer ${OPENANY_API_KEY}` and no key.

A manual Cursor MCP config uses a different syntax, `${env:OPENANY_API_KEY}`, documented on the MCP page. A signed-in Cursor 3.31.15 session connected with this variable empty and unsaved. That session did not show whether Cursor expands `${OPENANY_API_KEY}` or `${env:OPENANY_API_KEY}` when a value is saved. A saved key was not tested.

Saving the key does not approve a charge. The skill tells the agent to get an explicit user approval before `run_api`. That is guidance. The plugin does not add its own enforcement layer.

## Price disclosure

The plugin is MIT licensed. Installing it and using the plugin files is free. OpenAny models, APIs, and other listings are a separate service. A listing may charge separately under its own current terms. `run_api` is the tool that can start such a call. This note does not hide that charge.

[Cursor Marketplace Publisher Terms §3.1](https://cursor.com/marketplace-publisher-terms) says the Marketplace is free for publishers, that a published plugin is made available to users at no cost, and that the publisher will not charge users any fees, directly or indirectly, for access to or use of a plugin through the Marketplace. The terms we read do not say whether a separate external service, reached from the plugin, may charge for its own models or API calls. Please decide whether this split is allowed. This request does not treat the split as exempt or approved.

## What was checked without Cursor

On 2026-11-23, a direct Streamable HTTP client called `https://mcp.openany.si` with no API key:

- `initialize` returned server `OpenAny` `0.1.0`, protocol `2025-11-25`.
- `tools/list` returned `get_api`, `openany_overview`, `run_api`, and `search_apis` in one page, with no `nextCursor`.
- The same list returned with `Authorization: Bearer not-a-real-key`. That fake value was not used as a credential for a business call. `tools/list` is public in this check, so a rejected-looking token on list is not by itself an authentication bug.
- `openany_overview`, `search_apis` (`query` `weather`, `kind` `api`, `limit` 1), and `get_api` for the returned `api` completed without `run_api`. `get_api` and `search_apis` and `openany_overview` were marked `readOnlyHint: true`. `run_api` was marked `readOnlyHint: false`. `get_api` included `input_schema`, `pricing`, and `price_summary`.

No `run_api` call was made. No real key was created or read. The literal placeholder `${OPENANY_API_KEY}` was not sent.

Those calls are a protocol check. They are not the Cursor GUI test below.

## Cursor client, 2026-11-23

The full record is [client-test-2026-11-23.md](client-test-2026-11-23.md). This is not a full end-to-end test.

Passed in Cursor 3.31.15 on Debian 13 x86_64, signed in, using the local package at commit `471af31347bde22eff348e9d71e619d1174777d1`:

- Customize showed the Openany plugin at version 0.1.1 and the openany skill.
- `OPENANY_API_KEY` was shown as Optional, left empty, and was not entered or saved.
- The MCP server connected and listed `get_api`, `openany_overview`, `search_apis`, and `run_api`.
- Disable, then enable, then Reload Window returned the server to Connected.

The version string came from `cursor --version` on the official deb package. The GUI About box was not checked.

Not tested, and not treated as passed:

- A model chat.
- Invoking any tool from the GUI. No debug entry that calls a tool without a model was found.
- `run_api`.
- A valid key, a rejected key, and placeholder expansion after a key is saved.
- Uninstall.
- A full quit and restart. Reload Window is not that check.
- A paid call.
- Coexistence with a hand-written `~/.cursor/mcp.json`.

The OS keyring said it could not recognize the desktop environment. The prompt was closed without choosing a weaker encryption setting. The client continued to work after that. This is an environment note, not a plugin pass or failure.

## Marketplace name

On 2026-11-23, `https://cursor.com/marketplace/openany` displayed "Marketplace Plugin Not Found". A fetch of `https://cursor.com/marketplace?q=openany` did not contain the string `openany`. A web search for `openany` on `cursor.com/marketplace` did not show an OpenAny plugin page. A guessed slug that says not found is not a full registry export. Please search the marketplace admin index before treating the name as unused.

## Publish gate

Still not ready to publish. The local client check above is not a full end-to-end test, and the fee question in this note is not an approval.

Done:

- Local install of commit `471af31347bde22eff348e9d71e619d1174777d1` in Cursor 3.31.15.
- Customize recognized the plugin and skill.
- MCP connected with `OPENANY_API_KEY` empty and listed the four tools.
- Disable, enable, and Reload Window returned the server to Connected.

Still closed:

- Not submitted at https://cursor.com/marketplace/publish
- Placeholder expansion with a saved key was not observed
- Valid-key and rejected-key behavior was not tested
- `run_api` was not called
- Uninstall, a full quit and restart, and coexistence with a hand-written `~/.cursor/mcp.json` were not tested
- A paid call was not made
- Repository topic and marketplace settings were not changed
- Publisher Terms §3.1 has not been answered for the separate OpenAny service charge
