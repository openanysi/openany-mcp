# OpenAny skill for OpenClaw

This folder is an OpenClaw skill for the hosted OpenAny MCP server. It is a separate package from `plugins/openany`, which Hermes loads.

OpenAny is the B2A (business to agent) marketplace. Add one MCP endpoint, https://mcp.openany.si, and your agent can find and use 3,200+ APIs and 168+ AI models.

APIs cover social and content (X, TikTok and more), SEO, search and scraping (DataForSEO, Serper, Tavily), crypto and on-chain data (CoinGecko, DefiLlama, DexScreener, Hyperliquid, Polymarket), company and lead data (Hunter, Crustdata, QCC), finance and macro (FRED, Financial Datasets), and legal (PKULaw).

Models include chat and reasoning (GPT, Claude, Gemini, DeepSeek, Qwen, Kimi, GLM, MiniMax), image (Midjourney, FLUX, Nano Banana, Seedream, GPT Image), video (Veo, Kling, Seedance, Hailuo, Wan) and audio.

Searching and checking prices is free with no key. You only need an OpenAny API key to run a call, and you pay per call.

The skill tells an OpenClaw agent when to use that path, how to connect the lite MCP server, and the order of tool calls.

## Files

`SKILL.md` is the skill. Its front matter sets `name`, `description`, and `metadata.openclaw`. The homepage is https://openany.si. The skill names `OPENANY_API_KEY` as `primaryEnv`. The key is optional. Browsing, search, and pricing work without it. Only `run_api` needs it.

## Connect

Merge the server block from `SKILL.md` into `~/.openclaw/openclaw.json` under `mcp.servers`. Keep `transport` set to `streamable-http`. The header reads `OPENANY_API_KEY` from the environment. Do not put a real key in this folder.

The URL in the skill is `https://mcp.openany.si`. That server lists `openany_overview`, `search_apis`, `get_api`, and `run_api`. `/mcp-lite` is the same server. The manual example at `examples/openclaw.json` uses the same URL. The header there is optional.

## Publish

Publishing this folder to ClawHub is a separate maintainer step after it is on the default branch. Installing the skill locally does not require that publish.
