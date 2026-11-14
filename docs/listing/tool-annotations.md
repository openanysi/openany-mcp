# MCP tool annotations

The B2A (business to agent) marketplace. The lite server (`/` and `/mcp-lite`) advertises these hints on `tools/list`. Each per-API tool on `/mcp` uses the same execution hints as `run_api`, with `title` and `annotations.title` set to that catalog entry's title.

| Tool | title | readOnlyHint | destructiveHint | idempotentHint | openWorldHint | Justification |
| --- | --- | --- | --- | --- | --- | --- |
| `openany_overview` | OpenAny overview | true | false | true | false | Returns live catalog counts from a public snapshot; no arguments; no paid gateway or third-party call. |
| `search_apis` | Search APIs and models | true | false | true | false | Keyword search over OpenAny's own catalog metadata; returns summaries only; no paid call or write. |
| `get_api` | Get API or model details | true | false | true | false | Reads one catalog entry's schema and public price; discovery only; no gateway call. |
| `run_api` | Run an API or model call | false | false | false | true | Executes a selected API or chat-completions model call through OpenAny; can incur charges and have upstream side effects; does not itself delete resources, so not destructive, but not read-only and reaches external providers. |
