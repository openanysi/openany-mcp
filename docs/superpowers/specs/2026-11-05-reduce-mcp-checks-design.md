# Reduce MCP catalog checks

User-approved scope: description review becomes a warning; support gateway HTTP methods and normalize bodyless GET metadata; preserve legacy nullable enum semantics; resolve array policies across database environments.

## Decisions

- Implement in the standalone Go MCP service. Keep the Python naming script, database, tool names, gateway origin, caller credential forwarding and billing behavior unchanged.
- Allow GET/POST/PUT/PATCH/DELETE with no body, and JSON on non-GET methods. Normalize GET JSON to `none` only when no body input exists. Unsupported encodings and GET body inputs remain errors.
- Unreviewed descriptions matching the existing review keywords use a safe API/input title and emit a warning; reviewed overrides remain usable. Warnings do not quarantine tools or fail the catalog CLI.
- For legacy inputs without explicit JSON Schema, nullable enum values include null in the generated schema. Do not relax explicit schema enums or remove other validation.
- Add exact provider/method/path route overrides alongside existing API UUID overrides. UUID overrides take precedence, with per-parameter array policies merged without modifying source maps. Reject duplicate route policies.
- Ship source-backed policies for the 20 audited routes, plus explicit historical BlockRun path variants. Do not rewrite HTTP routes or guess formats for unknown providers.
- Missing policies on optional query arrays produce warnings and a documented pre-dispatch error only if a real array value is supplied. Missing required policies and invalid configured policies remain errors. Null omission is preserved.
- New policies without verified empty-array behavior use `empty: reject`; advertise `minItems: 1` while retaining stronger existing limits and reject before dispatch. Existing explicit empty/omit policies remain compatible.
- Report warning counts and safe issue identity in readiness/CLI diagnostics. Log diagnostics only when the catalog generation changes; never log source descriptions, credentials or argument values.

## Acceptance

The audited local snapshot has 740 active APIs: target 700 published, 40 pending and zero quarantined, with warnings explaining limitations. This is not proof of production's 118 APIs or supplier business behavior: the production attachment contains reasons only, without full request metadata.

Use regression tests for legacy versus explicit enums, all supported methods, description fallback, route policy precedence/isolation, optional unknown arrays, required unknown arrays, safe empty-array handling, credential boundaries and no gateway dispatch on encoding errors. Run the full Go suite/race/vet, independent review and read-only local catalog checks before integration.
