# OpenAny MCP Development

For the public endpoint, client setup and authentication caveats, see the
[repository README](../README.md). The following existing operator notes are
preserved as development history, not a statement about the current deployment.

Independent Go MCP server for OpenAny Agent APIs. Reads a PostgreSQL catalog and forwards each tool call to the configured gateway with **that request's credentials**. Authentication, rate limits, billing and upstream execution remain in the gateway. A separate Python CLI assigns persistent public names.

## Local quick start

```sh
cd /Users/legend/WorkSpace/openany/openany-mcp
go build -trimpath -o bin/openany-mcp ./cmd/openany-mcp
./scripts/run-local.sh --check-catalog
./scripts/run-local.sh
```

For local development, load the configuration in the same shell and run the source:

```sh
set -a
. ./.env
set +a
go run ./cmd/openany-mcp
```

Go does not load `.env` automatically. Restart `go run` after code changes. The local runner executes `bin/openany-mcp`, so rebuild it after changing Go source.

The local installation includes `.env` (reader role) and `.env.maintenance` (naming role), both mode 600 and excluded from Git. `run-local.sh` loads `.env`. Go itself reads environment variables, not dotenv files. No gateway API key is stored in either configuration; callers supply their own.

The installed local `.env` uses port 3335 because the original smoke server occupies 3333. `.env.example` retains the configurable default 3333.

Lightweight discovery is enabled by default. Rebuild the binary and restart to use it; set `MCP_LITE_ENABLED=false` to disable it. The original full/provider URLs remain available. New Agents should connect only to `/mcp-lite` to avoid loading the full catalog. [Public connection guide](mcp.md) ships at `/docs/mcp` when enabled.

Endpoints:

- `http://127.0.0.1:3335/mcp-lite` — exactly `search_apis`, `get_api`, `run_api` when enabled.
- `http://127.0.0.1:3335/docs/mcp` — public plain-text connection guide when enabled; available before catalog readiness.

- `http://127.0.0.1:3335/mcp` — complete catalog.
- `http://127.0.0.1:3335/mcp/blockrun` — provider-specific catalog; same names.
- `/healthz` — process health; `/readyz` — catalog age and active/published/pending/quarantined/warning counts. A listening port alone does not mean the catalog is ready.

Configure a compatible MCP client for **Streamable HTTP** and attach `Authorization: Bearer YOUR_OPENANY_KEY` (or the gateway-supported API-key header). The service does not implement an OAuth login. Calls without credentials are forwarded without a fallback key and rejected by the gateway. Native clients need no Origin; browser origins must be explicitly allowed.

## New installation

1. Copy `.env.example` to a private local `.env`, replace database credentials and set the gateway origin. Use a SELECT-only database role for the service.
2. Have an operator run `migrations/001_add_public_mcp_tool_name.sql` against the intended database. It adds a nullable text column, validated format constraint and global unique index; it does not change prices, enable APIs or assign names.
3. Review API descriptions and array query serialization. Description review produces warnings with safe display text. Source-derived route and API overrides are in `config/catalog-overrides.json`; evidence and limitations are in `config/transport-evidence.md`.
4. Run `--check-catalog` to validate active metadata, including unnamed rows. This command never calls a business API or writes names.
5. Set the top-level `DATABASE_URL` in `scripts/sync_mcp_tool_names.py` to a **separate maintenance credential**, preview/apply name assignment, then re-check the catalog.

Python setup and assignment:

```sh
uv venv --python 3.12 .venv
uv pip install --python .venv/bin/python -r scripts/requirements.lock
# First edit DATABASE_URL at the top of scripts/sync_mcp_tool_names.py.
.venv/bin/python scripts/sync_mcp_tool_names.py
.venv/bin/python scripts/sync_mcp_tool_names.py --apply
```

At the top of `scripts/sync_mcp_tool_names.py`, fill in the PostgreSQL connection string:

```python
DATABASE_URL = "postgresql://openany_mcp_namer:YOUR_PASSWORD@127.0.0.1:5432/YOUR_DATABASE"
```

The committed default is empty. The CLI uses only this variable, trims surrounding whitespace and stops when empty; shell `POSTGRES_DATABASE_URL` and dotenv files do not select the maintenance database. Use a complete connection string with host, port, database and maintenance user; omitted fields follow libpq defaults. The Go service continues to use its own environment configuration. After switching databases, make sure the target has the required table/column/constraints by running the existing operator migration if needed.

Successful JSON output includes `target.host`, `target.port`, `target.database`, `target.schema` and `target.table` from the actual connection. It does not print the connection string or password. Keep a real configured connection string local when committing changes. The installed `.env.maintenance` remains a private reference for the local naming-role credential; loading it does not configure this CLI anymore.

