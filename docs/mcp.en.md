# OpenAny MCP

This guide is English by default. It is Chinese when `Accept-Language` prefers zh. `?lang=zh` or `?lang=en` overrides that. A request whose `Accept` includes `text/html` is HTML; otherwise the response is text/plain.

Paste `https://mcp.openany.si` into any MCP client. The transport is Streamable HTTP. `/mcp-lite` is the same lightweight server. When executing a business API, send this header on every request:

```text
Authorization: Bearer YOUR_OPENANY_KEY
```

MCP does not store a fallback business key. Authentication, permission, rate limits, and billing stay in the gateway. Use HTTPS. Do not put a real key in a prompt, a business parameter, or a public config repository.

## API key

`openany_overview`, `search_apis`, and `get_api` read the catalog. They do not need a key and they do not call the paid gateway. `run_api`, and the business tools on `/mcp`, need the caller's own OpenAny API key.

When the request has no credential at all, the server rejects it locally and does not call the gateway. The error code is `openany_mcp_credentials_missing` and `execution_state` is `not_started`. Create a key at https://openany.si/dashboard?page=api-keys, then set `Authorization: Bearer <key>` in the MCP client's secure secret or connector settings. Do not ask the user to paste the key into the chat or prompt. A non-empty `X-Api-Key` or `Xi-Api-Key` is also accepted.

When a key is present and the gateway rejects it, the gateway code, HTTP status, and request_id stay as returned. A live invalid key is `openany_key_invalid`. The same guidance also covers `openany_authentication_invalid`, `openany_key_expired`, `openany_key_revoked`, `openany_authentication_expired`, and `openany_authentication_revoked`. The key is invalid, expired, or revoked. Replace it in the client's secure settings. Do not send a new key into the chat. Other gateway errors keep the generic message.

This server has no OAuth. OpenAny also has no MCP URL-elicitation completion page. If the client advertises URL elicitation, a missing key still returns this text. The server does not collect a secret with form elicitation. Form mode must not be used for secrets. A website that wants URL mode needs an HTTPS page that collects the key in a masked field, gives that key to the MCP client rather than the model, puts it on later requests as Authorization, and sends `notifications/elicitation/complete` to that client. This server is stateless and does not store keys, so the completion notification alone does not attach a key to the next call.

## Choose an endpoint

| Path | Contents |
| --- | --- |
| `/` | The same lightweight server as `/mcp-lite`. MCP POST uses the tool protocol; a browser GET returns this guide |
| `/mcp-lite` | openany_overview, search_apis, get_api, and run_api, same as `/` |
| `/mcp` | One tool per API, no models, for an existing exact call |
| `/mcp/{provider_id}` | One provider's tools |
| `/docs/mcp` | This guide, public when the lightweight server is enabled |

Configure one endpoint. Connecting the full catalog as well still loads the whole catalog, and that catalog has no models.

In a client that supports remote MCP, set the server URL, Streamable HTTP, and the Authorization header. Client config formats differ. An HTTP check here does not mean every client UI version has been verified. After a new config, reconnect and confirm the four tools appear.

## Call order

When the user asks what OpenAny offers, call openany_overview first. Present data APIs by category and AI models by modality, with both counts. Then:

1. search_apis for the capability.
2. get_api for the selected API or model's input schema, price, limits, and version.
3. Confirm the current price from get_api with the user.
4. run_api once.

The `api` identifier is an existing public tool name. Do not guess a name and do not pass a URL.

```text
search_apis({"query":"backlinks summary","limit":5})
get_api({"api":"<api from the search result>"})
run_api({
  "api":"<the same api>",
  "api_version":"<api_version from get_api>",
  "input":{...fill from input_schema...}
})
```

That is a shape sketch, not JSON to execute as-is. The tool call is still a standard MCP tools/call message.

### openany_overview

No arguments. It reads the current catalog snapshot and does not call the paid gateway. It returns `api_count`, `model_count`, APIs grouped by provider and by a human category, and model ids grouped by primary modality. Categories are social/content, crypto/on-chain, company & leads, SEO/search/scraping, finance/macro, legal, e-commerce/real estate/travel, and other. Model modalities are chat/reasoning, image, video, and audio. A model with no text, image, video, or audio output is other. Counts come from the current snapshot. Do not hardcode them. When `models_available` is false, a zero model count does not mean the product has no models.

### search_apis

`query` is at most 256 characters. Optional filters are `kind`, `provider`, `category`, `data_platform`, `data_product`, and `capability`. `kind` is `api`, `model`, or `all` (default `all`). `kind=api` returns only APIs, ranked as the API-only index. Provide keywords or one filter. `kind=model` alone browses model ids. Filter values are identifiers the catalog returned. `capability` accepts an id or a key. `limit` defaults to 5 and is at most 10.

