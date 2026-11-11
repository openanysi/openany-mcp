# Connect to OpenAny MCP

OpenAny MCP lets agents discover models and APIs, inspect input schemas and public pricing, and execute requests with one API key. Pay per call, no monthly fee.

## Connection

| Setting | Value |
| --- | --- |
| Server URL | `https://mcp.openany.si` |
| Transport | Streamable HTTP |
| Authentication | `Authorization: Bearer <OpenAny API key>`, optional. Only `run_api` needs it |
| Website | https://openany.si/ |
| Existing connection reference | https://mcp.openany.si/docs/mcp |

Create an account and obtain your own key through the [OpenAny API key page](https://openany.si/dashboard?page=api-keys). Configure the URL and Authorization header in a client that supports remote Streamable HTTP MCP servers and custom headers. Store the key in the client's secret store or private configuration. The endpoint does not provide an OAuth login flow.

Connect to this one endpoint and confirm that these tools appear. `/mcp-lite` is the same server.

| Tool | Purpose |
| --- | --- |
| `openany_overview` | Report live API and model counts, with APIs by category and models by modality. No arguments and no paid call |
| `search_apis` | Find model and API candidates by keywords, kind, and optional provider |
| `get_api` | Read a candidate's input schema, public or catalog pricing, limitations and version |
| `run_api` | Execute the selected API, or send the selected model to chat completions, with the caller's credentials |

## Discovery example

Ask your agent:

> Use OpenAny to find an API for a backlinks summary. Show its required inputs, pricing and limitations. Stop before executing it.

The corresponding MCP tool calls are:

```json
{"name":"search_apis","arguments":{"query":"backlinks summary","limit":1}}
```

Copy an `api` identifier from the actual search response, then inspect it:

```json
{"name":"get_api","arguments":{"api":"API_FROM_SEARCH"}}
```

These are `tools/call` parameters, not standalone HTTP requests. `API_FROM_SEARCH` is a placeholder. Discovery and inspection do not execute a billable business API.

## Execute a request

Read the current `input_schema`, `pricing`, limitations and `api_version` from `get_api`. Build `input` from that schema and execute within the user's authorized task and spending limits:

```json
{
  "name": "run_api",
  "arguments": {
    "api": "API_FROM_SEARCH",
    "api_version": "VERSION_FROM_GET_API",
    "input": {}
  }
}
```

Replace both placeholders and populate `input` with all required parameters. The empty object illustrates the envelope only; it is not a valid request for every API. Read each API's live schema rather than copying a static catalog or guessing parameter names.

`run_api` can incur usage charges and business side effects. Public pricing is information about the current catalog, not a locked quote. Unavailable pricing does not mean a call is free. Gateway authentication, permissions, rate limits and billing apply to each execution.

## Handle results and errors

- Inspect the MCP result's `isError`, the gateway status, and any business status in the returned data. HTTP 200 alone does not prove business success.
- If `openany_mcp_api_changed` is returned, call `get_api` again and review the current definition and pricing before deciding whether to execute.
- Correct schema errors using the exact schema returned by `get_api`.
- If execution is `unknown` after a timeout or disconnect, retain the `request_id` when available and investigate the original call. Do not automatically repeat a call that may already have run or incurred a charge.
- Check a client's ability to configure the optional Authorization header before a paid call. Browsing, search, and pricing work without a key. This guide does not establish compatibility with every product or client version.

## Check the public handshake

This request initializes MCP without executing a business API:

```sh
curl --fail-with-body --silent --show-error --max-time 30 \
  'https://mcp.openany.si' \
  --header 'User-Agent: OpenAnyListingAudit/1.0' \
  --header 'Content-Type: application/json' \
  --header 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"openany-connection-check","version":"1.0"}}}'
```

On November 6, 2026, `https://mcp.openany.si/mcp-lite` returned `OpenAny Lite`, version `0.1.0`, and supported this protocol version. That inspection confirmed the three discovery and execution tools, keyword search, and schema/pricing lookup without credentials. The root URL now serves that same lite server. Authenticated business execution and product-specific client behavior require separate verification.
