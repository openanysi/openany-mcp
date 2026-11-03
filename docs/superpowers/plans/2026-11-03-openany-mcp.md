# OpenAny MCP Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement task-by-task. Follow test-driven development and a fresh final code review.

**Goal:** Deliver the standalone Go MCP service and Python persistent-name maintenance in the user-requested directory.
**Architecture:** Read-only PostgreSQL metadata becomes an immutable, validated MCP registry; tool calls carry current request credentials to a fixed gateway. A separate Python CLI assigns only missing names transactionally.
**Tech Stack:** Go official MCP SDK, pgx, jsonschema; Python psycopg; PostgreSQL.
**Spec:** ../specs/2026-11-03-openany-mcp-service-design.md

## Global Constraints
- Go service, Python name maintenance; no shared gateway credential.
- Only NULL public_mcp_tool_name values are assigned; names are globally unique and <=64 ASCII characters.
- One request uses one snapshot; fresh-check 30s and maximum staleness 120s by default.
- No business retries or redirects; preserve numeric precision and distinguish HTTP 200 gateway errors.
- Real supplier calls are not used as tests. Local database migration and backfill are explicitly recorded.

## Review Focus
- Concurrent users must never share credentials.
- Root array JSON and relocated local schema references must not change request semantics.
- A stale or failed refresh must not silently keep serving indefinitely.
- Timeouts and cancellations after dispatch must report execution uncertainty.
- Python concurrency or uniqueness errors must roll back all assignments.

### Task 1: metadata/schema/transport
Files: internal/catalog, internal/schema, internal/transport, testdata.
Interfaces: catalog.Record/Input; schema.Compile; transport.Encode.
- [ ] Write failing tests for schema constraints, numeric precision, JSON root arrays, local references and unsafe input.
- [ ] Implement immutable bindings and HTTP encoding without upstream transformations.
- [ ] Run targeted Go tests and record results.

### Task 2: gateway/MCP/server
Files: internal/gateway, internal/mcpserver, internal/config, cmd/openany-mcp.
Interfaces: gateway.Call; catalog.Loader/Manager; mcpserver.NewHandler.
- [ ] Write failing tests using real HTTP gateway substitutes for auth isolation, errors, no replay, scope and pagination.
- [ ] Implement PostgreSQL snapshots, SDK tool handlers, versioned pagination, admission and health.
- [ ] Run Go tests, race detector, vet and build.

### Task 3: name maintenance/migration
Files: migrations/001_add_public_mcp_tool_name.sql, scripts/sync_mcp_tool_names.py and tests.
- [ ] Write failing Python tests for deterministic names, collision extension and existing names.
- [ ] Implement transaction-only NULL backfill, migrations and minimal roles.
- [ ] Verify against a disposable PostgreSQL database including concurrent/rollback behavior.

### Task 4: catalog readiness/deployment
Files: config/catalog-overrides.json, README.md, Dockerfile, .env.example.
- [ ] Verify actual array serialization from authoritative source metadata/docs; retain explicit blockers when unverifiable.
- [ ] Run full current local catalog compilation without business calls.
- [ ] Apply authorized local migration and name backfill after dry-run; verify idempotency.
- [ ] Request one fresh whole-project review; fix material findings with regression tests.
- [ ] Install into /Users/legend/WorkSpace/openany/openany-mcp and rerun smoke/verification.
