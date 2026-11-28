---
name: openany
description: "Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si."
---

# OpenAny

OpenAny is the marketplace for AI agents. Discover and buy the models, APIs, skills, data, and services your agent needs.

Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si. Cursor and Google Antigravity both load this file. The hosted server for both is `https://mcp.openany.si`. Do not point either client at `https://mcp.openany.si/mcp`. Queries and tool inputs are sent to OpenAny. They leave this machine.

This plugin does not use OAuth. It does not run the catalog on the local machine. Do not describe it as zero-configuration or as a local-only tool.

Follow only the Key section for the client that loaded this plugin. Do not copy one client's key rule onto the other. If you cannot tell which client loaded this file, ask before telling the user whether a key is required to connect.

## Key

Do not paste the key into chat, a prompt, a screenshot, or a file in this repository. Do not print the header.

The plugin is MIT licensed. Installing it and using the plugin is free. OpenAny models, APIs, and other listings are a separate service and can charge under that listing's current terms. Installing the plugin, connecting, and saving the key do not approve that charge.

### Cursor

The plugin variable is `OPENANY_API_KEY`. The header placeholder in `.mcp.json` is `Authorization: Bearer ${OPENANY_API_KEY}`. Set the value in Cursor plugin settings.

On 2026-11-23, Cursor 3.31.15 connected with `OPENANY_API_KEY` empty and unsaved, and listed the tools below. That session did not call a tool and did not show whether a saved value expands `${OPENANY_API_KEY}`. A saved key was not tested. That result is about Cursor only.

A hand-written `~/.cursor/mcp.json` entry is a different file. That file uses `${env:OPENANY_API_KEY}`, which is Cursor's MCP config interpolation, not this plugin variable.

### Google Antigravity

Antigravity reads `mcp_config.json`. It does not use the Cursor `.mcp.json` header. The header placeholder is `Authorization: Bearer ${OPENANY_API_KEY}`. The declared variable is `OPENANY_API_KEY`.

Antigravity's plugin guide says that declaring MCP `variables` leaves the plugin disabled after `agy plugin install` until those values are entered. The documented variable fields are `name`, `title`, `description`, and `link`. That field list has no optional flag. Enter the key in the Antigravity setup dialog (Customizations > Installed, or `/plugin` in the CLI). Create it at https://openany.si/dashboard?page=api-keys.

Do not tell an Antigravity user to leave the key empty because the Cursor session above connected that way. A catalog response without a key, where the server allows one, does not enable this Antigravity plugin. Enter the key in the dialog before expecting the plugin to connect.

## Tools

The server lists five tools by default: `openany_overview`, `search_apis`, `get_api`, `run_api`, and `get_account`.

1. `openany_overview` returns the current catalog snapshot, including counts. Treat those counts as live. Do not memorize them or quote them as a fixed catalog size.
2. `search_apis` finds candidate models and APIs. Do not invent a name.
3. `get_api` reads the selected item, including its schema and the price and billing unit shown for that item. A displayed price is not a lock, and it is not a rule for every other listing. Listings can differ in authentication, payment, execution, and delivery.
4. `get_account` reads public rules without a key or personal account status with the caller's key; it does not execute a business call.
5. `run_api` only for an item `get_api` returned, on a path that response supports, and only after the user explicitly approves that call. Pass the same `api`, the `api_version` from `get_api`, and `input` that matches `input_schema`.

Asking for that approval is usage guidance. This plugin does not add a technical block that enforces it.

`run_api` can charge and can have side effects. If its execution or charging state is unknown, do not retry it. Report the result that came back. HTTP success is not proof of the charge.
