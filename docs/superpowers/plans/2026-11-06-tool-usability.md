# MCP Tool Usability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline. User forbids subagents in this side chat; perform a separate author review, no delegation.

**Goal:** Improve discovery, input guidance and actionable execution feedback while preserving existing routes/contracts.
**Architecture:** Optional public metadata enriches immutable discovery snapshots; one validator/encoder remains authoritative; presentation happens after one gateway call.
**Tech Stack:** Existing Go/pgx/jsonschema/MCP SDK; no new dependencies.
**Spec:** docs/superpowers/specs/2026-11-06-tool-usability-design.md

## Global Constraints
- Isolated branch only; no parent dev changes, merge/push, private config edits, schema changes, business writes, paid calls or subagents.
- Preserve three tools/default5/max10, default raw data and direct schema/hash.
- Metadata faults cannot stop core catalog; no secret values in examples/errors/summaries.

## Review Focus
- All search filters bind cursors; Chinese intent cannot accidentally suggest write/delete for a read request.
- Samples preserve pointers/null/precise numbers and fail safely when missing/redacted/incompatible.
- Slow/failing metadata cannot overwrite new contracts or block core readiness.
- Invalid presentation and input make zero gateway calls; output clipping does not change billing/request.
- Error messages/result summaries reveal only safe structure, not credentials or undocumented semantics.

### Task 1: Public metadata and search
Files: catalog/metadata.go, search.go, snapshot.go, mcpserver/lite.go, cmd main and tests.
- [x] RED tests for metadata-backed Chinese matching/filter/cursor isolation and failures preserving original core.
- [x] Read a public-only metadata projection; enrich immutable snapshots without changing core hash. Preserve old Search caller with a filtered companion method.
- [x] Expose bounded optional filters/match explanation. Run catalog/full suite; commit.

### Task 2: API detail guidance
Files: catalog/guidance.go and tests.
- [ ] RED pointer/query/header samples, precise numbers/null, sensitive/invalid samples, missing metadata, required-field and pricing/response hints.
- [ ] Implement schema+encoder-checked examples, public pricing descriptions and bounded value-free shape hints. Run suites; commit.

### Task 3: Execution feedback and presentation
Files: schema/errors.go, mcpserver/output.go, lite.go, execute.go and tests.
- [ ] RED safe field errors and recovery actions; presentation validation/no-dispatch; clipping/summary retains request+fees and default raw data.
- [ ] Implement additive lite feedback and explicit output options. Keep direct error behavior and original data default. Run tests/race/vet; commit.

### Task 4: Docs, verification and review
- [ ] Update public docs and verification record.
- [ ] Validate offline metadata and optional read-only local catalog; no real tools/call to gateway.
- [ ] Separate self-review of complete diff, fix important issues with regressions.
- [ ] Leave isolated branch with reviewable commits and clean state; do not merge into parent or push.
