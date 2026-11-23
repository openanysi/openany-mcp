---
name: openany
description: The B2A (business to agent) marketplace. One MCP endpoint for 4,400+ APIs (social, SEO, crypto, company, finance, legal) and 168+ AI models (GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo, Kling). Browse free; pay per call. Use when the user wants an OpenAny model or API.
---

# OpenAny

OpenAny is the B2A (business to agent) marketplace. Use this skill when the user wants a model or an API from OpenAny.

## Server

The pi-openany package registers a remote Streamable HTTP server named `openany` at `https://mcp.openany.si`. Its tools are `openany_overview`, `search_apis`, `get_api`, and `run_api`. `/mcp-lite` is the same server. Do not use `https://mcp.openany.si/mcp` unless a different server is connected. That URL lists one tool per API and does not include models.

`OPENANY_API_KEY` is optional. When it is unset or blank, the extension does not send an Authorization header, and browsing, search, and pricing still work. When it is set, the extension sends `Authorization: Bearer` plus that value. Only `run_api` needs the key. Do not ask the user to paste the key into the chat, and do not print the header or the environment.

A same-named server in `~/.pi/agent/mcp.json` or `.pi/mcp.json` replaces this registration. The manual example is `examples/pi.mcp.json` in the OpenAny repository.

## Call order

1. When the user asks what is available, call `openany_overview` first. Present data APIs by category and AI models by modality, with the counts it returns. Do not invent a model count or an API count.
2. `search_apis` with the user's task. Results include models (`kind` = `model`) and APIs (`kind` = `api`). Do not invent an API name or a model id.
3. `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the public price before execution.
4. `run_api` only after the user approves that call. Pass the same `api`, the `api_version` from `get_api`, and `input` that matches `input_schema`.

`run_api` can spend the user's OpenAny balance and can have side effects. Do not invent a price. Do not retry a call whose execution or charging state is unknown. Report the result that was returned. HTTP success is not proof of the charge.

Invoke this skill directly with `/skill:openany`.
