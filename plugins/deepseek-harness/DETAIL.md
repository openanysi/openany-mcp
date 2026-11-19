# OpenAny

The B2A (business to agent) marketplace for DeepSeek Harness.

This plugin registers `https://mcp.openany.si` through the harness MCP manager. The server lists `openany_overview`, `search_apis`, `get_api`, and `run_api`.

Browsing, schema, and pricing work with no API key. `run_api` needs `OPENANY_API_KEY` in the harness credential store and can charge per call. Create a key at https://openany.si/dashboard?page=api-keys. The plugin stores the header as `Bearer ${credential:OPENANY_API_KEY}` and does not put the key in its settings.

Install with `dsh plugin --profile web add` of this repository. The plugin activates on Web and Desktop profiles from DeepSeek Harness 0.2.2-rc.2 up to, and not including, 0.3.0. Headless, sdk, and acp profiles do not mount the MCP manager, so the row stays pending there.

Remove it with `dsh plugin --profile web remove dsh-openany`. That withdraws the server. It leaves the saved API key in place until you delete the `OPENANY_API_KEY` credential.
