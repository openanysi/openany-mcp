# Search recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct search error next_action guidance and regress existing execution-state behavior.

**Architecture:** Extend the existing withRecovery function using error code and details_scope; keep execution-state handling first and keep the public result envelope/error codes unchanged.

**Tech Stack:** Go standard library and existing MCP mock-gateway tests.

**Spec:** Approved in chat: minimal search-error guidance fix, regression of other branches. A search error must direct the caller to search_apis, omit an invalid cursor and restart; tool-argument errors use the current tool inputSchema, API-input errors use get_api.input_schema. Unknown/completed executions never encourage resubmission.

## Global Constraints

- Preserve search ranking, database data, tool names, error codes and execution states.
- No new protocol fields, dependencies, database writes, remote business calls or deployment.
- Preserve unrelated untracked documents and synchronize the verified local fix to primary dev.

## Review Focus

- Search limit validation fails before the search implementation; details_scope=arguments must not lead to API business schema.
- Search cursor and missing-criteria errors share invalid_search and both need actionable search guidance.
- Unknown/completed/reported_failure state handling must override error-code hints.
- Correcting a discovery request must not dispatch any business request.
- Existing API-input, version, output and admission hints must remain correct.

### Task 1: Search recovery and regression

**Files:** internal/mcpserver/output.go, internal/mcpserver/output_test.go, internal/mcpserver/usability_test.go, docs/mcp.md, docs/verification.md

**Interfaces:** Preserve withRecovery(gateway.Result) gateway.Result and the existing serialized/structured MCP result.

- [ ] Add failing recovery-table tests and a bad search cursor/empty criteria/limit correction cycle to the existing mock MCP workflow. Assert expected next tool/schema, unchanged code/state and zero business calls.
- [ ] Run targeted tests and observe the current wrong get_api guidance (RED).
- [ ] Add an invalid_search next_action and use details_scope=arguments to guide outer tool argument validation without changing API-input guidance.
- [ ] Run targeted/full tests, race and vet (GREEN); update connection docs and record actual checks.
- [ ] Obtain independent read-only review; fix material findings, commit, fast-forward primary dev after checking its base/tracked state, rebuild the local binary and verify primary tests. Do not push or deploy as part of this scope.
