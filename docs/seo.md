# Google Keyword Checker

这个词该不该做

写下一篇之前先停一下：这个词到底有没有人在搜？

This is the standalone OpenAny SEO MCP. A client that installs only this server can finish the SEO job. Data comes from DataForSEO. The server does not claim Ahrefs, Semrush, backlinks, Domain Rating, Serper, or Tavily.

| | |
| --- | --- |
| Registry title | Google Keyword Checker |
| Registry name | `si.openany/seo` |
| Intended endpoint | https://seo.openany.si |
| Candidate descriptor | [`seo/server.json`](../seo/server.json) |
| Root marketplace | https://mcp.openany.si |

`seo/server.json` is not published by this repository. Do not run `mcp-publisher publish` for it here. Do not create the `seo.openany.si` DNS record or change the Cloudflare zone from this tree. The root process at https://mcp.openany.si stays the default when `MCP_SERVER` is unset.

OpenAny is the B2A (business to agent) marketplace, with 4,400+ APIs and 168+ AI models. This server is the SEO slice of that marketplace. Anything outside keyword research still belongs on the root MCP.

## Tools

| Tool | Question it answers | DataForSEO route |
| --- | --- | --- |
| `keyword_research` | Is this keyword worth writing? Volume, and on Google, keyword difficulty. | Google: Labs Keyword Overview, which returns search volume and keyword difficulty together. `include_trend` adds Historical Keyword Data for those same keywords. That history is not new-keyword discovery. Bing: Bing search volume only. v1 has no Bing keyword difficulty. |
| `keyword_ideas` | Which terms should this article or landing page use? | `ideas` (default): Google Keyword Ideas from seeds. `related`: Google related searches for the first seed. Site alone: Keywords For Site, which is terms relevant to that site, not its current rankings. |
| `ranked_keywords` | Which keywords does this domain or page currently rank for? | Google Ranked Keywords. `historical_serp_mode` defaults to `live`. `positions` 10 means `rank_group <= 10`. Google only. |
| `serp_overview` | Who is on page 1? | Google or Bing organic SERP, regular, depth 10. |

Highest-volume related terms are `keyword_ideas`. Who is on a SERP is `serp_overview`. A domain's own current rankings are `ranked_keywords`.

Each tool description carries Use when / Do NOT, including the boundary against social listening and against backlink or Domain Rating claims.

## Price before a paid call

The first call returns the current published tariff and a `price_version`. `is_quote` is false. No DataForSEO call is made.

Show that price to the user. Call the same tool again with the same arguments, `confirm: true`, and that `price_version` only after they confirm. A displayed price is not a price lock. The gateway computes the charge from the published rules, including a per-result amount where the catalog has one.

Execution needs the caller's OpenAny API key on the MCP connection:

`Authorization: Bearer <OPENANY_API_KEY>`

Create the key at https://openany.si/dashboard?page=api-keys and store it in the client secret settings. Do not paste it into chat. A missing key is rejected locally. If `execution_state` is `unknown`, do not retry automatically. A later call in the same confirmed request is not started when an earlier one is unknown.

When location or language is omitted on a route that requires one, the price response shows the default: `location_code` 2840 (United States) and `language_code` en.

## Run

Same image and binary as the root MCP. Set `MCP_SERVER=seo` on the process that should become the SEO host. Leave it unset on the process that serves https://mcp.openany.si.

```sh
MCP_SERVER=seo \
POSTGRES_DATABASE_URL="postgres://..." \
OPENANY_GATEWAY_BASE_URL="https://gateway.example" \
MCP_PUBLIC_HOSTS="seo.openany.si" \
MCP_LISTEN_ADDR="0.0.0.0:3000" \
./openany-mcp
```

Use the same `POSTGRES_DATABASE_URL` and `OPENANY_GATEWAY_BASE_URL` as the root process. The gateway origin above is a placeholder, not a production address.

The process reads the same catalog database so it can price and dispatch the DataForSEO routes. It does not expose `openany_overview`, `search_apis`, `get_api`, or `run_api`.

Local process, loopback only:

```sh
MCP_SERVER=seo MCP_LISTEN_ADDR="127.0.0.1:3334" \
POSTGRES_DATABASE_URL="postgres://..." \
OPENANY_GATEWAY_BASE_URL="https://gateway.example" \
./openany-mcp
```

Streamable HTTP is `http://127.0.0.1:3334/` and `http://127.0.0.1:3334/mcp`. A browser GET of `/` returns the human guide. `GET /healthz` and `GET /readyz` are the process checks. `GET /server-card` returns the SEO card.

When the host should be public, point `seo.openany.si` at this process and set `MCP_PUBLIC_HOSTS=seo.openany.si`. That DNS step is separate from this repository.

## Client

[`examples/seo.mcp.json`](../examples/seo.mcp.json) points a client at the intended endpoint. The header is optional until a confirmed call. `${OPENANY_API_KEY}` is an environment placeholder, not a key.

```json
{
  "mcpServers": {
    "openany-seo": {
      "url": "https://seo.openany.si",
      "headers": {
        "Authorization": "Bearer ${OPENANY_API_KEY}"
      }
    }
  }
}
```

For a local process, use `http://127.0.0.1:3334/` as the URL.
