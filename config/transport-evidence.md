# Catalog transport evidence

Reviewed 2026-11-03 without making paid business requests.

- BlockRun states its prediction-market gateway mirrors the Predexon v2 data API: https://blockrun.ai/docs/api-reference/prediction-markets
- Source document: https://docs.predexon.com/openapi-v2.json (OpenAPI 3.1.0).
- All 46 stored array query inputs on eight API records were matched to the source parameter name and array schema. Parameters omit style/explode, so OpenAPI 3.1 query defaults are form and explode=true: repeated keys. Historical UUID overrides encode empty arrays as an empty value. The current source does not establish business behavior for that case; the new route policies use `empty: reject`. This is specification-derived, not a claimed paid end-to-end provider test.
- The non-keyset Polymarket market/event endpoints use the documented equivalent keyset filter contract; the current source OpenAPI only lists their /keyset counterparts. BlockRun documents both routes and equivalence. No route URL is rewritten by these overrides.
- OpenAPI serialization source: https://spec.openapis.org/oas/v3.1.0.html#parameter-object
- Gateway rules billing rejects repeated query keys. Catalog validation rejects this combination rather than silently forwarding a request that cannot meet its contract.
- Public descriptions are limited to business information. Historical QCC availability/schema uncertainty is retained. Provider credential instructions and supplier pricing are not advertised as caller instructions.
- Gang /api/stream remains discoverable with an explicit unsupported continuous-SSE result; it is never dispatched as an unbounded business stream.

Re-review these source-derived rules if the upstream document or API metadata changes.


## Cross-environment route rules (2026-11-05)

`routes` uses exact provider/method/path keys. A matching `apis` UUID entry overrides individual parameter policies; other route parameter policies remain in force. No wildcard matching or URL rewriting is performed. Restart the service after changing the override file; periodic refresh reads database metadata, not this file.

The audited current metadata contains 102 array parameters on 20 routes: BlockRun 8 routes/46 fields, LI.FI 4/24, Polymarket 7/31, Blockscout 1/1. Each field was matched to the provider specification and form/explode=true serialization (including OpenAPI defaults): repeated query keys. Eight explicit historical BlockRun `/api` variants preserve known route identities without inference. New rules use `empty: reject`; published `minItems: 1` retains stronger constraints. Existing UUID entries retain precedence and their original empty behavior.

- LI.FI source: https://docs.li.fi/openapi.yaml
- Polymarket source: https://docs.polymarket.com/api-spec/gamma-openapi.yaml
- Blockscout source: https://docs.blockscout.com/openapi-specs/pro-api-v12.json

The source URL and parameter-contract evidence are recorded in each rule's `source`. Cross-environment reuse requires the same provider, HTTP method, route and parameter contract. New or changed APIs still need contract review; an optional array without a known rule remains callable only when that parameter is omitted (or nullable null), with an explicit local error if an array is supplied.
