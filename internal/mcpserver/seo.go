package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/schema"
	"github.com/openany/openany-mcp/internal/seo"
)

func (h *Handler) newSEOServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: seo.RegistryName, Title: seo.RegistryTitle, Version: seo.ServerVersion}, &mcp.ServerOptions{
		Instructions:              seo.Instructions,
		Logger:                    h.options.Logger,
		Capabilities:              &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{ListChanged: false}},
		SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"},
	})
	type definition struct {
		name, title, description string
		inputs                   []model.Input
	}
	definitions := []definition{
		{"keyword_research", "Keyword volume and difficulty", seo.KeywordResearchDescription, []model.Input{
			schemaInput("keywords", true, `{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":80}}`, "Keywords to check. One to 100."),
			schemaInput("search_engine", false, `{"type":"string","enum":["google","bing"],"default":"google"}`, "google or bing. Default google. Bing does not return keyword difficulty."),
			schemaInput("location", false, `{"type":"string","minLength":1,"maxLength":120}`, "Location name, or a numeric location_code such as 2840. Defaults to 2840 when the route requires a location."),
			schemaInput("language", false, `{"type":"string","minLength":1,"maxLength":120}`, "Language name, or a language code such as en. Defaults to en when the route requires a language."),
			schemaInput("include_trend", false, `{"type":"boolean","default":false}`, "When true on Google, also price Historical Keyword Data for these same keywords. Not new-keyword discovery. Not available for Bing."),
			confirmInput(), priceVersionInput(),
		}},
		{"keyword_ideas", "Keyword ideas", seo.KeywordIdeasDescription, []model.Input{
			schemaInput("seeds", false, `{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":80}}`, "Seed keywords. mode related uses only the first seed."),
			schemaInput("site", false, `{"type":"string","minLength":1,"maxLength":500}`, "Domain or page URL. Used only when seeds are omitted. Returns related terms, not current rankings."),
			schemaInput("location", false, `{"type":"string","minLength":1,"maxLength":120}`, "Location name or numeric location_code. Defaults to 2840 when required."),
			schemaInput("language", false, `{"type":"string","minLength":1,"maxLength":120}`, "Language name or language code."),
			schemaInput("limit", false, `{"type":"integer","minimum":1,"maximum":100,"default":20}`, "Maximum keywords to return. Default 20."),
			schemaInput("mode", false, `{"type":"string","enum":["ideas","related"],"default":"ideas"}`, "ideas uses Google Keyword Ideas. related uses Google related searches for the first seed."),
			confirmInput(), priceVersionInput(),
		}},
		{"ranked_keywords", "Ranked keywords", seo.RankedKeywordsDescription, []model.Input{
			schemaInput("target", true, `{"type":"string","minLength":1,"maxLength":500}`, "Domain, subdomain, or page URL to inspect on Google."),
			schemaInput("location", false, `{"type":"string","minLength":1,"maxLength":120}`, "Optional location name or numeric location_code."),
			schemaInput("language", false, `{"type":"string","minLength":1,"maxLength":120}`, "Optional language name or language code."),
			schemaInput("limit", false, `{"type":"integer","minimum":1,"maximum":100,"default":20}`, "Maximum ranking keywords to return. Default 20."),
			schemaInput("positions", false, `{"type":"integer","minimum":1,"maximum":100}`, "Keep keywords with rank_group less than or equal to this number. 10 is page 1."),
			schemaInput("historical_serp_mode", false, `{"type":"string","enum":["live","lost","all"],"default":"live"}`, "live is current rankings. lost or all also return rankings that are no longer live."),
			confirmInput(), priceVersionInput(),
		}},
		{"serp_overview", "Page-1 SERP", seo.SERPOverviewDescription, []model.Input{
			schemaInput("keyword", true, `{"type":"string","minLength":1,"maxLength":700}`, "The keyword whose page-1 results you want."),
			schemaInput("search_engine", false, `{"type":"string","enum":["google","bing"],"default":"google"}`, "google or bing. Default google."),
			schemaInput("location", false, `{"type":"string","minLength":1,"maxLength":120}`, "Location name or numeric location_code. Defaults to 2840."),
			schemaInput("language", false, `{"type":"string","minLength":1,"maxLength":120}`, "Language name or language code. Defaults to en."),
			confirmInput(), priceVersionInput(),
		}},
	}
	for _, definition := range definitions {
		compiled, err := schema.Compile(model.Record{UnknownParameters: "reject", Inputs: definition.inputs})
		if err != nil {
			panic("invalid SEO MCP schema")
		}
		name := definition.name
		s.AddTool(&mcp.Tool{Name: name, Title: definition.title, Description: definition.description, InputSchema: compiled.Document, Annotations: executionAnnotations(definition.title)}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			raw := req.Params.Arguments
			if len(raw) == 0 {
				raw = json.RawMessage(`{}`)
			}
			args, err := compiled.Validate(raw)
			if err != nil {
				result := gateway.Failure("openany_mcp_invalid_input", "Parameters do not match this tool schema.", "validate", "not_started", nil, nil)
				result.Value["error"].(map[string]any)["details"] = schema.ValidationDetails(err)
				result.Value["error"].(map[string]any)["details_scope"] = "arguments"
				return toMCP(withRecovery(result)), nil
			}
			return h.seoTool(ctx, req, name, args), nil
		})
	}
	return s
}