The default is read-only preview. `--apply` assigns only NULL values in one transaction, using advisory/row locks and the unique index. Existing or manually assigned names are never changed. Names have no automatic UUID suffix. On a collision, only the new name receives a numeric alias (`_2`, `_3`, ...); the base is shortened as needed to keep the full name at most 64 ASCII characters. Disabled APIs also receive and retain names. No force-rename option exists.

Names normally look like `blockrun_get_api_v1_crypto_list` or `dataforseo_post_v3_backlinks_summary_live`. They are stored in `agent_apis.public_mcp_tool_name`; the existing `mcp_tool_name` remains the upstream tool name. API imports and updates must preserve this column when not explicitly changing it. No control-plane importer was modified by this independent project.

### Migrating names from the original UUID format

The explicit migration switch previews changes to names that exactly match the old generator for the same provider, method, path and API UUID:

```sh
# First set the script's DATABASE_URL to your maintenance connection string.
.venv/bin/python scripts/sync_mcp_tool_names.py --rename-generated
.venv/bin/python scripts/sync_mcp_tool_names.py --rename-generated --apply
```

Other existing names are reserved and preserved. A hand-picked name identical to an old generated value cannot be distinguished without historical provenance; if metadata paths have changed, the old name may not match and will be retained. Output includes `previous_tool_name` for each rename. `assigned` counts all proposed/written changes, including `renamed`; `already_named` counts non-NULL rows before changes.

Migration and NULL fills commit atomically under the same advisory lock. Row locks, expected-old-value checks and the unique index protect concurrent changes. No intermediate NULL values are visible to catalog readers. Save the preview and apply JSON reports before a migration; local reports belong under ignored `var/maintenance/`.

After intentional renaming, reconnect or re-list tools in Agents. Old tool names are removed on catalog refresh; they are not runtime aliases. The `_2`, `_3` names each identify their own API, and repeated normal runs never re-number saved names.

## Runtime behavior

