# Catalog transport evidence

Reviewed 2026-11-03 without making paid business requests.

- BlockRun states its prediction-market gateway mirrors the Predexon v2 data API: https://blockrun.ai/docs/api-reference/prediction-markets
- Source document: https://docs.predexon.com/openapi-v2.json (OpenAPI 3.1.0).
- All 46 stored array query inputs on eight API records were matched to the source parameter name and array schema. Parameters omit style/explode, so OpenAPI 3.1 query defaults are form and explode=true: repeated keys. Empty arrays use the form empty value. This is specification-derived, not a claimed paid end-to-end provider test.
- The non-keyset Polymarket market/event endpoints use the documented equivalent keyset filter contract; the current source OpenAPI only lists their /keyset counterparts. BlockRun documents both routes and equivalence. No route URL is rewritten by these overrides.
- OpenAPI serialization source: https://spec.openapis.org/oas/v3.1.0.html#parameter-object
- Gateway rules billing rejects repeated query keys. Catalog validation rejects this combination rather than silently forwarding a request that cannot meet its contract.
- Public descriptions are limited to business information. Historical QCC availability/schema uncertainty is retained. Provider credential instructions and supplier pricing are not advertised as caller instructions.
- Gang /api/stream remains discoverable with an explicit unsupported continuous-SSE result; it is never dispatched as an unbounded business stream.

Re-review these source-derived rules if the upstream document or API metadata changes.
