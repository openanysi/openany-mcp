# MCP usability fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make existing MCP tools return finite text results, rank business intent reliably, and offer honest usable request guidance.

**Architecture:** Extend the shared gateway decoder and existing in-memory discovery/guidance. Keep request contracts and public tool identities stable; distinguish templates from complete requests using existing metadata.

**Tech Stack:** Go standard library, existing MCP SDK, existing JSON Schema compiler, httptest.

**Spec:** docs/superpowers/specs/2026-11-06-mcp-usability-fixes-design.md

## Global Constraints

- Preserve the three tools, endpoint paths, public tool names and existing JSON result shape.
- Preserve credential forwarding, gateway billing, readonly database access, byte/result limits and no-replay behavior.
- No new dependencies, migrations, remote deployment or remote business calls.
- Keep unrelated primary-checkout documents untouched.

## Review Focus

- Unsupported or invalid response bodies must not leak HTML/private content or trigger a replay (Task 1).
- A short Latin word must not match inside an unrelated token; synonyms must not dominate via duplicated fields (Task 2).
- A search cursor must not cross temporal intent or ranking versions (Task 2).
- Valid empty requests must not bypass required/conditional inputs or stale revision checks (Task 3).
- Returning a sample detail must not expose credentials or permit snapshot mutation (Task 3).

### Task 1: Finite text responses

**Files:** internal/gateway/client.go, internal/gateway/client_test.go, internal/mcpserver/output.go, internal/mcpserver/output_test.go

**Interfaces:** Keep Client.Call(ctx, model.Request, map[string]any, http.Header) Result; successful text uses string Value["data"]. Keep present(gateway.Result, outputOptions) gateway.Result.

- [ ] Write tests for text/plain and Markdown (including Chinese), empty text, invalid UTF-8/charset, invalid JSON, unsupported HTML/SSE, byte/result limits and unchanged Accept/auth/no replay. Test string raw/summary/compact presentation.
- [ ] Run targeted tests; expect new text-success and format diagnostics assertions to fail before implementation.
- [ ] Implement allowlisted media decoding with bounded diagnostics and string structural length; keep existing errors and execution-state semantics.
- [ ] Run gateway and mcpserver tests; expect PASS, then commit.

### Task 2: Business intent ranking

**Files:** internal/catalog/search.go, internal/catalog/search_intent_test.go

**Interfaces:** Keep Discovery.SearchFiltered(SearchQuery) (SearchResult, error) and MatchReason fields. Ranking remains keyword; introduce a ranking-version constant into discovery hash/cursor validation without changing execution versions.

- [ ] Write at least twenty fixed search cases covering latest/hot/history/news, Codex-specific queries, quote/address lookups, exact tools, filters, metadata absence, Latin boundaries, duplicated metadata and read/write intent. Add cursor intent/ranking regression checks.
- [ ] Run targeted tests; expect temporal/hot ranking and substring cases to fail.
- [ ] Implement preserved aliases/concepts, strongest-field scoring and coverage/intent ordering without per-API pinning.
- [ ] Run all catalog tests; expect PASS, then commit.

### Task 3: Honest complete examples

**Files:** internal/catalog/guidance.go, internal/catalog/guidance_test.go, internal/catalog/usability_test.go

**Interfaces:** Keep existing examples envelope and supplier_verified=false. Add per-item kind=example|template|empty_request and placeholder_paths for templates; use status=template_only if no complete request. Add supported_media_types to response_hint as service capabilities.

- [ ] Change minimal-example regression to require optional business parameter preservation. Add tests for empty fallback, required/conditional rejection, stale samples, nested placeholders, invalid templates, complete-over-template ordering and independent cloned outputs.
- [ ] Run targeted tests; expect optional preservation/classification/fallback/mutation assertions to fail.
- [ ] Remove pruning; clone projected inputs, classify bounded template placeholders and validate empty fallback with the existing schema/encoder. Keep original metadata immutable.
- [ ] Run all catalog tests; expect PASS, then commit.

### Task 4: Integration, docs and delivery

**Files:** internal/mcpserver/usability_test.go, internal/mcpserver/lite.go, docs/mcp.md, README.md, docs/verification.md

**Interfaces:** Consume string gateway results and the guidance envelope from Tasks 1 and 3 through existing tools/call and direct entrances.

- [ ] Add a mock-gateway text workflow through lite and direct endpoints, asserting unchanged credentials and single dispatch. Verify guidance and output errors reach MCP clients.
- [ ] Run the new integration before implementation-dependent changes as applicable, then update tool descriptions and docs to the actual contract.
- [ ] Run go test ./..., go test -race ./..., go vet ./...; expect PASS. Record limitations: no live content-type diagnosis, supplier-data backfill, deployment or Grok UI verification.
- [ ] Obtain one independent whole-change review, fix any material findings with regression tests, and verify affected checks again.
- [ ] Commit and fast-forward primary dev only if tracked state and base remain unchanged; preserve unrelated untracked documents. Verify primary branch status and built binary before delivery.