- Active, named APIs are compiled from `agent_api_request_configs.inputs` into specific JSON Schemas and tool bindings. Object/array bodies and local references are retained; large JSON integers are not converted to float64.
- JSON body arrays with the internal `item_type: "any"` and no explicit `json_schema` publish `items: {}`. Elements may be any JSON value while the outer array, required/nullable rules and configured body pointer are preserved. Explicit schemas remain authoritative; unknown types and invalid explicit schemas are still rejected. This needs no database migration or tool-name changes.
- Each cached tool retains only identity/title/provider/public description, published Input Schema and compiled validator, plus a thin gateway request specification (method/path/content type, parameter placement, query encoding and unsupported-response flag). Raw database records and duplicate original input schemas are released after refresh validation/hashing. Source override configuration remains once in the manager for future refreshes.
- A request pins one immutable catalog generation. Refresh defaults to 30 seconds; after 120 seconds without successful verification, new MCP requests receive 503. Unnamed rows are reported as `pending` and skipped. Incompatible named APIs are quarantined individually; compatible tools remain available and `/readyz` reports `quarantined`. Quarantined APIs are checked again on every refresh and return automatically when their configuration is corrected. No usable tools in a nonempty active catalog means not ready. Database read failures and catalog-wide integrity errors retain the last verified snapshot until it expires.
- `--check-catalog` audits unnamed APIs too and outputs status, `issues` and `warnings`. Diagnostics identify the API ID, provider, method, route and original request content type. Hard issues cause exit 1; warnings alone exit 0. `warning_count` counts warnings, not APIs. Warnings and quarantine identities are logged only on a new catalog generation. This is a stricter operator audit; its nonzero exit does not stop a separately running server from publishing compatible tools. Public `/readyz` exposes counts, not issue details.
- Description review keywords no longer quarantine APIs. Unreviewed tool descriptions fall back to the API title; affected parameter descriptions fall back to the parameter name. Operator-reviewed public descriptions remain supported. Original review text is not included in warnings.
- Gateway methods GET, POST, PUT, PATCH and DELETE are supported, preserving their method. Bodyless requests use `none`; non-GET JSON requests use `application/json`. A GET marked JSON with no body inputs is normalized to `none` with a warning. Real GET body inputs, body inputs with `none`, and unsupported body encodings remain hard errors.
- Legacy nullable enums without an explicit `json_schema` include null in the generated enum. Legacy numeric enum strings are projected as exact JSON numbers. Explicit schemas keep their enum/null constraints. No database edits are required.
- Optional query parameters declared nullable accept explicit JSON `null` by omitting that query parameter. Omitted values, `false`, `0`, empty strings and empty arrays remain distinct; the configured array policy still controls empty arrays. JSON body nulls remain unchanged. Nullable path/header inputs and required nullable query inputs remain unsupported and are quarantined.
- Lite discovery defaults to five keyword matches (maximum ten); `get_api` returns the exact original input schema and public pricing. `run_api` requires the returned per-API version (including that API revision) and shares the existing validator/admission/credential/transport path. No schema/price lock or automatic business retry is implied. Price reads use an independent refresh task and a separate one-connection read-only pool; failures report unavailable pricing without failing core readiness. `lite_ready` and `pricing_available` appear in readiness only when enabled. Price/search versions do not affect the direct catalog hash.
- When lite is enabled, the same auxiliary refresh also GETs `{OPENANY_MODELS_BASE_URL}/v1/models` with no API key and caches the rows in memory. The default origin is `https://api.openany.si`. A path of `/v1` is accepted and stripped. `search_apis` can filter `kind` to `api`, `model` or `all` (default `all`). `kind=api` keeps the previous API ranking. `get_api` on a model returns the chat completions input shape. `run_api` posts that input to `{origin}/v1/chat/completions` with the caller key. A failed refresh keeps the last successful model list, marks `models_stale`, and does not fail API readiness. `/readyz` adds `models_available`, `models_stale` and `models_published` only while the loader is configured. The direct `/mcp` catalog hash does not include models.
- Public price loading selects only sale rules and public billing fields. Reader permissions for these columns are documented in the connection guide; no supplier costs, upstream keys or new business tables are needed. The same binary can serve a separate lite instance for stronger load isolation.
- Tools are sorted and paginated 50 at a time. Cursors bind catalog hash and provider view. A catalog change requires restarting discovery. `listChanged` is not advertised; clients re-list/reconnect.
- Only declared business headers and current-call credential headers reach the gateway. Client URLs, redirects, transport headers and environment proxies cannot override the fixed gateway origin.
- One tool invocation dispatches one business request, with no application retry. The business transport uses HTTP/1.1 without keepalive reuse to avoid transparent replay of billable GETs. Reverse proxies must also disable retries.
- HTTP 200 can still contain the gateway's `type=openany_error`; it becomes `isError=true`. Tool errors retain the gateway status and request ID. Oversized or unsupported results produce explicit errors, never truncated success.
- A timeout/disconnect after dispatch is **execution unknown**, not proof of no charge. The gateway may continue execution. Repeating the tool call can charge again; MCP request IDs are not gateway idempotency keys.
- `gang /api/stream` is discoverable but explicitly unsupported for continuous business SSE. It fails locally before dispatch. Other supported responses are finite JSON (including +json), UTF-8 text/plain and text/markdown. Text uses the existing result envelope with string data; HTML, binary and SSE remain unsupported. Response errors retain bounded content_type diagnostics and request IDs without echoing response bodies.
- Array rules resolve by exact `(provider_id, method, path_template)` across database UUIDs. Existing API UUID overrides take precedence per parameter. The shipped route rules cover 102 audited array fields on 20 BlockRun, LI.FI, Polymarket and Blockscout routes, plus eight explicit historical BlockRun path variants. They are source-derived, without paid end-to-end calls. New rules reject unverified empty arrays locally and advertise `minItems: 1`; stronger existing limits and old explicit empty-array policies are preserved.
- A missing serialization policy on an optional query array emits a warning and allows the tool to publish. Omit that parameter (or pass null if nullable) to call the API. Supplying an array fails locally with `openany_mcp_array_policy_missing` and `execution_state: not_started`; no query format is guessed and no gateway request is sent. A missing required policy, an invalid configured policy, and repeated keys with rules billing remain hard errors.

## Configuration

See `.env.example` for local configuration. Required: `POSTGRES_DATABASE_URL`, `OPENANY_GATEWAY_BASE_URL`. Optional: `OPENANY_MODELS_BASE_URL` (default `https://api.openany.si`). The Docker image supplies `OPENANY_GATEWAY_BASE_URL=http://127.0.0.1:8888` and listens on `0.0.0.0:3000`. The gateway URL must be an origin without userinfo, query, fragment or non-root path. The model origin is separate from the agent-API gateway: catalog reads use `GET /v1/models`, and model execution uses `POST /v1/chat/completions` on that origin. The process needs egress to the model origin. No model API key is stored in this service.

