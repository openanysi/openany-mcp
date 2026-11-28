package seo

// Guide is the human page for the SEO server. The Chinese hook stays here,
// not in the English registry description.
const Guide = `# Google Keyword Checker

这个词该不该做

写下一篇之前先停一下：这个词到底有没有人在搜？

This server is the OpenAny SEO MCP. A client that installs only this server can answer four questions. Data comes from DataForSEO. It does not claim Ahrefs, Semrush, backlinks, Domain Rating, Serper, or Tavily.

Intended public endpoint: https://seo.openany.si

Registry name, for a later manual submission: si.openany/seo

This process does not change DNS. Point seo.openany.si at it only when you are ready.

## Tools

- keyword_research: Google or Bing search volume, and Google keyword difficulty. Ask: 「email finder API」在美国值得做吗？每月有多少人搜、难不难？
- keyword_ideas: related terms for an article or landing page. A site alone returns keywords relevant to that site, not its current rankings.
- ranked_keywords: keywords a domain or page currently ranks for on Google. v1 is Google only. positions 10 keeps rank_group <= 10.
- serp_overview: who is on page 1 of Google or Bing for one keyword.

Do not use this server for Xiaohongshu, Douyin, TikTok, Weibo, or a cross-platform brief. Those jobs are outside this server. The root marketplace remains https://mcp.openany.si.

## Price before a paid call

The first call returns the current published price and price_version. It does not call DataForSEO. is_quote is false. Show that price to the user. Call the same tool again with confirm true and that price_version only after they confirm. Execution needs Authorization: Bearer <OpenAny API key> from the client secret settings. Do not paste the key into chat.

If execution_state is unknown, do not retry automatically.

## Run this process

Use the same binary as the root MCP. MCP_SERVER=seo serves only the four tools. Leave MCP_SERVER unset on https://mcp.openany.si.

MCP_SERVER=seo
POSTGRES_DATABASE_URL=...
OPENANY_GATEWAY_BASE_URL=...
MCP_PUBLIC_HOSTS=seo.openany.si
MCP_LISTEN_ADDR=0.0.0.0:3000

The process still reads the OpenAny catalog so it can price and dispatch the DataForSEO routes. It does not expose search_apis, get_api, run_api, or openany_overview.

Local check without a public host:

MCP_SERVER=seo MCP_LISTEN_ADDR=127.0.0.1:3334

Then POST Streamable HTTP to http://127.0.0.1:3334/ or http://127.0.0.1:3334/mcp.

## Upstream routes

- keyword_research on Google calls DataForSEO Labs Keyword Overview, because that route returns search volume and keyword difficulty together. include_trend adds Historical Keyword Data for those same keywords.
- keyword_research on Bing calls Bing search volume. There is no Bing keyword-difficulty route in v1.
- keyword_ideas uses Google Keyword Ideas, Google related keywords for the first seed, or Keywords For Site when only a site is given.
- ranked_keywords uses Google Ranked Keywords. historical_serp_mode defaults to live.
- serp_overview uses Google or Bing organic SERP regular with depth 10.

When location or language is omitted on a route that requires one, the price response shows the default location_code 2840 (United States) and language_code en before anything is charged.
`
