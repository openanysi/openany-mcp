# MCP usability fixes

Approved scope: improve response compatibility, discovery ranking and request examples inside the existing Go service. Preserve the three tools, all endpoint paths, public tool names, caller credential forwarding, gateway billing, readonly database access and current JSON result shape.

## Response compatibility

Keep JSON and +json decoding. Also accept finite UTF-8 text/plain and text/markdown responses as string data in the same result envelope. Keep Accept, byte/result limits, timeouts and no-replay behavior. Unsupported media, invalid charset/UTF-8 and invalid JSON remain explicit errors with HTTP status, request ID and bounded actual media type, never raw response bodies. HTML, binary and SSE remain unsupported. Summary is structural, not an LLM interpretation; compact on a string keeps the existing completed/output-unavailable result with original data.

## Discovery ranking

Keep deterministic keyword search with bounded results. Preserve latest/today/hot/history intent through bilingual aliases, match complete Latin words rather than substrings, score each business concept once using its strongest field, prioritize concept coverage and meaningful title/purpose matches over broad categories. Keep explicit filters, exact tool-name lookup, read-intent write screening and metadata degradation. Search cursors must bind to the ranking version as well as all query/filter/intent values; display/ranking changes must not alter direct catalog hashes or per-API execution versions.

## Guidance

Preserve optional business values in public samples instead of pruning fields merely because schema validation permits it. Clone projected bodies so public metadata cannot be mutated through a detail result. Continue credential filtering, current-schema and HTTP-encoder validation, bounded source samples and stale revision rejection. Label samples with placeholders as templates and list their placeholder paths. Complete examples, empty requests and templates are distinguishable; template-only is not available. If no valid sample exists and the empty object passes current schema and encoding, provide an explicit empty_request example unless source samples are stale. Never fabricate business values or claim supplier verification. Expose globally supported response media as capabilities, not an API-specific response guarantee. Maintain required-input guidance.

## Data and verification

Reuse existing samples. Automatically deriving a valid empty request and labeling templates are code changes; filling supplier-specific descriptions/examples is catalog data work and must not fabricate evidence. No migrations, new infrastructure or remote business calls are needed for this implementation. Add mock-gateway integration coverage and a fixed set of at least twenty search cases. Run the full Go suite, race and vet. Preserve other pending work in the primary checkout; synchronize the verified changes to its current dev branch without publishing remotely.