func schemaInput(name string, required bool, doc, description string) model.Input {
	return model.Input{Name: name, Required: required, Description: description, Schema: json.RawMessage(doc)}
}

func confirmInput() model.Input {
	return schemaInput("confirm", false, `{"type":"boolean"}`, "Set true only after the user confirms the price_version from the unpaid response.")
}

func priceVersionInput() model.Input {
	return schemaInput("price_version", false, `{"type":"string","minLength":64,"maxLength":64,"pattern":"^[a-f0-9]{64}$"}`, "The price_version returned with the current price. Required when confirm is true.")
}

func (h *Handler) seoTool(ctx context.Context, req *mcp.CallToolRequest, name string, args map[string]any) *mcp.CallToolResult {
	plan, err := seo.Build(name, args)
	if err != nil {
		return toMCP(withRecovery(gateway.Failure("openany_mcp_invalid_input", err.Error(), "validate", "not_started", nil, nil)))
	}
	snapshot, _ := ctx.Value(snapshotKey{}).(*catalog.Snapshot)
	if snapshot == nil || snapshot.Discovery == nil {
		return toMCP(withRecovery(gateway.Failure("openany_mcp_catalog_unavailable", "Catalog pricing is not ready. No paid call was made.", "validate", "not_started", nil, nil)))
	}
	quotes := make([]map[string]any, 0, len(plan.Calls))
	versions := make([]string, 0, len(plan.Calls))
	bindings := make([]*catalog.Binding, 0, len(plan.Calls))
	bodies := make([]json.RawMessage, 0, len(plan.Calls))
	for _, call := range plan.Calls {
		binding := snapshot.Tools[call.API]
		detail, ok := snapshot.Discovery.Detail(call.API)
		version, _ := detail["api_version"].(string)
		if binding == nil || !ok || version == "" {
			return toMCP(withRecovery(gateway.Failure("openany_mcp_api_unavailable", "DataForSEO route "+call.API+" is not in the current catalog. No paid call was made.", "validate", "not_started", nil, nil)))
		}
		body, err := json.Marshal(map[string]any{"body": call.Tasks})
		if err != nil {
			return toMCP(withRecovery(gateway.Failure("openany_mcp_invalid_input", "Could not encode the DataForSEO request.", "validate", "not_started", nil, nil)))
		}
		if _, err = binding.Input.Validate(body); err != nil {
			result := gateway.Failure("openany_mcp_invalid_input", "The DataForSEO request does not match the current catalog schema. No paid call was made.", "validate", "not_started", nil, nil)
			result.Value["error"].(map[string]any)["details"] = schema.ValidationDetails(err)
			result.Value["error"].(map[string]any)["details_scope"] = "input"
			return toMCP(withRecovery(result))
		}
		pricing, _ := detail["pricing"].(map[string]any)
		quotes = append(quotes, map[string]any{
			"api": call.API, "purpose": call.Purpose, "note": call.Note, "api_version": version,
			"pricing": pricing, "price_summary": detail["price_summary"], "tariff": seo.Tariff(pricing, call.Units),
			"input": map[string]any{"body": call.Tasks},
		})
		versions = append(versions, version)
		bindings = append(bindings, binding)
		bodies = append(bodies, body)
	}
	priceVersion, err := seo.PriceVersion(plan.Calls, versions)
	if err != nil {
		return toMCP(withRecovery(gateway.Failure("openany_mcp_invalid_input", "Could not fingerprint the current price.", "validate", "not_started", nil, nil)))
	}
	confirmed, _ := args["confirm"].(bool)
	if !confirmed {
		value := map[string]any{
			"confirmed": false, "execution_state": "not_started", "is_quote": false,
			"message": seo.QuoteMessage, "next_action": seo.QuoteMessage, "price_version": priceVersion,
			"tool": name, "resolved": plan.Resolved, "notes": plan.Notes, "calls": quotes,
		}
		if total, ok := bundleMicro(quotes); ok {
			value["bundle_published_tariff_maximum_micro_usd"] = total
		}
		return toMCP(gateway.Result{Value: value})
	}
	supplied, _ := args["price_version"].(string)
	if supplied == "" {
		return toMCP(withRecovery(gateway.Failure("openany_mcp_invalid_input", "confirm requires the price_version from the unpaid price response. No paid call was made.", "validate", "not_started", nil, nil)))
	}
	if supplied != priceVersion {
		return toMCP(withRecovery(gateway.Failure("openany_mcp_api_changed", "Displayed price or request changed; call again without confirm to read the current price. No paid call was made.", "validate", "not_started", nil, nil)))
	}
	results := make([]any, 0, len(plan.Calls))
	stopped := false
	for i, call := range plan.Calls {
		if stopped {
			results = append(results, map[string]any{"api": call.API, "purpose": call.Purpose, "execution_state": "not_started", "skipped": true, "message": "Not called because an earlier call in this confirmed request did not finish cleanly."})
			continue
		}
		upstream := withRecovery(h.executeResult(ctx, req, bindings[i], name, bodies[i], snapshot.Hash, true))
		state := executionState(upstream)
		row := map[string]any{"api": call.API, "purpose": call.Purpose, "execution_state": state, "http_status": upstream.Value["http_status"], "request_id": upstream.Value["request_id"]}
		if upstream.IsError {
			row["error"] = upstream.Value["error"]
			results = append(results, row)
			if state == "unknown" || state == "started" || state == "not_started" {
				stopped = true
			}
			continue
		}
		if projected, ok := seo.Project(call.API, upstream.Value["data"]); ok {
			row["result"] = projected
		} else {
			row["result"] = upstream.Value["data"]
		}
		results = append(results, row)
	}
	value := map[string]any{"confirmed": true, "is_quote": false, "tool": name, "price_version": priceVersion, "resolved": plan.Resolved, "notes": plan.Notes, "calls": results, "execution_state": bundleState(results)}
	isError := value["execution_state"] != "completed"
	if isError {
		value["next_action"] = "Do not retry a call whose execution_state is unknown or completed. Read request_id before deciding."
	}
	return toMCP(gateway.Result{Value: value, IsError: isError})
}

