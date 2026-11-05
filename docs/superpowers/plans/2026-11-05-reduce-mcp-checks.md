# Reduce MCP Checks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Publish compatible tools across environments while moving nonessential catalog checks to actionable warnings.

**Architecture:** Preserve the current immutable catalog pipeline. Resolve overrides by stable route plus UUID, normalize only known-equivalent metadata and compile request capabilities into the existing thin bindings. Keep unsupported calls local before the gateway.

**Tech Stack:** Go, existing MCP SDK/jsonschema/pgx dependencies, JSON configuration.

**Spec:** `docs/superpowers/specs/2026-11-05-reduce-mcp-checks-design.md`

## Global Constraints

- No business database writes, credential configuration changes, gateway changes, new dependencies or paid API calls.
- Preserve explicit JSON Schema constraints, HTTP methods, request placement and exact JSON numbers.
- Keep old UUID policies compatible; route matching is exact, without wildcards or request URL rewriting.
- User has approved implementation and integration into the existing project/main workflow. Do not repeat approval questions for these reversible changes.

## Review Focus

- Nullable legacy enums must not weaken explicit schema enums.
- Unknown optional arrays and unverified empty arrays must never reach the gateway with guessed encoding.
- Route policy resolution must not share mutable maps or cross provider/method/path boundaries.
- Description fallback must not leak the original suspicious text into diagnostics or tool descriptions.
- Metadata normalization must never drop real GET body inputs or change DELETE/PATCH to POST.

### Task 1: Legacy enum projection

Files: `internal/schema/compile.go`, `internal/schema/compile_test.go`.

- [ ] Add failing tests for nullable legacy enums (absent/null source schema), enum membership and explicit-schema precedence.
- [ ] Run focused tests and observe the existing nullable conflict.
- [ ] Include null only in legacy generated enum arrays, preserving source bytes and all other constraints.
- [ ] Run the schema suite and commit.

### Task 2: Stable route policies and empty-array capability

Files: `internal/model/model.go`, `internal/catalog/overrides.go`, `internal/catalog/overrides_test.go`, `internal/schema/compile.go`, `internal/schema/compile_test.go`, `internal/transport/encode.go`, transport tests, `config/catalog-overrides.json`.

Interfaces: `RouteOverride` embeds `Override` and has provider/method/path keys; `Resolve` applies exact route defaults and UUID-specific overrides. `Input.NonEmpty` is an internal-only capability flag. `empty: reject` keeps stronger schema limits.

- [ ] Add failing route precedence, duplicate-route, immutable-map and environment UUID tests; add nonempty array schema/encoder tests.
- [ ] Implement minimal policy resolution and the internal nonempty capability; preserve explicit schema references and stronger constraints.
- [ ] Populate the audited 102 parameter rules and historical explicit BlockRun variants from existing source evidence; keep UUID overrides.
- [ ] Run affected suites and commit.

### Task 3: Relax catalog checks and validate the complete service

Files: `internal/catalog/snapshot.go`, catalog tests, `internal/gateway/client.go`, gateway tests, `cmd/openany-mcp` diagnostics/tests, README and verification record.

Interfaces: add `Snapshot.Warnings []Issue`, safe provider/method/path/content-type identity in issues and `warning_count` in status. `compileBinding` returns binding, warnings and error. Unknown optional array calls return a specific local error before dispatch.

- [ ] Add failing tests for description fallback, methods/body rules, optional versus required unknown arrays, CLI warning/error behavior and safe logging.
- [ ] Implement warnings/normalization and use resolved route policies; preserve hard validation and snapshot expiration.
- [ ] Update obsolete tests to use required arrays where isolation is expected. Verify optional unsupported parameters cannot cause business dispatch.
- [ ] Run full Go tests/race/vet, full offline local metadata validation, read-only actual catalog check and an independent temporary-port discovery check.
- [ ] Obtain one fresh whole-change review; fix Important findings with regression tests.
- [ ] Document limitations, integrate into main without discarding remote changes, rebuild the local binary and push normally under the existing authorization.
