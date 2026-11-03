# Script-local database configuration

Approved in chat, 2026-11-03: put DATABASE_URL at the top of sync_mcp_tool_names.py so editing one variable selects the maintenance database. Keep default preview and --apply writes.

- DATABASE_URL defaults to an empty string. Read only this variable for CLI selection, strip surrounding whitespace and fail clearly when empty. Do not fall back to POSTGRES_DATABASE_URL; it remains the separate Go service configuration.
- Keep synchronize(database_url, apply=False, rename_generated=False) and naming/transaction/locking behavior intact.
- Successful result JSON includes target.host, target.port, target.database, target.schema=public, target.table=agent_apis, derived from the connected driver's info. Never emit the full connection string or password.
- Keep the existing prerequisite schema checks and explicit legacy rename switch. No automatic DDL or real catalog writes are required for this code change.
- Update maintenance instructions and verify empty config, explicit selection despite a misleading environment, preview/apply/idempotence and sanitized output against disposable PostgreSQL.
