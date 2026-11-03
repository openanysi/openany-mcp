# OpenAny MCP

Independent Go MCP server for OpenAny Agent APIs. Reads a PostgreSQL catalog and forwards each tool call to the configured gateway with **that request's credentials**. Authentication, rate limits, billing and upstream execution remain in the gateway. A separate Python CLI assigns persistent public names.

## Local quick start

```sh
cd /Users/legend/WorkSpace/openany/openany-mcp
go build -trimpath -o bin/openany-mcp ./cmd/openany-mcp
./scripts/run-local.sh --check-catalog
./scripts/run-local.sh
```

The local installation includes `.env` (reader role) and `.env.maintenance` (naming role), both mode 600 and excluded from Git. `run-local.sh` loads `.env`. Go itself reads environment variables, not dotenv files. No gateway API key is stored in either configuration; callers supply their own.

Endpoints:

- `http://127.0.0.1:3333/mcp` — complete catalog.
- `http://127.0.0.1:3333/mcp/blockrun` — provider-specific catalog; same names.
- `/healthz` — process health; `/readyz` — catalog age and counts.

Configure a compatible MCP client for **Streamable HTTP** and attach `Authorization: Bearer YOUR_OPENANY_KEY` (or the gateway-supported API-key header). The service does not implement an OAuth login. Calls without credentials are forwarded without a fallback key and rejected by the gateway. Native clients need no Origin; browser origins must be explicitly allowed.

## New installation

1. Copy `.env.example` to a private local `.env`, replace database credentials and set the gateway origin. Use a SELECT-only database role for the service.
2. Have an operator run `migrations/001_add_public_mcp_tool_name.sql` against the intended database. It adds a nullable text column, validated format constraint and global unique index; it does not change prices, enable APIs or assign names.
3. Review API descriptions and array query serialization. Source-derived local overrides are in `config/catalog-overrides.json`; evidence and limitations are in `config/transport-evidence.md`.
4. Run `--check-catalog` to validate active metadata, including unnamed rows. This command never calls a business API or writes names.
5. Preview and apply name assignment using a **separate maintenance credential**, then re-check the catalog.

Python setup and assignment:

```sh
uv venv --python 3.12 .venv
uv pip install --python .venv/bin/python -r scripts/requirements.lock
# Export a maintenance POSTGRES_DATABASE_URL in this shell before these commands.
.venv/bin/python scripts/sync_mcp_tool_names.py
.venv/bin/python scripts/sync_mcp_tool_names.py --apply
```

The default is read-only preview. `--apply` assigns only NULL values in one transaction, using advisory/row locks and the unique index. Existing or manually assigned names are never changed. A collision extends only the new UUID suffix. Disabled APIs also receive and retain names. No force-rename option exists.

Names normally look like `blockrun_get_api_v1_crypto_list_e1183d75`. They are stored in `agent_apis.public_mcp_tool_name`; the existing `mcp_tool_name` remains the upstream tool name. API imports and updates must preserve this column when not explicitly changing it. No control-plane importer was modified by this independent project.

## Runtime behavior

- Active, named APIs are compiled from `agent_api_request_configs.inputs` into specific JSON Schemas and tool bindings. Object/array bodies and local references are retained; large JSON integers are not converted to float64.
- A request pins one immutable catalog generation. Refresh defaults to 30 seconds; after 120 seconds without successful verification, new MCP requests receive 503. Unnamed rows are reported and skipped. Invalid named metadata prevents the whole new snapshot from publishing.
- Tools are sorted and paginated 50 at a time. Cursors bind catalog hash and provider view. A catalog change requires restarting discovery. `listChanged` is not advertised; clients re-list/reconnect.
- Only declared business headers and current-call credential headers reach the gateway. Client URLs, redirects, transport headers and environment proxies cannot override the fixed gateway origin.
- One tool invocation dispatches one business request, with no application retry. The business transport uses HTTP/1.1 without keepalive reuse to avoid transparent replay of billable GETs. Reverse proxies must also disable retries.
- HTTP 200 can still contain the gateway's `type=openany_error`; it becomes `isError=true`. Tool errors retain the gateway status and request ID. Large or non-JSON results produce explicit errors, never truncated success.
- A timeout/disconnect after dispatch is **execution unknown**, not proof of no charge. The gateway may continue execution. Repeating the tool call can charge again; MCP request IDs are not gateway idempotency keys.
- `gang /api/stream` is discoverable but explicitly unsupported for continuous business SSE. It fails locally before dispatch. Other supported responses are finite JSON.
- The 46 Blockrun query arrays use specification-derived repeated keys from the provider's OpenAPI, not a paid end-to-end test. Rules billing cannot combine with repeated query keys; validation rejects that combination.

