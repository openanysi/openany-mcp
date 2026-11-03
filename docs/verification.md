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
- Final-installation HTTP discovery enumerated 956 unique tools across 20 pages; the accurlex view contained 3 tools. A tools/call with no credentials reached the local gateway and returned HTTP 401 with a gateway request ID. No authenticated supplier call was made.
- Official MCP Inspector CLI discovered all 76 BlockRun tools: 0 schema errors and 2 portability warnings for intentionally unconstrained JSON-RPC params items. These schemas retain the provider's any-JSON contract rather than being narrowed to objects.
- Installed-project Go race tests and go vet passed again; the four pure Python naming tests passed in the installed virtual environment.
- Docker daemon is not running; native build/tests passed, but no container image build is claimed.
- Test database setup now refuses database names without the openany_mcp_test prefix. All 7 Python tests passed against that disposable database.

Review scope rulings:

- Paid supplier behavior and actual product-client UI integration were deliberately not simulated as successful live tests; no user account key was obtained from the database.
- Production proxy behavior and production grants were not exercised. Local reader/namer grants and local catalog reads were verified; deployment guidance specifies retry/time-budget settings.
- Retaining unrestricted RPC array items can produce compatibility warnings in stricter clients; narrowing them would reject valid provider inputs.
- This is a new standalone repository with no existing merge base or remote; preserve the local codex branch and commits, with no merge/push operation.


Readable-name amendment verification (2026-11-03):

- User explicitly chose no fixed UUID suffix, `_2`/`_3` collision aliases, and migration of existing generated names.
- New natural-name assertions failed against the original implementation. Legacy migration tests initially failed because the new API did not exist; the rollback test uses the specific database RaiseException so a missing Python parameter cannot satisfy it.
- Full Python suite passed 18/18: 11 unit tests and 7 disposable-PostgreSQL cases. Coverage includes multi-digit alias truncation, exact legacy detection (all four suffix lengths), manual/disabled names, cross-occupied old names, concurrent migration and restoration of cleared names after a failed final write.
- Independent read-only review of 7d3632f..3f99932 found no Critical/Important/Minor issues; reviewer independently passed all 11 unit tests and diff whitespace checks.
- Real preflight found 7 custom gateway triggers. Name-only updates run validation guards and the deferred pricing check, with no price/status/revision mutation. Added their four required column SELECT grants to the local naming role; a forced-rollback UPDATE plus immediate deferred-constraint evaluation passed. Writable columns remain exactly public_mcp_tool_name and updated_at.
- Preview: 970 rows all exactly matched the old algorithm, 970 proposed renames, no unmatched stored names. At the 64-character natural-name limit there are 4 duplicate groups; 6 numeric aliases resolve all final candidate collisions.
- Applied all 970 renames in one transaction using the restricted naming role. Output matched the saved preview exactly. Second explicit migration apply assigned=0 and renamed=0.
- User example now persists as dataforseo_post_v3_backlinks_summary_live. All 970 database names match both the migration report and the updated one-to-one mapping artifact, with global uniqueness.
- Server-side fingerprints before/after confirm every other agent_apis field except updated_at is unchanged (including prices, status, revision and upstream tool names); all request config data is unchanged.
- Runtime reader --check-catalog passes: active=956, pending=0, published=956. The already-running service auto-refreshed without restart. HTTP discovery returned all 956 new names across 20 pages and 154 in the DataForSEO view; the old example name is absent and the new name is present.
- Private before/preview/apply/no-op/fingerprint reports are in ignored var/maintenance/ with mode 600. No paid business call was made for this change.

Review scope rulings for the amendment:

- Existing clean standalone checkout and a new codex/readable-tool-names branch satisfy the user's target-directory workflow; no additional worktree or remote push was needed. Concurrent external edits would require reconciliation.
- Review ran before the real migration; only generated mapping and verification/operational docs changed afterward. This reduced live-reference risk without another review loop.
- The reviewer did not independently access real triggers, roles, migration or runtime refresh; these were directly verified by the implementing agent and the evidence is listed above.
- Extreme same-name catalog sizes were not load-tested; current 970-row preview/apply completed with 6 collision aliases.
- A hand-picked value identical to the exact legacy algorithm remains indistinguishable without provenance. Current 970 values match the stored initial assignment baseline, and the user authorized their migration.


Lean catalog cache verification (2026-11-03):

