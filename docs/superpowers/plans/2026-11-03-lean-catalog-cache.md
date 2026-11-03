# Lean Catalog Cache Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution.

**Goal:** Retain only data needed by MCP discovery, validation and gateway forwarding.
**Architecture:** Separate refresh Record/Input from runtime Request/Parameter/ArrayPolicy; bind the latter into an immutable catalog. Keep source hashing unchanged.
**Tech Stack:** Existing Go standard library, MCP SDK and JSON Schema validator.
**Spec:** docs/superpowers/specs/2026-11-03-lean-catalog-cache-design.md

## Global Constraints

- No database writes, tool-name/schema/protocol changes, new dependencies or Redis.
- Keep required compiled schemas and encoder safety checks.
- Copy mutable source pointers/maps; preserve generation/cursor hashes.

## Review Focus

- Raw metadata accidentally retained through Binding or handler closures.
- Lost query encoding, root body mapping or unsupported-response behavior.
- Source aliasing changing an already-published snapshot.
- Hash/cursor changes when all source and published data are unchanged.
- Discovery or credential forwarding regressions after the runtime type switch.

### Task 1: Thin runtime bindings and verification

**Files:** internal/model/model.go, internal/catalog/snapshot.go, internal/transport/encode.go, internal/gateway/client.go, internal/mcpserver/server.go and their tests; README.md; docs/verification.md.

- [ ] Save full live discovery and its catalog hash as a before baseline.
- [ ] Add behavior regressions for source-independent cached routing/schema and unsupported forwarding; observe RED.
- [ ] Introduce thin runtime types and project validated records, update direct callers/tests.
- [ ] Run full Go tests, race, vet, Python regressions and actual read-only catalog compilation.
- [ ] Get one independent read-only review and address Important/Critical findings with RED/GREEN.
- [ ] Build/install locally, restart the existing service gracefully and verify exact discovery/hash equivalence.
- [ ] Record results and save local commits without remote push.
