# Lean catalog cache

User-approved change, 2026-11-03: remove refresh-only raw metadata and duplicate original schemas from the persistent in-process tool cache. Preserve discovery, validation, forwarding, catalog hashes and refresh behavior.

## Approach

Keeping an entire database Record but clearing unused fields saves payloads while leaving the wrong runtime contract. Use a small runtime Request/Parameter/ArrayPolicy plus Binding identity/title/provider/public description/compiled schema. Raw Record/Input and source overrides remain ingestion/configuration types, used during refresh validation and hashing only. No Redis, schema change, tool-name change or new dependency.

## Contract

- Each cached Binding retains API ID (logging), provider ID (views), title/public description, published Input Schema, compiled validator, and the minimal gateway request specification.
- Runtime requests retain method, public path, content type, parameter name/location/body pointer, query encoding/empty policy, and unsupported-response flag. Copy parameter pointers/maps so they do not retain mutable raw input containers.
- Do not retain Record status/revision/upstream protocol/billing mode/config marker/path template/unknown-parameter policy, raw input JSON Schema/enum/bounds/type/description, or override source annotations in per-tool bindings. Their effects remain in validation, compiled schemas, descriptions and catalog hash.
- Keep one authoritative source override configuration in Manager because future refreshes must validate it again. It is not copied wholesale into each runtime binding.
- Preserve hashing from raw records + source overrides + published schemas/descriptions; no cursor invalidation caused solely by this refactor.
- Retain runtime encoding safeguards and all previous behavior: credential isolation, exact JSON integers, root arrays, query policies, rejection of overlapping pointers, unsupported response mode and no retries.
- No concrete memory savings figure will be claimed without measuring retained heap. Verify code shape, regression behavior, full real catalog and exact live discovery before/after.
