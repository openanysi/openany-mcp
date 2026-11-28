package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/seo"
)

func TestSEOServerQuotesBeforeCallingAndStaysOffTheRoot(t *testing.T) {
	var mu sync.Mutex
	var seen []map[string]string
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, map[string]string{"path": r.URL.Path, "body": string(body), "auth": r.Header.Get("Authorization")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tasks":[{"result":[{"items":[{"keyword":"email finder api","keyword_info":{"search_volume":1000},"keyword_properties":{"keyword_difficulty":42}}]}]}]}`))
	}))
	defer gw.Close()
	client, err := gateway.New(gw.URL, time.Second, time.Second, 1<<20, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	manager := seoManager(t)
	handler := New(manager, client, Options{LiteEnabled: true, Product: "seo"})
	server := httptest.NewServer(handler)
	defer server.Close()

	list := seoCall(t, server.URL, "/", "tools/list", map[string]any{}, "")
	tools := list["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 4 {
		t.Fatal(len(tools))
	}
	names := map[string]bool{}
	for _, item := range tools {
		tool := item.(map[string]any)
		name := tool["name"].(string)
		names[name] = true
		description := tool["description"].(string)
		if !strings.Contains(description, "Use when") || !strings.Contains(description, "Do NOT") || !strings.Contains(description, "DataForSEO") {
			t.Fatal("tool description is missing the routing copy", name)
		}
		for _, banned := range []string{"search_apis", "get_api", "run_api", "openany_overview"} {
			if strings.Contains(description, banned) {
				t.Fatal(name, "names a root tool")
			}
		}
	}
	for _, name := range []string{"keyword_research", "keyword_ideas", "ranked_keywords", "serp_overview"} {
		if !names[name] {
			t.Fatal("missing", name)
		}
	}
	root := New(manager, client, Options{LiteEnabled: true})
	rootServer := httptest.NewServer(root)
	defer rootServer.Close()
	rootList := seoCall(t, rootServer.URL, "/mcp-lite", "tools/list", map[string]any{}, "")
	for _, item := range rootList["result"].(map[string]any)["tools"].([]any) {
		if item.(map[string]any)["name"] == "keyword_research" {
			t.Fatal("root MCP gained the SEO tool")
		}
	}

	quote := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}}, "")
	content := quote["structuredContent"].(map[string]any)
	if quote["isError"] == true || content["confirmed"] != false || content["execution_state"] != "not_started" || content["is_quote"] != false {
		t.Fatal(quote)
	}
	if content["price_version"] == "" || !strings.Contains(seo.QuoteMessage, "No paid upstream call") {
		t.Fatal(content)
	}
	calls := content["calls"].([]any)
	if len(calls) != 1 || calls[0].(map[string]any)["api"] != seo.GoogleOverview {
		t.Fatal(calls)
	}
	tariff := calls[0].(map[string]any)["tariff"].(map[string]any)
	if tariff["is_quote"] != false || tariff["base_usd"] != "0.012" || tariff["per_result_usd"] != "0.00012" {
		t.Fatal(tariff)
	}
	mu.Lock()
	if len(seen) != 0 {
		t.Fatal("price check called the gateway", seen)
	}
	mu.Unlock()

	denied := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}, "confirm": true, "price_version": content["price_version"]}, "")
	if denied["isError"] != true || !strings.Contains(mustJSON(denied), "openany_mcp_credentials_missing") {
		t.Fatal(denied)
	}
	mu.Lock()
	if len(seen) != 0 {
		t.Fatal("missing key reached the gateway", seen)
	}
	mu.Unlock()

	stale := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}, "confirm": true, "price_version": strings.Repeat("ab", 32)}, "Bearer seo-key")
	if stale["isError"] != true || !strings.Contains(mustJSON(stale), "openany_mcp_api_changed") {
		t.Fatal(stale)
	}
	mu.Lock()
	if len(seen) != 0 {
		t.Fatal("stale price reached the gateway", seen)
	}
	mu.Unlock()

	paid := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}, "confirm": true, "price_version": content["price_version"]}, "Bearer seo-key")
	if paid["isError"] == true {
		t.Fatal(paid)
	}
	paidContent := paid["structuredContent"].(map[string]any)
	if paidContent["execution_state"] != "completed" {
		t.Fatal(paidContent)
	}
	result := paidContent["calls"].([]any)[0].(map[string]any)["result"].(map[string]any)
	row := result["keywords"].([]any)[0].(map[string]any)
	if row["keyword_difficulty"] != json.Number("42") && row["keyword_difficulty"] != float64(42) {
		t.Fatal(row)
	}
	mu.Lock()
	if len(seen) != 1 || seen[0]["auth"] != "Bearer seo-key" || seen[0]["path"] != "/v1/agent-apis/dataforseo/v3/dataforseo_labs/google/keyword_overview/live" {
		t.Fatal(seen)
	}
	if !strings.Contains(seen[0]["body"], `"keywords":["email finder API"]`) || !strings.Contains(seen[0]["body"], `"location_code":2840`) || strings.HasPrefix(seen[0]["body"], "{") {
		t.Fatal(seen[0]["body"])
	}
	mu.Unlock()

	bing := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}, "search_engine": "bing"}, "")
	bingCalls := bing["structuredContent"].(map[string]any)["calls"].([]any)
	if bingCalls[0].(map[string]any)["api"] != seo.BingVolume {
		t.Fatal(bingCalls)
	}
	site := seoTool(t, server.URL, "keyword_ideas", map[string]any{"site": "https://www.example.com/pricing"}, "")
	siteCall := site["structuredContent"].(map[string]any)["calls"].([]any)[0].(map[string]any)
	if siteCall["api"] != seo.GoogleForSite || !strings.Contains(mustJSON(siteCall["input"]), "https://www.example.com/pricing") {
		t.Fatal(siteCall)
	}
	ranked := seoTool(t, server.URL, "ranked_keywords", map[string]any{"target": "www.example.com", "positions": 10}, "")
	rankedCall := ranked["structuredContent"].(map[string]any)["calls"].([]any)[0].(map[string]any)
	if rankedCall["api"] != seo.GoogleRanked || !strings.Contains(mustJSON(rankedCall["input"]), "rank_group") {
		t.Fatal(rankedCall)
	}
	serp := seoTool(t, server.URL, "serp_overview", map[string]any{"keyword": "email finder API", "search_engine": "bing"}, "")
	if serp["structuredContent"].(map[string]any)["calls"].([]any)[0].(map[string]any)["api"] != seo.BingSERP {
		t.Fatal(serp)
	}
	mu.Lock()
	if len(seen) != 1 {
		t.Fatal("later quotes called the gateway", seen)
	}
	mu.Unlock()

	res, err := http.Get(server.URL + "/mcp-lite")
	if err != nil || res.StatusCode != 404 {
		t.Fatal("seo server exposed the root lite path", res.StatusCode, err)
	}
	res.Body.Close()
	guide, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	guideBody, _ := io.ReadAll(guide.Body)
	guide.Body.Close()
	if guide.StatusCode != 200 || !strings.Contains(string(guideBody), "这个词该不该做") || !strings.Contains(string(guideBody), "https://seo.openany.si") {
		t.Fatal("human guide missing the SEO hook", guide.StatusCode)
	}
	cardRes, err := http.Get(server.URL + "/server-card")
	if err != nil {
		t.Fatal(err)
	}
	var card serverCardDocument
	if json.NewDecoder(cardRes.Body).Decode(&card) != nil || card.Name != seo.RegistryName || card.Title != seo.RegistryTitle || card.Description != seo.RegistryDescription {
		t.Fatal(card)
	}
	cardRes.Body.Close()
	if len(card.Remotes) != 1 || card.Remotes[0].URL != seo.Endpoint {
		t.Fatal(card.Remotes)
	}

	foreign := httptest.NewRequest(http.MethodGet, server.URL+"/", nil)
	foreign.Host = "mcp.openany.si"
	foreign.Header.Set("Accept", "text/html")
	locked := New(manager, client, Options{Product: "seo", Hosts: []string{"seo.openany.si"}})
	recorded := httptest.NewRecorder()
	locked.ServeHTTP(recorded, foreign)
	if recorded.Code != 403 {
		t.Fatal("seo host allowlist ignored", recorded.Code)
	}
}

