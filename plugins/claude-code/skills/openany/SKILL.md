---
name: openany
description: Marketplace for AI agents. Discover models, APIs, skills, data, and services. Use when this community plugin is connected and the user wants something from OpenAny.
---

# OpenAny

OpenAny is a marketplace for AI agents. It lists models, APIs, skills, data, and services. This plugin is maintained by Alephant AI LLC. It is not an Anthropic certification and it is not a listing in Anthropic's official plugin directory.

The plugin is free to install. A model, API, or other listing may charge under its own current terms. Installing the plugin, connecting, and saving a key do not approve that charge.

## Server

The packaged server is remote Streamable HTTP at `https://mcp.openany.si`. `/mcp-lite` is the same server. Its tools are `openany_overview`, `search_apis`, `get_api`, and `run_api`. Queries and tool inputs are sent to OpenAny. They leave this machine.

Claude Code reads `Authorization: Bearer ${OPENANY_API_KEY:-}` from this plugin's `.mcp.json`. Do not put a key in this repository, a chat, a prompt, or a log. Do not ask the user to paste the key, and do not print the header or the environment.

Codex reads `bearer_token_env_var` `OPENANY_API_KEY` from the same file. Codex CLI 0.171.0 sends `Authorization: Bearer <value>` when that variable is non-empty. In that version, an unset or empty variable did not open the MCP connection. Set `OPENANY_API_KEY` in the environment that starts Codex before a session that needs the server.

`https://mcp.openany.si/mcp` is a different server. It lists one tool per API. This package connects to `https://mcp.openany.si`.

## Call order

1. When the user asks what is available, call `openany_overview` first. Present what it returns. Do not invent a model count or an API count.
2. `search_apis` with the user's task or one filter that search returned. Read the summaries. Do not invent an API name or a model id.
3. `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the price and billing unit shown for that item. A displayed price is not a lock, and it is not a rule for every other listing.
4. `run_api` only after the user approves that call. Pass that same `api`, the `api_version` from `get_api`, and `input` that matches `input_schema`.

`run_api` can charge and can have side effects. Do not invent a price. If a call's execution or charging state is unknown, do not retry it.

`tools/list` does not prove that a later `tools/call` is authenticated, priced, or successful. A tool without `readOnlyHint: true` needs approval. Do not bypass that approval.

Report the result that was actually returned, including gateway errors. HTTP success is not business success and is not proof of the charge.
