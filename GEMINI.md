# OpenAny

OpenAny is the B2A (business to agent) marketplace. Add one MCP endpoint, https://mcp.openany.si, and your agent can find and use 3,200+ APIs and 168+ AI models.

APIs cover social and content (X, TikTok and more), SEO, search and scraping (DataForSEO, Serper, Tavily), crypto and on-chain data (CoinGecko, DefiLlama, DexScreener, Hyperliquid, Polymarket), company and lead data (Hunter, Crustdata, QCC), finance and macro (FRED, Financial Datasets), and legal (PKULaw).

Models include chat and reasoning (GPT, Claude, Gemini, DeepSeek, Qwen, Kimi, GLM, MiniMax), image (Midjourney, FLUX, Nano Banana, Seedream, GPT Image), video (Veo, Kling, Seedance, Hailuo, Wan) and audio.

Searching and checking prices is free with no key. You only need an OpenAny API key to run a call, and you pay per call.

This extension connects to the lite MCP server at `https://mcp.openany.si` over Streamable HTTP. `/mcp-lite` is the same server.

The server tools are `openany_overview`, `search_apis`, `get_api`, and `run_api`. `Authorization: Bearer <OpenAny API key>` is optional. Browsing, search, and pricing work without a key. Only `run_api` needs it. Gemini CLI sends `Authorization: Bearer` with the extension setting `OPENANY_API_KEY` when that setting is present. Do not ask the user to paste the key into the chat, and do not print the header.

`tools/list` is public. A list of tools does not mean a later call is authenticated, priced, or successful.

## Call order

1. When the user asks what is available, call `openany_overview` first. Present data APIs by category and AI models by modality, with the counts from that response. Do not invent a model count or an API count.
2. Call `search_apis` with the user's task, or with one filter that search returned. Results can be models (`kind` = `model`) or APIs (`kind` = `api`). Read the summaries. Do not invent an API name or a model id.
3. Call `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the public price. For a model, the schema is chat completions and the price comes only from fields the model catalog returned.
4. Tell the user the price `get_api` returned and wait for approval.
5. Call `run_api` only after that approval. Pass the same `api`, the `api_version` from `get_api`, and an `input` object that matches `input_schema`.

`run_api` can charge the user's OpenAny balance and can have side effects. Do not invent a price, a model count, or an API count. If a call's execution or charging state is unknown, do not retry it. Report the result that came back, including gateway errors.
