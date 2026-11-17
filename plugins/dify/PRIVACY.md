# Privacy

This plugin does not open an account, does not store an API key, and does not write a database of its own. Dify stores the optional provider secret in the workspace credential store. The plugin reads that secret only while handling a `run_api` invocation.

## What is sent

The plugin sends MCP JSON to `https://mcp.openany.si` over HTTPS.

| Action | Data sent | Authorization header |
| --- | --- | --- |
| `openany_overview`, `search_apis`, `get_api` | The tool name and the arguments the user or model supplied | Absent |
| `run_api` | `api`, `api_version`, `input`, and optional `output` | `Bearer` plus the provider secret |

`input` is the business payload from `get_api`'s schema. It can contain whatever the selected API asks for, including queries about companies, people, markets, or other subjects the user chooses to look up. The plugin does not add the workspace user's name, email, or the API key to that object.

A request that fails the local key or argument check is not sent.

## Where it goes

OpenAny (Alephant AI LLC) receives the MCP request. For `run_api`, OpenAny routes the call to the selected API or model provider. Those providers receive the inputs needed to perform the operation. Their retention rules are their own. The public OpenAny policy is https://openany.si/privacy/ (updated November 3, 2026 on the page fetched while writing this plugin). That page says OpenAny processes API request and response content in transit and does not persist that content as stored content, and that account, payment, and usage records are a separate category. This plugin does not change that policy.

Catalog responses can include public catalog text, schemas, and prices. `run_api` responses can include the selected provider's result. Before a result or an error is returned to Dify, the plugin replaces the exact API key with `[REDACTED]` when the key is at least 8 characters.

## What this plugin does not collect

The plugin does not run website analytics, does not set a cookie, and does not send the key to a proxy or to a redirect target. Logs in this package are not written by the tool code. Dify and OpenAny may still keep operational records under their own policies.

Privacy requests about an OpenAny account go to team@openany.si, as the hosted policy states. This file is the plugin's own data-flow notice. It is not a data-processing agreement.
