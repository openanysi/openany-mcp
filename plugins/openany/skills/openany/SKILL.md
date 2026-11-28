---
name: openany
description: "Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si."
---

# OpenAny

Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si. Read current resources and live counts with `openany_overview`.

## Server

The packaged server is remote Streamable HTTP at `https://mcp.openany.si`. `/mcp-lite` is the same server. It exposes five tools by default: `openany_overview`, `search_apis`, `get_api`, `run_api`, and `get_account`. `Authorization: Bearer <OpenAny API key>` is optional. Browsing, search, and pricing work without a key. Business execution and personal `get_account` reads need it. Do not call tool names from the full `/mcp` catalog unless a different server is connected.

Hermes sends `Authorization: Bearer` with the profile secret `OPENANY_API_KEY`. Do not ask the user to paste the key into the chat, and do not print the header or the profile environment.

`https://mcp.openany.si/mcp` is a different server. Hermes 0.23.0 stops `tools/list` after 50 pages, so that server is not what this package connects. A provider path such as `https://mcp.openany.si/mcp/coingecko` lists only that provider.


`get_account` reads public registration and benefit rules without a key, or personal balance, key allowance and recorded signup credit with the caller's key. It does not execute a business call.

## Call order

1. When the user asks what is available, call `openany_overview` first. Present data APIs by category and AI models by modality, with the counts it returns. Do not invent a model count or an API count.
2. `search_apis` with a business query or one returned filter. Results include models (`kind` = `model`) and APIs (`kind` = `api`). Read the summaries. Do not invent an API name or a model id.
3. `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the public price before execution. A model uses the chat completions input shape.
4. `run_api` only after the user approves the call. Pass that same `api`, the `api_version` from `get_api`, and `input` that matches `input_schema`.

`run_api` can spend the user's OpenAny balance and can have side effects. This package does not declare a fixed price. Do not invent a price, a model count, or an API count.

`tools/list` does not prove that a later `tools/call` is authenticated, priced, or successful. The server is `trust: untrusted`. A tool without `readOnlyHint: true` needs approval, and the call fails closed when nobody can answer. Do not bypass that approval. Do not retry a call whose execution or charging state is unknown.

Report the result that was actually returned, including gateway errors. HTTP success is not business success and is not proof of the charge.
