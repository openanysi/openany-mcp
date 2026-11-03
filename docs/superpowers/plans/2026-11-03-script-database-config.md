# Script Database Configuration Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution.

**Goal:** Edit one top-level DATABASE_URL to select the name-maintenance database.
**Architecture:** Replace CLI environment lookup with the approved constant; add sanitized actual-connection target metadata to existing results. Leave SQL behavior intact.
**Tech Stack:** Existing Python/psycopg and PostgreSQL.
**Spec:** docs/superpowers/specs/2026-11-03-script-database-config-design.md

## Global Constraints

- Empty default, no environment fallback, preview by default, writes only with --apply.
- Fixed public.agent_apis; no new dependencies or automatic migration; no real database writes for verification.
- Never emit full connection strings/passwords or commit a real configured connection string.

## Review Focus

- Environment or stale configuration silently selecting another database.
- Preview performing writes or apply bypassing existing atomicity.
- Target metadata differing from the actual server connection.
- Secret leakage through output or new error handling.
- Existing README commands still telling users to select this script via environment.

### Task 1: CLI configuration and target metadata

**Files:** scripts/sync_mcp_tool_names.py, scripts/test_sync_mcp_tool_names.py, scripts/test_database.py, README.md, docs/verification.md.

- [x] Add CLI/config and real-database regressions; observe RED.
- [x] Implement the constant and sanitized target output; run full Python suite with disposable PostgreSQL.
- [x] Verify actual edit-file-and-run workflow against two isolated databases and misleading environment values.
- [x] Update instructions; review once; verify remaining project checks.
- [x] Save local commits and verification evidence without remote push.
