---
name: openany
description: The B2A (business to agent) marketplace. One MCP endpoint for 4,400+ APIs (social, SEO, crypto, company, finance, legal) and 168+ AI models (GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo, Kling). Browse free; pay per call.
---

# OpenAny

OpenAny is the B2A (business to agent) marketplace. Use this skill only when this plugin's MCP server is connected.

## Server

The packaged server is remote Streamable HTTP at `https://mcp.openany.si`. `/mcp-lite` is the same server. Its tools are `openany_overview`, `search_apis`, `get_api`, and `run_api`. `Authorization: Bearer <OpenAny API key>` is optional. Browsing, search, and pricing work without a key. Only `run_api` needs it. Do not call tool names from the full `/mcp` catalog unless a different server is connected.

Claude Code reads `Authorization: Bearer ${OPENANY_API_KEY:-}` from this plugin's `.mcp.json`. An unset variable expands to an empty token. This server treats `Bearer` with no token as no key, so browsing still works.

Codex reads `bearer_token_env_var` `OPENANY_API_KEY` from the same file. Codex CLI 0.171.0 sends `Authorization: Bearer <value>` when that variable is non-empty. In that version, an unset or empty variable did not open the MCP connection. Set `OPENANY_API_KEY` in the environment that starts Codex before a session that needs the server. Do not ask the user to paste the key into the chat, and do not print the header or the environment.

`https://mcp.openany.si/mcp` is a different server. It lists one tool per API. This package connects to `https://mcp.openany.si`.

## Call order

1. When the user asks what is available, call `openany_overview` first. Present data APIs by category and AI models by modality, with the counts it returns. Do not invent a model count or an API count.
2. `search_apis` with a business query or one returned filter. Results include models (`kind` = `model`) and APIs (`kind` = `api`). Read the summaries. Do not invent an API name or a model id.
3. `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the public price before execution. A model uses the chat completions input shape.
4. `run_api` only after the user approves the call. Pass that same `api`, the `api_version` from `get_api`, and `input` that matches `input_schema`.

`run_api` can spend the user's OpenAny balance and can have side effects. This package does not declare a fixed price. Do not invent a price, a model count, or an API count.

`tools/list` does not prove that a later `tools/call` is authenticated, priced, or successful. A tool without `readOnlyHint: true` needs approval. Do not bypass that approval. Do not retry a call whose execution or charging state is unknown.

Report the result that was actually returned, including gateway errors. HTTP success is not business success and is not proof of the charge.
