# Lightweight MCP

Approved: add `/mcp-lite` with exactly search_apis, get_api and run_api; retain `/mcp` and `/mcp/{provider_id}`. Go service, caller credentials forwarded to the gateway, read-only database. No naming changes, new dependencies, Redis, gateway behavior changes or paid test calls.

## Approach

Keep the old entry points and add an independent lightweight view. Pagination/compression alone still exposes the full catalog; replacing /mcp would break existing clients. User chose the additive route. Default MCP_LITE_ENABLED=false for rollout; disabled means no extra price reads or discovery index. Enabled means public static documentation is served with the service, plus Markdown source and README links.

Discovery only covers published bindings. Search uses lexical matching (exact name before weighted title/name, description, route, parameter names), basic bilingual synonyms, optional provider, default5/max10, a bounded summary, and stateless generation/query-bound pagination. No claim of general semantic search. get_api returns exact existing input_schema, public metadata, price and a per-API content version. run_api takes api (saved public tool name), api_version and input. It uses the same validator, concurrency admission, encoder, current-call credentials, logging and gateway execution as direct calls. Unknown/stale/quarantined inputs fail locally. Version is not an authorization token or a price lock. Do not auto-retry business calls.

Core publishing/readiness never waits for optional price IO. Prices refresh on a separate schedule and a separate max-one-connection read-only pool; late results enrich the latest core snapshot, never restore an old generation. Failed auxiliary reads mark prices unavailable without poisoning core readiness. Do not expose supplier pricing, auth material, mappings or fixed headers/body. Public prices use exact strings and USD, with fixed/usage/rules shapes and charge conditions. Only public sale-rule fields are selected. Price availability is not inferred as free. Do not duplicate the gateway pricing calculator.

Core catalog hashing stays unchanged. Discovery/search has its own hash. Per-API version hashes the API revision, schema, request semantics, public metadata/prices, not timestamps or other APIs. Refreshing optional metadata must not rebuild unchanged direct views. Discovery snapshots immutable; each call pins one. Prices observed from the database are informative; gateway settlement remains authoritative.

Acceptance: lite tools/list envelope <=10KiB, exactly3 tools, no all-API enum. Search default5/max10. Direct list/schema/provider pagination and encoded gateway calls unchanged. Limits, host/origin protections and expiration remain. New traffic shares process resources; separate-instance rollout supported by the same binary/config. Tests use local substitutes; actual metadata/discovery checks do not invoke business APIs.
