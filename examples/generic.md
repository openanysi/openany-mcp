<!-- CONFIRMED authentication header: Authorization: Bearer <OPENANY_API_KEY>. -->
# Generic remote MCP client

- Transport: **Streamable HTTP**, not legacy SSE.
- URL: https://mcp.openany.si/mcp
- Header (confirmed): `Authorization: Bearer <OPENANY_API_KEY>`
- Key source: the client's private `OPENANY_API_KEY` environment variable;
  `<OPENANY_API_KEY>` above is a placeholder, not a literal header value.
- Agent one-liner: Read https://openany.si/AGENT.md and connect OpenAny

`tools/list` is public. Resource calls through `tools/call` require an OpenAny
API key. No OAuth is implemented. Validate a specific task with an explicitly
authorized call before reporting its execution as successful.
Do not paste keys into chat or authorize spending merely by connecting a client.