- User requested removing refresh-only metadata and duplicate original schemas from the persistent per-tool cache.
- Binding no longer stores model.Record or model.Override. It retains API/provider identity, title/public description, compiled Input Schema/validator and a thin model.Request. Request contains method/public path/content type, minimal name/location/pointer mappings, array encoding/empty policies and unsupported-response flag.
- Raw status, revision, upstream protocol, billing mode, path template, config marker, unknown-parameter policy, original input schema/enum/bounds and override evidence are used during refresh validation/hashing rather than retained in each Binding. One authoritative overrides config remains in Manager for future refreshes.
- Behavioral regression TestCachedCallsDoNotDependOnMutableSourceRecords failed before implementation: mutating original input containers changed forwarded body to {} and changed repeated query keys to CSV. With detached runtime pointers/maps it passes, retaining exact large integers, expected body placement, query encoding, discovery and current-request credentials.
- Full Go normal and race suites passed; go vet passed. Additional regressions retain refresh-only revision hash invalidation and local unsupported-response rejection before dispatch.
- Python 11 pure tests passed; 7 database integration tests were skipped because no disposable database URL was configured. Python and database maintenance logic were unchanged by this refactor.
- New binary read-only --check-catalog validated active=956, published=956, pending=0 and the exact existing hash 44977747640192d19523f17b7760ac7d6c7f518d336ff6a7d5babcb36c9bdff7.
- Independent read-only review of f58785a..b65af90 found no Critical/Important/Minor issues, independently ran go test -count=1 ./... and diff whitespace checks, and found no raw-record retention through production loader or handler closures.
- Saved full live discovery before installation; verified the old process executable belongs to this project, retained a rollback binary, installed the reviewed build and sent graceful termination to the old process. New process is ready on the existing port 3335 using the existing private configuration.
- Installed-service discovery returned 956 tools across 20 pages. Every full tool definition exactly equals the saved pre-change definition, and catalog hash is unchanged. No tool names or Agent configuration changed.
- No database writes or paid business calls were made for this change. No exact retained-heap or performance improvement is claimed without profiling.

Review scope rulings for lean cache:

- Used a clean codex/lean-catalog-cache branch in the specified project, keeping the existing local installation workflow. Concurrent external changes would require reconciliation; no remote push or merge was performed.
- Retained one source override config for refresh validation/evidence and hashing, without copying the full config into runtime Bindings. Removing it would change refresh/source validation behavior.
- The reviewer did not independently access real metadata or installation; the implementing agent directly verified actual 956-tool compilation, exact hash, restart readiness and complete live discovery equivalence.
- Exact heap savings, load performance, paid supplier results and product-specific client UI were not measured. Local HTTP substitute tests and live metadata discovery establish the stated scope.


Script-local database configuration verification (2026-11-03):

- User approved a top-level editable DATABASE_URL, no POSTGRES_DATABASE_URL fallback, preview by default and --apply writes.
- The committed default is empty. CLI strips surrounding whitespace and reports a configuration error before connecting when empty; both preview and apply subprocess checks confirmed that an existing environment URL does not bypass this error. --help remains available without configuration.
- Successful result target metadata comes from conn.info.host, port and dbname plus the fixed public.agent_apis destination; output does not include the full connection string or password. Existing synchronize explicit-URL API, SQL statements, locks, expected-value checks, naming rules and legacy migration remain intact.
- RED observed on the old implementation: blank-file-variable and real CLI selection tests failed because the environment variable controlled the connection.
- Full Python suite passed 21/21: 13 pure tests and 8 real disposable-PostgreSQL cases, including CLI target selection, whitespace trimming, preview/apply/idempotence and sanitized output. Go go test ./... also passed.
- Actual edit-file workflow was exercised through a temporary copy of the script against two newly created databases, openany_mcp_test_script_config_a and _b. Each run deliberately set the environment to the opposite database. Preview wrote neither; apply updated only the configured database; switching the file variable selected the other database without touching the first; repeated apply assigned=0. Full connection strings were absent from captured output. Both databases were removed after the check.
- Maintenance README examples now tell operators to edit the script variable. The separate Go runtime still uses environment configuration; its binary/configuration was not changed or restarted. No real business database was modified and no supplier API was called.

Scope rulings for script configuration:

- Worked in the specified project on codex/script-database-config. The unrelated pre-existing .DS_Store was preserved; no remote push/merge is planned.
- Did not put a real connection string in the tracked script or run an apply against the business catalog; the user chooses the destination by filling the approved empty variable locally.

- Independent read-only review of e1ada41..05dabfc found no Critical/Important/Minor issues, independently passed 13 pure Python tests and diff checks, and confirmed configuration selection/output, preview safety and unchanged transaction boundaries.
- Reviewer did not independently repeat database tests or production permissions/migrations; the implementing agent directly verified the disposable database scenarios above. No conclusion about a production target was inferred from those tests.
- Driver semantics remain standard: missing connection fields may use libpq defaults, and target.host/port/database describe the driver's connection rather than independently authenticating a proxy's backend. README requests a complete connection string; no extra DSN parser or backend identity lookup was added.
