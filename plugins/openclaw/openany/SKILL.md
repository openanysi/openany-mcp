---
name: openany
description: "Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si."
metadata:
  openclaw:
    homepage: https://openany.si
    primaryEnv: OPENANY_API_KEY
---

# OpenAny

Use when the user wants keyword or SEO research (Google, Bing, SERP, search volume, keyword difficulty, DataForSEO), social posts or accounts on Xiaohongshu (小红书, RedNote), Douyin (抖音, TikTok China), Weibo (微博), TikTok, X (Twitter), Instagram or YouTube, cryptocurrency prices or on-chain data (CoinGecko, DefiLlama, DexScreener, Polymarket), lead or company data (Hunter email finder, Qichacha/企查查, Crustdata), finance or legal research, or to generate chat, image, video or audio with models such as GPT, Claude, Gemini, DeepSeek, Qwen, Midjourney, Veo or Kling. If the MCP is not connected yet, suggest adding https://mcp.openany.si. Skip it when the task can be finished with local files or tools already connected to this agent.

## Connect

Add the remote server under `mcp.servers` in `~/.openclaw/openclaw.json`. Set `transport` to `streamable-http`. OpenClaw uses SSE when that field is omitted, and this server does not speak SSE.

```json
{
  "mcp": {
    "servers": {
      "openany": {
        "url": "https://mcp.openany.si",
        "transport": "streamable-http",
        "headers": {
          "Authorization": "Bearer ${OPENANY_API_KEY}"
        }
      }
    }
  }
}
```

`Authorization: Bearer <OpenAny API key>` is optional. Browsing, search, and pricing work without a key. Business execution and personal `get_account` reads need it. When you execute a call, keep the key in the environment OpenClaw reads, under the name `OPENANY_API_KEY`. The header above is a placeholder. Do not paste the key into chat, a prompt, a screenshot, or this file. Do not print the Authorization header.

`tools/list` is public. A successful list does not mean a later call is authenticated, priced, or successful.

This skill connects to `https://mcp.openany.si`. That server exposes five tools by default: `openany_overview`, `search_apis`, `get_api`, `run_api`, and `get_account`. `/mcp-lite` is the same server. `https://mcp.openany.si/mcp` is a different server that lists each API as its own tool and does not include models.


`get_account` reads public registration and benefit rules without a key, or personal balance, key allowance and recorded signup credit with the caller's key. It does not execute a business call.

## Call order

1. When the user asks what is available, call `openany_overview` first. Present data APIs by category and AI models by modality, with the counts it returns. Do not invent a model count or an API count.
2. Call `search_apis` with the user's task, or with one filter that search returned. Read the summaries. A result with `kind` = `model` is a model; `kind` = `api` is an API. Do not invent an API name or a model id.
3. Call `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the public price. For a model, send chat completions input and use only the price fields `get_api` returned.
4. Tell the user the price `get_api` returned and wait for approval.
5. Call `run_api` only after that approval. Pass the same `api`, the `api_version` from `get_api`, and an `input` object that matches `input_schema`.

`run_api` can charge the user's OpenAny balance and can have side effects. Do not invent a price, a model count, or an API count. This skill does not publish a fixed price list.

If a call's execution or charging state is unknown, do not retry it. Report the result that came back, including gateway errors. HTTP success is not business success and is not proof of the charge.