func TestSEODoesNotStartTheNextPaidCallAfterAnUnknownResult(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path)
		mu.Unlock()
		time.Sleep(time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer gw.Close()
	client, err := gateway.New(gw.URL, 200*time.Millisecond, 200*time.Millisecond, 1<<20, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := httptest.NewServer(New(seoManager(t), client, Options{Product: "seo"}))
	defer server.Close()
	quote := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}, "include_trend": true}, "")
	version := quote["structuredContent"].(map[string]any)["price_version"].(string)
	if len(quote["structuredContent"].(map[string]any)["calls"].([]any)) != 2 {
		t.Fatal("trend did not add the historical call to the price")
	}
	paid := seoTool(t, server.URL, "keyword_research", map[string]any{"keywords": []any{"email finder API"}, "include_trend": true, "confirm": true, "price_version": version}, "Bearer seo-key")
	content := paid["structuredContent"].(map[string]any)
	if content["execution_state"] != "unknown" {
		t.Fatal(content["execution_state"], paid)
	}
	calls := content["calls"].([]any)
	if len(calls) != 2 || calls[1].(map[string]any)["skipped"] != true || calls[1].(map[string]any)["execution_state"] != "not_started" {
		t.Fatal(calls)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || !strings.Contains(seen[0], "keyword_overview") {
		t.Fatal(seen)
	}
}

