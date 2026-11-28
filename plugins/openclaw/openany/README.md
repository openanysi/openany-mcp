# OpenAny skill for OpenClaw

This folder is an OpenClaw skill for the hosted OpenAny MCP server. It is a separate package from `plugins/openany`, which Hermes loads.

One MCP endpoint for keyword research, Xiaohongshu and Douyin posts, email finding, token prices, and AI models. Browse and price-check free; usage-based execution.

OpenAny is the B2A (business to agent) marketplace. Add one MCP endpoint, https://mcp.openany.si, and your agent can find and use data APIs and AI models.

APIs cover social and content (Xiaohongshu / 小红书 / RedNote, Douyin / 抖音 / TikTok China, Weibo / 微博, international TikTok, X/Twitter, Instagram, and YouTube), SEO, search and scraping (DataForSEO, Serper, Tavily), crypto and on-chain data (CoinGecko, DefiLlama, DexScreener, Hyperliquid, Polymarket), company and lead data (Hunter, Crustdata, QCC), finance and macro (FRED, Financial Datasets), and legal (PKULaw).

Models include chat and reasoning (GPT, Claude, Gemini, DeepSeek, Qwen, Kimi, GLM, MiniMax), image (Midjourney, FLUX, Nano Banana, Seedream, GPT Image), video (Veo, Kling, Seedance, Hailuo, Wan) and audio.

Searching and checking prices is free with no key. You only need an OpenAny API key to run a call, and you pay per call.

The skill tells an OpenClaw agent when to use that path, how to connect the lite MCP server, and the order of tool calls.

## Files

`SKILL.md` is the skill. Its front matter sets `name`, `description`, and `metadata.openclaw`. The homepage is https://openany.si. The skill names `OPENANY_API_KEY` as `primaryEnv`. The key is optional. Browsing, search, and pricing work without it. Business execution and personal `get_account` reads need it.

## Connect

Merge the server block from `SKILL.md` into `~/.openclaw/openclaw.json` under `mcp.servers`. Keep `transport` set to `streamable-http`. The header reads `OPENANY_API_KEY` from the environment. Do not put a real key in this folder.

The URL in the skill is `https://mcp.openany.si`. That server lists five tools by default: `openany_overview`, `search_apis`, `get_api`, `run_api`, and `get_account`. `/mcp-lite` is the same server. The manual example at `examples/openclaw.json` uses the same URL. The header there is optional for public discovery and account rules; personal account reads and business execution use the caller's key.

## Publish

Publishing this folder to ClawHub is a separate maintainer step after it is on the default branch. Installing the skill locally does not require that publish.
