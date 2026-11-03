# Implementation and local verification

Date: 2026-11-03. Source branch: codex/initial-implementation. No remote push or public deployment.

Implemented Go service, Python NULL-only name assignment, operator SQL migration, source-backed transport overrides, health/configuration, Docker build recipe and tests.

Verified before installation:

- Go normal tests, race detector and vet pass; tests use local HTTP substitutes.
- Four target MCP protocol versions, request credential isolation, provider views, generation-bound pagination, stale recovery/drain, root array encoding, precise JSON numbers, local references, HTTP-200 gateway errors, redirect/no-replay and non-JSON stream rejection.
- Python 7 tests pass, including a disposable PostgreSQL instance: read-only preview, idempotency, concurrent scripts and rollback of partial assignment attempts.
- The local openany_local migration added public_mcp_tool_name plus the format check and global nullable unique index.
- Full active metadata/schema/transport compilation succeeded before name backfill.
- First local apply assigned 970 names; second apply assigned 0.
- Live metadata check after backfill: active=956, pending=0, published=956. Published is a tool definition count, not supplier success count.
- Separate local database reader/namer roles were created. Reader cannot update tool names; namer cannot update prices. Private role credentials are stored only in ignored mode-600 .env files.
- Maintenance-role preview succeeds with assigned=0. The runtime reader successfully loads the full catalog.
- The original port-3333 Python smoke process is retained; local new-service installation uses port 3335.

Decisions recorded during implementation:

1. A brand-new repository is the isolated workspace; no gateway worktree or source change is needed. The project was authored in writable temporary staging, then installed into the user-requested directory with authorized filesystem access.
2. User explicitly approved the full design and requested implementation; no extra implementation-plan approval gate was added.
3. Go SDK servers are memoized for a validated immutable snapshot on first use. A request pins its selected server in context, avoiding catalog/MCP import cycles without mixing generations. Cost: a small first-request construction cost for a new view.
4. All 46 stored query arrays were matched to the provider OpenAPI. Repeated-key encoding is derived from OpenAPI 3.1 form/explode=true, not claimed paid-provider success. Non-keyset Polymarket uses the documented same-filter keyset contract. Re-review if provider contracts change.
5. Local compiler is Go 1.27.0; the module minimum satisfies the SDK and the container recipe pins Go 1.27.3. Container build verification is recorded separately; it is not inferred from native tests.

Explicit boundaries:

- No real user key was read from account tables or used to execute paid supplier calls.
- Gang continuous business SSE remains explicitly unsupported.
- Historical QCC availability/schema uncertainty remains visible in public descriptions.
- Product-specific Grok/Claude/Cursor header/UI interoperability is not established by SDK/HTTP tests.
- Existing API importers were not modified; they must preserve the new public name column.

Final review and installation verification:

- Independent whole-project review reported two Important issues, no Critical/Minor: draft-07 reference sibling semantics and overlapping body pointers. Both were reproduced with failing tests and fixed, then the full race suite passed.
- Related regressions cover modern dependentRequired, unsupported embedded resource identity, literal properties called definitions, decoded pointer overlap and input non-mutation.
- A slow database refresh was reproduced blocking old-snapshot readiness; refresh serialization now uses a separate mutex, and the regression/race suite passes.
- Published schema content participates in the catalog hash, so transformation changes invalidate old cursors.
- Final-installation HTTP discovery enumerated 956 unique tools across 20 pages; the accurlex view contained 3 tools. No business call was made.
- Official MCP Inspector CLI discovered all 76 BlockRun tools: 0 schema errors and 2 portability warnings for intentionally unconstrained JSON-RPC params items. These schemas retain the provider's any-JSON contract rather than being narrowed to objects.
- Docker daemon is not running; native build/tests passed, but no container image build is claimed.
- Test database setup now refuses database names without the openany_mcp_test prefix. All 7 Python tests passed against that disposable database.

Review scope rulings:

- Paid supplier behavior and actual product-client UI integration were deliberately not simulated as successful live tests; no user account key was obtained from the database.
- Production proxy behavior and production grants were not exercised. Local reader/namer grants and local catalog reads were verified; deployment guidance specifies retry/time-budget settings.
- Retaining unrestricted RPC array items can produce compatibility warnings in stricter clients; narrowing them would reject valid provider inputs.
- This is a new standalone repository with no existing merge base or remote; preserve the local codex branch and commits, with no merge/push operation.