Defaults: listen `127.0.0.1:3333`, database pool 4, concurrent calls 16, query timeout 10s, gateway connect 5s, gateway total 1260s, shutdown 1300s; input 8 MiB, gateway response 8 MiB, MCP result 20 MiB. Host defaults only allow localhost/loopback; production must set `MCP_PUBLIC_HOSTS`. If an Origin header is supplied, it must be in `MCP_ALLOWED_ORIGINS`.

Gateway timeout comes from the gateway's actual global execution budget, not `agent_apis.timeout_ms`. The audited gateway default was 1200s. Coordinate client, reverse-proxy and termination deadlines; short client deadlines do not cancel billing.

The service uses PostgreSQL `default_transaction_read_only=on` in addition to role grants. Runtime SELECT columns: id, name, description, provider_id, public_mcp_tool_name, method, path_template, public_path, status, revision, upstream_protocol, request_content_type, unknown_parameters, billing_mode; request config needs api_id and inputs. Maintenance needs SELECT naming fields and UPDATE(public_mcp_tool_name, updated_at). DDL belongs to a migration role.

The deployed gateway may install SECURITY INVOKER validation triggers on `agent_apis`. In this local schema, maintenance also needs SELECT on `agent_apis.billing_mode`, `agent_api_pricing_policies.api_id` and `provider_registration_publications(upstream_key_id, registration_id)`. Those four column-read grants are included in the local setup; UPDATE is still limited to `public_mcp_tool_name` and `updated_at`. APIs linked to provider registrations may need additional trigger read/row-lock privileges, audited against the deployed gateway schema.

## Verification

```sh
go test ./...
go test -race ./...
go vet ./...
.venv/bin/python -m unittest discover -s scripts -p 'test_*.py'
```

The Python DB tests require `MCP_TEST_DATABASE_URL` pointing to a disposable database whose name starts with `openany_mcp_test`: they drop/recreate `public.agent_apis`. **Never point this test variable at your real catalog.** Without the variable, only DB integration cases are skipped.

The Go loader integration test uses `MCP_TEST_CATALOG_DATABASE_URL` and refuses database names without the `openany_mcp_test_` prefix. It recreates `public.agent_apis` and `public.agent_api_request_configs`; use a separate disposable database. It checks that malformed inputs in one named or unnamed API do not block other tools. Without this variable, that database case is skipped.

For a full offline snapshot, supply `MCP_TEST_CATALOG_FILE` (Record JSON with config_present and proposed/stored names) and absolute `MCP_TEST_OVERRIDES_FILE` when running `TestFullCatalogSnapshot`. Full production metadata is not committed as a fixture.

Tests use local gateway substitutes. No production key, paid provider call, or direct supplier connection is needed. SDK version tests cover 2025-03-26, 2025-06-18, 2025-11-25 and 2026-07-28. Product-specific Grok/Claude/Cursor setup must still be validated with those products; standard HTTP/SDK tests do not establish their header/UI behavior.

## Container and rollback

```sh
docker build -t openany-mcp:local .
# Export database credentials and the public Host/Origin allowlists before running.
docker run --rm \
  -e POSTGRES_DATABASE_URL \
  -e MCP_PUBLIC_HOSTS \
  -e MCP_ALLOWED_ORIGINS \
  -p 3000:3000 openany-mcp:local
```

The image defines the MCP listener (`0.0.0.0:3000`, exposed TCP port **3000**) and gateway origin (`http://127.0.0.1:8888`) in the Dockerfile; neither needs a runtime environment variable. The loopback gateway address requires the gateway to share the container's network namespace, such as another container in the same Kubernetes Pod. Separate Docker containers or Kubernetes Pods must use a reachable gateway service address. The local `.env.example` is for local execution; passing it with `--env-file` would override image defaults, including the listener.

The service image contains no Python. Run naming maintenance as a separate task. Public deployments require HTTPS at the ingress, explicit Host/Origin settings, no business retries, suitable response buffering and deadlines, and termination grace >=1310 seconds for the default long-call budget.

Rollback the service/config image, keeping the persistent name column and assigned values. Do not clear/rebuild names. Structured logs omit credentials and business bodies; request IDs correlate calls with gateway usage.

The approved design and original mapping proposal are under `docs/superpowers/specs/`. The original design retains the approved baseline; the readable-name amendment, this README and `docs/verification.md` record subsequent changes and verification.

### Discovery and request guidance

Search retains latest/hot/history intent, prefers business-concept coverage over repeated broad metadata and matches complete Latin words. Ranking upgrades invalidate old search cursors, not tool names or execution versions. Public request examples preserve optional business values; placeholder templates are labeled template_only and identify fields to replace. Valid parameterless requests offer input {}. Local schema/encoding validation does not prove supplier acceptance. See [the connection guide](mcp.md) for response modes and example kinds.