func seoManager(t *testing.T) *catalog.Manager {
	t.Helper()
	routes := map[string]string{
		seo.GoogleOverview:   "/v3/dataforseo_labs/google/keyword_overview/live",
		seo.GoogleHistorical: "/v3/dataforseo_labs/google/historical_keyword_data/live",
		seo.GoogleIdeas:      "/v3/dataforseo_labs/google/keyword_ideas/live",
		seo.GoogleRelated:    "/v3/dataforseo_labs/google/related_keywords/live",
		seo.GoogleForSite:    "/v3/dataforseo_labs/google/keywords_for_site/live",
		seo.GoogleRanked:     "/v3/dataforseo_labs/google/ranked_keywords/live",
		seo.BingVolume:       "/v3/keywords_data/bing/search_volume/live",
		seo.GoogleSERP:       "/v3/serp/google/organic/live/regular",
		seo.BingSERP:         "/v3/serp/bing/organic/live/regular",
	}
	prices := map[string]map[string]any{}
	var records []model.Record
	pointer := ""
	bodySchema := json.RawMessage(`{"type":"array","minItems":1,"maxItems":1,"items":{"type":"object","additionalProperties":true}}`)
	for name, path := range routes {
		tool := name
		record := model.Record{
			APIID: tool, Name: "DataForSEO " + path, Description: "DataForSEO SEO route", ProviderID: "dataforseo", ToolName: &tool,
			Status: "active", Method: "POST", RequestContentType: "application/json", PathTemplate: path,
			PublicPath: "/v1/agent-apis/dataforseo" + path, ConfigPresent: true, UnknownParameters: "reject",
			Inputs: []model.Input{{Name: "body", Location: "body", Required: true, Pointer: &pointer, Schema: bodySchema}},
		}
		records = append(records, record)
		base, per, max := "12000", "120", "132000"
		switch name {
		case seo.BingVolume:
			base, per, max = "90000", "0", "90000"
		case seo.GoogleSERP, seo.BingSERP:
			base, per, max = "2000", "0", "2000"
		}
		prices[tool] = rulesPrice(base, per, max)
	}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		return prices, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	return manager
}

func rulesPrice(base, per, max string) map[string]any {
	quantities := []any{}
	if per != "0" {
		quantities = append(quantities, map[string]any{"id": "returned_results", "pointer": "/tasks/0/result/0/items_count", "price_micro_usd": per, "unit": "result", "unit_quantity": "1", "when": []any{}})
	}
	return map[string]any{
		"mode": "rules", "currency": "USD",
		"policy": map[string]any{
			"policy_version": 2, "parameters": []any{},
			"sale": map[string]any{
				"base_price_micro_usd": base, "max_cost_micro_usd": max, "rounding": "ceil",
				"variants": []any{}, "addons": []any{}, "quantities": quantities,
			},
		},
	}
}

func seoCall(t *testing.T, base, path, method string, params any, auth string) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	if json.NewDecoder(res.Body).Decode(&out) != nil || res.StatusCode != 200 {
		t.Fatal(res.StatusCode, out)
	}
	return out
}

func seoTool(t *testing.T, base, name string, args map[string]any, auth string) map[string]any {
	t.Helper()
	res := seoCall(t, base, "/", "tools/call", map[string]any{"name": name, "arguments": args}, auth)
	result, ok := res["result"].(map[string]any)
	if !ok {
		t.Fatal(res)
	}
	return result
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