Each candidate has `kind` (`api` or `model`), api, title, provider, summary, method, path, and any category, platform, product, capability, purpose, match_reason, required_inputs_brief, and known limitations. Model candidates also have `model_id`. `modalities`, `context_length`, and `pricing_brief` appear only when the catalog provides them. Model categories always include `models` and `llm`, and `category=ai-generation` also matches models. `capability=chat` matches models whose catalog modalities or features include chat. match_reason names the fields and terms that matched. It is not a supplier quality score. Missing API categories and capabilities are not invented. The summary does not include the full schema.

A model name or vendor word (deepseek, qwen, claude, anthropic) ranks models first. A task word mixes models and related APIs. Translation, writing, code, chat, and the words translate, write, code, chat, and completion match only models whose output modality includes chat, coding, or reasoning. Input chat is only the prompt channel, and the shared `/v1/chat/completions` path is not a chat-model signal, so image and video models do not match those words and are not ranked ahead of text models. Vision words match models that accept an image and emit text. Drawing words and image generation match models that emit an image. Aliases are a limited bilingual list, not general semantic search. `models_available=false` means the model catalog has not been read successfully yet. `models_stale=true` means the latest refresh failed and results still come from the previous successful catalog.

```json
{"query":"company registration profile","limit":5}
```

ranking is keyword. It searches the tool name, description, path, parameter names, existing translations, and categories, with limited bilingual aliases and word forms. A named provider is preferred, then coverage of the core business concept in the title or capability, then latest, hot, history, and batch intent, then field relevance. A cross-reference to another API in a description is a weak signal. The same word repeated across fields is not added again. A whole English word beats a fragment, so ai does not match chain. A time intent helps choose an API and does not fill a business date. A read intent avoids obvious delete or create routes. That is only a ranking aid. Read the API. This is not general semantic search. If nothing matches, try the real tool name, the provider id, or a more specific English word.

metadata_available=false means category, translation, or example reads are temporarily unavailable. Name, description, and provider search still work. Category filters may find nothing. Pagination sends next_cursor and keeps query, every filter, and limit the same. Search again after a catalog or ranking change, or when the cursor expires.

### get_api

`api` is required. An API returns the exact `input_schema`, public `pricing`, `api_version`, description, method, public path, and API id. Parameter text may mention an optional array limit that is not supported yet.

A model's `kind` is `model`. `input_schema` is the chat completions shape: `messages` is required, and each item needs `role` and `content`. Other fields may be added. `model` may be omitted; execution then sends `model_id`. If it is set, it must equal that `model_id`. `stream: true` is rejected locally. `pricing` is the raw catalog price. `pricing_brief` appears only when the catalog has a non-zero price. prompt, completion, and input_cache_read are USD per token. `pricing_brief` multiplies those three by 1000000 and writes them per million tokens. image, request, and duration are shown as published. A displayed price is not a lock.

`input_schema` matches the direct tool's parameter schema. Keep the original top-level names and schema references for body, query, path, and header parameters. Do not regroup them. The API id is for debugging. Execution still passes api.

Added guidance:

| Field | Meaning |
| --- | --- |
| required_inputs | JSON Pointer paths, types, and existing descriptions that are declared required. scope=request is a top-level requirement. if_parent_is_object and each_present_item apply when an object or array item is present |
| examples | Public HTTP samples turned into MCP input, keeping optional business fields. Each sample must pass the current schema and the HTTP encoder. A template also needs its placeholders replaced |
| price_summary | An exact USD string, the billing unit, and the charge condition. is_quote is always false. rules mode still needs the full rules |
| response_hint | The shape, top-level fields, or array length of a public response sample. supported_media_types only names response types MCP can carry. It does not promise this API's format. Raw sample values are not returned |

examples.status is available, template_only, missing, stale, or no_safe_valid_sample. items[].kind is example (a complete public sample), template (a placeholder template), or empty_request (an empty request that validates). A template lists placeholder_paths, including `${...}` and `{{...}}` inside strings. A sample with more than 40 placeholder paths is dropped so a replacement hint is not missed. A template cannot be called as-is. template_only means there is no complete request sample. When there is no complete sample but input {} passes the schema and HTTP encoder, the server offers empty_request. An empty request does not hide a stale sample. At most three items are shown, and complete or empty samples come before templates. The server reads at most the first three public samples per API, and each request sample is at most 16KiB. It does not invent a company name, address, token, or other business value. An auth header is not turned into input. A declared parameter that looks like a credential drops that sample. validation=mcp_schema_and_http_encoding and supplier_verified=false mean MCP can accept and encode the request. They do not prove upstream data, permission, or extra business rules. The detail is still input_schema. Conditional branches and cross-field constraints are not visible from the required-field summary alone.

