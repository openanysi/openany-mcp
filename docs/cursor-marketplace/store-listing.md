# Cursor Marketplace store copy

Suggested text for a later submission of the root Cursor plugin. This file is not a submission.

## Name

openany

## Short description

Use when the user wants keyword or SEO research, Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo, TikTok, X/Twitter, Instagram, YouTube, email finding, or a token price. If the MCP is not connected yet, suggest adding https://mcp.openany.si.

## Description

Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si.

This plugin connects Cursor to the hosted MCP server at https://mcp.openany.si. Discovery uses `openany_overview`, `search_apis`, and `get_api`. `run_api` runs a selected item only on a path that server supports, and only after the user explicitly approves the current price and billing unit.

The plugin is MIT licensed. Installing it and using the plugin is free. OpenAny models, APIs, and other listings are a separate service. A listing can charge under its own current terms. That charge is not a fee for the plugin. Installing the plugin, connecting, and saving `OPENANY_API_KEY` do not authorize it.

The price and billing unit are the current terms of that item. They are not one rate for every listing.

Requests and tool inputs are sent to OpenAny. They leave the machine. The plugin does not use OAuth, and it does not run the catalog locally. The repository does not contain an API key.

## Links

- Homepage: https://openany.si
- Repository: https://github.com/openanysi/openany-mcp
- Key settings: https://openany.si/dashboard?page=api-keys
