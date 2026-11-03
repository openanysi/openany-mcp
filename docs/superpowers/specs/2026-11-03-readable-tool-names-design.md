# Readable public MCP tool names

User-confirmed change, 2026-11-03: remove the automatic UUID suffix, use `_2`, `_3`, … only for collisions, and adjust existing script-generated names too.

## Approaches and decision

Keeping UUID suffixes preserves the old format but does not meet the readability request. Manual aliases require operator input for every collision. The selected approach uses the normalized provider/method/path as the name, with numeric aliases only when needed.

## Contract

- Normalize exactly as before: lowercase ASCII letters/digits, other runs become underscores, trim surrounding underscores. Names must start with a letter and fit the existing 64-character database constraint.
- Use the first 64 characters (trim trailing underscores) without a suffix when available. For collisions, try `_2`, `_3`, …, truncating the base to leave room for the suffix. Check every candidate against all reserved and newly assigned names, including disabled APIs.
- Assign a batch in sorted API-ID order. Existing stored names remain stable on subsequent normal runs; a later API never displaces an earlier stored name.
- Normal execution continues to fill NULL only. Explicit `--rename-generated` opts into renaming names that exactly match the previous algorithm for their current provider, method, path and UUID (8/12/16/32 digits). Other names are preserved. A manually selected value identical to the old algorithm cannot be distinguished without historical provenance.
- Default remains read-only preview. Preview/apply output includes each changed old/new name. Applying uses the existing advisory lock plus row locks. Renames first clear only the expected old values inside the transaction, then write all final values; readers never observe the intermediate NULLs. Any conflict/error rolls back the batch.
- Save private before/after reports locally, perform the user-authorized local migration, and verify names, idempotence and runtime discovery. Existing Agent references must be re-discovered after this intentional rename.
- No Go runtime change, schema migration, new dependency, or change to upstream `mcp_tool_name`, pricing or API status.
