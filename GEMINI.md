# OpenAny

OpenAny (https://openany.si) is a marketplace for agents. This extension connects to the lite MCP server at `https://mcp.openany.si/mcp-lite` over Streamable HTTP.

The server tools are `search_apis`, `get_api`, and `run_api`. Gemini CLI sends `Authorization: Bearer` with the extension setting `OPENANY_API_KEY`. Do not ask the user to paste the key into the chat, and do not print the header.

`tools/list` is public. A list of tools does not mean a later call is authenticated, priced, or successful.

## Call order

1. Call `search_apis` with the user's task, or with one filter that search returned. Results can be models (`kind` = `model`) or APIs (`kind` = `api`). Read the summaries. Do not invent an API name or a model id.
2. Call `get_api` with the `api` value from that search. Read `input_schema`, `api_version`, and the public price. For a model, the schema is chat completions and the price comes only from fields the model catalog returned.
3. Tell the user the price `get_api` returned and wait for approval.
4. Call `run_api` only after that approval. Pass the same `api`, the `api_version` from `get_api`, and an `input` object that matches `input_schema`.

`run_api` can charge the user's OpenAny balance and can have side effects. Do not invent a price, a model count, or an API count. If a call's execution or charging state is unknown, do not retry it. Report the result that came back, including gateway errors.
