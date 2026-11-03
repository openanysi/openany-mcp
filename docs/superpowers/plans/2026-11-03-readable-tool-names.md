# Readable MCP Names Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution of this small maintenance change.

**Goal:** Readable default names and user-authorized migration of legacy generated names.
**Architecture:** Keep persistent database names and the existing transaction/locks; add an explicit legacy-renaming switch to the Python CLI. Go continues to read names unchanged.
**Tech Stack:** Python 3.12, psycopg 3, existing PostgreSQL constraints.
**Spec:** docs/superpowers/specs/2026-11-03-readable-tool-names-design.md

## Global Constraints

- Only numeric collision aliases; 64 ASCII characters maximum.
- Preserve manual names and default NULL-only behavior.
- Read-only preview, atomic apply, expected-value updates, no changes to price/status/upstream tool names.

## Review Focus

- Alias collision with existing natural or numbered names, including disabled APIs.
- Truncation leaving room for multi-digit aliases; input-order independence.
- Opt-in recognition of all four legacy suffix lengths without suffix-only stripping.
- Atomic migration when a final name currently belongs to another legacy row.
- Concurrent runs and failure rollback, including restoration of temporarily cleared old names.

### Task 1: Script, tests and migration

**Files:** scripts/sync_mcp_tool_names.py; scripts/test_sync_mcp_tool_names.py; scripts/test_database.py; README.md; docs/verification.md; .gitignore.

- [x] Write naming and real-database regression tests, run against the old implementation and observe failure.
- [x] Implement natural names, numeric aliases, explicit legacy rename and atomic conditional updates.
- [x] Run all Python tests on a disposable PostgreSQL database, and the existing Go suite.
- [x] Update operational documentation; preview the real catalog and save private old/new reports.
- [x] Apply the approved local migration; verify the example name, uniqueness, idempotence, unchanged non-name metadata and refreshed discovery.
- [x] Review the final diff and save a local commit, without remote push.