func executionState(result gateway.Result) string {
	if e, ok := result.Value["error"].(map[string]any); ok {
		if state, _ := e["execution_state"].(string); state != "" {
			return state
		}
	}
	if result.IsError {
		return "unknown"
	}
	return "completed"
}

func bundleState(results []any) string {
	sawFailure := false
	for _, raw := range results {
		row, _ := raw.(map[string]any)
		switch row["execution_state"] {
		case "unknown", "started":
			return "unknown"
		case "reported_failure":
			sawFailure = true
		case "completed":
		default:
			return "not_started"
		}
	}
	if sawFailure {
		return "reported_failure"
	}
	return "completed"
}

func bundleMicro(quotes []map[string]any) (string, bool) {
	total := 0
	for _, quote := range quotes {
		tariff, _ := quote["tariff"].(map[string]any)
		raw, _ := tariff["published_tariff_maximum_micro_usd"].(string)
		if raw == "" || strings.Trim(raw, "0123456789") != "" {
			return "", false
		}
		n := 0
		for _, c := range raw {
			n = n*10 + int(c-'0')
		}
		total += n
	}
	if total == 0 {
		return "", false
	}
	digits := []byte{}
	for n := total; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits), true
}

func (h *Handler) serveSEO(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == serverCardPath {
		serveSEOServerCard(w, r)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && !slices.Contains(h.options.Origins, origin) {
		http.Error(w, "Forbidden origin", http.StatusForbidden)
		return
	}
	if r.URL.Path == "/docs/mcp" || (r.URL.Path == "/" && humanRead(r)) {
		h.serveSEOGuide(w, r)
		return
	}
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/readyz" {
			if !h.manager.Ready() {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			json.NewEncoder(w).Encode(h.manager.Status())
		} else {
			w.Write([]byte(`{"ok":true}`))
		}
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	var snapshot *catalog.Snapshot
	if h.manager.Ready() {
		snapshot = h.manager.Current()
	}
	ctx := context.WithValue(r.Context(), snapshotKey{}, snapshot)
	h.sdk.ServeHTTP(w, r.WithContext(context.WithValue(ctx, serverKey{}, h.seo)))
}

func (h *Handler) serveSEOGuide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		w.Write([]byte(seo.Guide))
	}
}

var seoServerCardBody, seoServerCardETag = encodeServerCard(serverCardDocument{
	Schema:      serverCardSchemaURL,
	Name:        seo.RegistryName,
	Title:       seo.RegistryTitle,
	Version:     seo.ServerVersion,
	Description: seo.RegistryDescription,
	WebsiteURL:  "https://openany.si",
	Repository:  serverCardRepo{URL: "https://github.com/openanysi/openany-mcp", Source: "github"},
	Remotes: []serverCardRemote{{
		Type: "streamable-http", URL: seo.Endpoint,
		SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"},
		Headers: []serverCardHeader{{
			Name: "Authorization", Description: "OpenAny API key as Bearer token. Optional for the price check; required before a confirmed paid call.",
			IsRequired: false, IsSecret: true, Value: "Bearer {OPENANY_API_KEY}",
			Variables: map[string]serverCardInput{"OPENANY_API_KEY": {Description: "OpenAny API key", IsRequired: false, IsSecret: true}},
		}},
	}},
})

func serveSEOServerCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match")
	w.Header().Set("Access-Control-Expose-Headers", "ETag")
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodGet, http.MethodHead:
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", serverCardMediaType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", seoServerCardETag)
	if serverCardNoneMatch(r.Header.Get("If-None-Match"), seoServerCardETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(seoServerCardBody)))
	if r.Method == http.MethodGet {
		w.Write(seoServerCardBody)
	}
}