### run_api

`api`, `api_version`, and `input` are required. For an API, input is an object matching the input_schema just read, which is the direct tool's arguments. Pass large integers as JSON numbers.

A model call uses the same parameters. The server posts input to `POST /v1/chat/completions` on the model origin and attaches this request's caller credential. When `model` is omitted, it sends the catalog `model_id`. This can cost money. `stream: true` is not sent. After a timeout, execution and charging are unknown. Do not retry automatically.

After the parameter, version, and publication checks pass, the server encodes the request under the existing rules and sends it to the fixed gateway. The result keeps http_status, request_id, and data. A tool error has code, stage, execution_state, and next_action. A schema error also returns at most 10 details (path, keyword, message) and does not echo the submitted value. details_scope=input is relative to input. details_scope=arguments is relative to the whole tool argument object. MCP does not retry a business call.

Supported responses are finite JSON (application/json, application/*+json), UTF-8 text/plain, and text/markdown. JSON parsing stays the same. Text and Markdown stay in data, and content_type names the media type. A missing charset or utf-8 must be valid UTF-8. HTML, binary, and SSE are not passed through as text. Invalid JSON is not disguised as a text success.

A successful read adds summary: the real JSON shape, fields, or the number of array items actually returned. A string reports type=string and characters (Unicode code points). It is not a news summary or other business interpretation. business_status=not_interpreted. HTTP 200 and this summary do not mean the business operation succeeded. The server does not infer a supplier total, the actual charge, or a download link.

Optional output changes only the presentation:

```json
{
  "api":"<api from get_api>",
  "api_version":"<api_version from get_api>",
  "input":{"<parameter name>":"<parameter value>"},
  "output":{"mode":"compact","items_path":"/rows","fields":["id","name"],"max_items":5}
}
```

That sketch applies only when the real response data.rows is an array whose items have those fields.

| mode | Behavior |
| --- | --- |
| raw (default) | Keep data and add a structural summary |
| summary | Return the structural summary and HTTP or request id, and omit data. Text reports only type and length |
| compact | Select fields on the root object or selected array items. An array shows at most max_items, default 10, allowed 1 to 100 |

items_path is an RFC6901 JSON Pointer relative to the original data. An empty string selects a root array. A nested array needs an explicit path. fields is at most 20 literal keys of the current object or array item, not a nested path. Missing fields are omitted. items_path, fields, and max_items apply only to compact. items_path or max_items requires an array. The server does not search for and clip a nested array on its own. A real reduction sets presentation.truncated=true.

An output configuration that can be judged illegal is rejected before dispatch. If the path or shape is wrong only after execution, the result is openany_mcp_output_unavailable, execution_state=completed, and the original data and request_id are kept. Do not rerun to fix presentation. A string does not support compact field or array projection. The same result returns the original text with completed, and says not to call again to change the display. Output clipping does not change the business request or the charge, and it cannot rescue data that already exceeded the gateway response or MCP result limit.

## Price and billing

openany_overview, search_apis, and get_api read the catalog and do not make a paid call. run_api can charge and can have business side effects for both APIs and models. Model prices come from `GET /v1/models` and do not use the fixed, usage, or rules shapes below.

pricing.status=available means the public price data can be read. It is not a quote for the current parameters, and it does not mean the gateway will accept execution.

- fixed: price_micro_usd is an exact integer micro-USD string. Read it with unit and charge_condition.
- usage: also includes unit_quantity, quantity_scale, rounding, and usage_pointer. The total may be known only after execution.
- rules: policy includes public parameters and sale rules, including conditional prices, quantity prices, rounding, and caps.
- unavailable: the public price is temporarily unavailable. That is not free.

1 USD = 1,000,000 micro-USD. Do not add money with floating point. A displayed price is not a lock. The gateway settles the charge. If the response has no actual charge, do not invent one.

## Errors and retries

- openany_mcp_credentials_missing: the request had no key, and the gateway was not called. Create a key at https://openany.si/dashboard?page=api-keys and set `Authorization: Bearer <key>` in the client's secure settings. Do not paste the key into chat.
- openany_key_invalid, openany_key_expired, openany_key_revoked, openany_authentication_invalid, openany_authentication_expired, openany_authentication_revoked: a key was sent and the gateway rejected it. The original code, HTTP status, and request_id are kept. Replace it in the client's secure settings. Do not paste a new key into chat.
- openany_mcp_api_changed: the definition or displayed price changed. Call get_api again, then decide whether to execute.
- openany_mcp_api_unavailable: the API is not in the current published catalog. Search again.
- openany_mcp_invalid_search: fix search_apis query, filters, or limit as the error says. When a cursor is invalid, drop only cursor, keep the current query, filters, and limit, and call search_apis again from the first page. The gateway was not called.
- openany_mcp_invalid_input: the parameters do not match the schema and were rejected locally. details_scope=arguments means fix the tool arguments against the current MCP tool inputSchema and error.details. details_scope=input means fix the business input against get_api.input_schema. Do not treat search parameters as business parameters.
- openany_mcp_array_policy_missing: omit that optional array, or wait for an encoding rule. No request was sent.
- openany_mcp_busy: local business-call concurrency is full. This call did not start.
- openany_mcp_invalid_output: the output option combination is invalid and was not forwarded. A type or range error may also be invalid_input.
- openany_mcp_output_unavailable: the request already ran and cannot be presented as asked. Use the returned raw data. Do not rerun automatically.
- openany_mcp_unsupported_response / openany_mcp_invalid_response: the response format, encoding, or JSON is unsupported. HTTP status, request id, and a bounded content_type are kept. The raw error page is not echoed. HTTP 200 can still fail while MCP handles the result, so it does not prove the call did not run or was not charged.
- execution_state=unknown: the request may already have run or been charged. Do not repeat it because of a timeout. Save request_id.

api_version includes the revision, schema, public price, and forwarding rules. It only detects a known definition change. It is not a credential, an idempotency key, or a gateway price lock. This version has no general automatic deduplication or long-task recovery tool. Result size limits stay as they are. A result that is too large is not disguised as a truncated success. Adding a summary checks the output budget again.

## Deploy and check

The lightweight endpoint is on by default. Restart the service after updating the binary. Set `MCP_LITE_ENABLED=false` to turn it off. When it is off, the lightweight price and search indexes are not loaded, and the original endpoints still work. The database role stays read-only. Public price and auxiliary data share a separate refresh and a read-only pool of at most one connection. Those queries do not take a connection from the original catalog pool. Database connection quota still reserves that extra connection.

The public price query needs the public billing and charge-condition columns on agent_apis, and api_id, policy_version, parameters, and sale_policy on agent_api_pricing_policies. It does not read supplier_policy or supplier_price_micro_usd. The auxiliary query reads only existing public columns:

- agent_apis: id, revision, status, categories, data_platform, data_product, capability_id, upstream_key_id, name_translations, description_translations.
- agent_api_request_configs: api_id, samples. Only the request sample and response shape are projected. Raw response sample values are not stored in the MCP catalog.
- agent_api_capabilities: id, key, name, description, status, name_translations.
- key_market_upstream_keys: only id and status, so availability matches the public catalog. Key contents are not read.

The server does not project a whole JSON row and does not load supplier cost, internal mappings, or fixed auth config. It needs no new table, column, Redis, vector store, or extra service. A database administrator checks read-only grants for the required public columns. This implementation does not grant or migrate. If a deployment lacks an auxiliary column, table, or grant, the auxiliary information degrades and the original catalog and calls keep working. An API tied to a known disabled capability or upstream key is hidden from lightweight discovery. The gateway still decides real permission.

```sh
./scripts/run-local.sh --check-catalog
./scripts/run-local.sh
```

An unset or empty variable defaults to on. An explicit false in .env still turns it off. Delete that setting or set it to true to turn it on. Source development also needs the environment loaded first. run-local.sh runs bin/openany-mcp, so rebuild after changing code.

In /readyz, ready still means the original catalog. When lite is enabled it also shows lite_ready, pricing_available, and metadata_available. When the model catalog is enabled it also shows models_available, models_stale, and models_published. A failed price, auxiliary, or model read does not fail the original catalog. A failed model read keeps the last success and sets models_stale. get_api returns price unavailable or says no usable example. search_apis and get_api also report metadata_available. An auxiliary refresh does not change the original catalog hash. An example-only update refreshes display and does not invalidate an old api_version. The model catalog is not part of the `/mcp` catalog hash. Until lightweight preparation finishes, MCP requests to `/` and /mcp-lite return 503. A browser GET of `/` and /docs/mcp does not wait for catalog readiness.

The model catalog is read from `https://api.openany.si/v1/models` by default, with no API key. `OPENANY_MODELS_BASE_URL` can name another origin, or the same origin with a `/v1` path. The process must be able to reach that origin. Model execution goes to `/v1/chat/completions` on the same origin and uses the caller's own key. The service configuration does not store a model-call key.

Production must set MCP_PUBLIC_HOSTS. When a browser sends Origin, set MCP_ALLOWED_ORIGINS. The reverse proxy should allow `/`, /mcp-lite, and /docs/mcp, forward the caller credential, and not retry business calls. An explicit false already in the environment still overrides the default on.

`/`, /mcp-lite, and the full catalog share the process and the business concurrency budget. For stronger isolation, run another copy of the same image and route `/` and /mcp-lite to it.
