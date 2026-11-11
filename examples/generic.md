<!-- CONFIRMED authentication header: Authorization: Bearer <OPENANY_API_KEY>. The header is optional. -->
# Generic remote MCP client

- Transport: **Streamable HTTP**, not legacy SSE.
- URL: https://mcp.openany.si
- Server: the lite server, with `openany_overview`, `search_apis`, `get_api`, and `run_api`.
- Header (confirmed, optional): `Authorization: Bearer <OPENANY_API_KEY>`
- Only `run_api` needs the key. Browsing, search, and pricing work without it.
- Key source: the client's private `OPENANY_API_KEY` environment variable;
  `<OPENANY_API_KEY>` above is a placeholder, not a literal header value.
- Agent one-liner: Read https://openany.si/AGENT.md and connect OpenAny

`tools/list` is public. `run_api` requires an OpenAny API key. No OAuth is
implemented. Validate a specific task with an explicitly authorized call before
reporting its execution as successful.
Do not paste keys into chat or authorize spending merely by connecting a client.
