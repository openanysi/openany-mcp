---
name: openany
description: Call tools from the connected OpenAny marketplace MCP server. Calls can spend the user's OpenAny prepaid balance. Use only tools from the current tools/list, review the live price, and get approval before a paid call. A tools/list result is not an authenticated execution.
---

# OpenAny

OpenAny is a marketplace for agents. Use this skill only when the OpenAny MCP server from this plugin is connected in the current Hermes session.

## Before a call

- The packaged server is remote Streamable HTTP at `https://mcp.openany.si/mcp`.
- Hermes sends `Authorization: Bearer` with the profile secret `OPENANY_API_KEY`. Do not ask the user to paste the key into the chat, and do not print the header or the profile environment.
- `tools/list` can succeed without proving that a later `tools/call` is authenticated, priced, or successful.
- This package does not declare per-tool prices. Do not invent a price, a model count, or an API count.
- Call only tool names present in the current `tools/list`, with arguments that match that tool's current schema.
- Hermes may show a shortened model-facing name when `mcp__openany__<tool>` exceeds 64 characters. Dispatch still uses the server's original tool name.
- The server is `trust: untrusted`. A tool without `readOnlyHint: true` needs an approval before it runs, and the call fails closed when nobody can answer. Do not bypass that approval.

## After a call

- Report the result that was actually returned, including gateway errors.
- HTTP success is not business success and is not proof of the charge.
- Do not retry a call whose execution or charging state is unknown.

## Other endpoint

`https://mcp.openany.si/mcp-lite` is a different public server. It is not connected by this package. Do not call `search_apis`, `get_api`, or `run_api` unless the user has pointed Hermes at that endpoint.