## Configuration

See `.env.example`. Required: `POSTGRES_DATABASE_URL`, `OPENANY_GATEWAY_BASE_URL`. The gateway URL must be an origin without userinfo, query, fragment or non-root path.

Defaults: listen `127.0.0.1:3333`, database pool 4, concurrent calls 16, query timeout 10s, gateway connect 5s, gateway total 1260s, shutdown 1300s; input 8 MiB, gateway response 8 MiB, MCP result 20 MiB. Host defaults only allow localhost/loopback; production must set `MCP_PUBLIC_HOSTS`. If an Origin header is supplied, it must be in `MCP_ALLOWED_ORIGINS`.

Gateway timeout comes from the gateway's actual global execution budget, not `agent_apis.timeout_ms`. The audited gateway default was 1200s. Coordinate client, reverse-proxy and termination deadlines; short client deadlines do not cancel billing.

The service uses PostgreSQL `default_transaction_read_only=on` in addition to role grants. Runtime SELECT columns: id, name, description, provider_id, public_mcp_tool_name, method, path_template, public_path, status, revision, upstream_protocol, request_content_type, unknown_parameters, billing_mode; request config needs api_id and inputs. Maintenance needs SELECT naming fields and UPDATE(public_mcp_tool_name, updated_at). DDL belongs to a migration role.

## Verification

```sh
go test ./...
go test -race ./...
go vet ./...
.venv/bin/python -m unittest discover -s scripts -p 'test_*.py'
```

The Python DB tests require `MCP_TEST_DATABASE_URL` pointing to a disposable database: they drop/recreate `public.agent_apis`. **Never point this test variable at your real catalog.** Without the variable, only DB integration cases are skipped.

For a full offline snapshot, supply `MCP_TEST_CATALOG_FILE` (Record JSON with config_present and proposed/stored names) and absolute `MCP_TEST_OVERRIDES_FILE` when running `TestFullCatalogSnapshot`. Full production metadata is not committed as a fixture.

Tests use local gateway substitutes. No production key, paid provider call, or direct supplier connection is needed. SDK version tests cover 2025-03-26, 2025-06-18, 2025-11-25 and 2026-07-28. Product-specific Grok/Claude/Cursor setup must still be validated with those products; standard HTTP/SDK tests do not establish their header/UI behavior.

## Container and rollback

```sh
docker build -t openany-mcp:local .
# Set an explicit public Host allowlist; 127.0.0.1 inside a container is the container itself.
docker run --rm --env-file .env -e MCP_LISTEN_ADDR=0.0.0.0:3333 -p 3333:3333 openany-mcp:local
```

The service image contains no Python. Run naming maintenance as a separate task. Public deployments require HTTPS at the ingress, explicit Host/Origin settings, no business retries, suitable response buffering and deadlines, and termination grace >=1310 seconds for the default long-call budget.

Rollback the service/config image, keeping the persistent name column and assigned values. Do not clear/rebuild names. Structured logs omit credentials and business bodies; request IDs correlate calls with gateway usage.

The approved design and original mapping proposal are under `docs/superpowers/specs/`. Their original "not implemented" status describes the planning baseline; this README and `docs/verification.md` record the implementation state.
