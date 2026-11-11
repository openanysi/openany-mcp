package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

func TestRootRoutesLiteOverviewAndInstructions(t *testing.T) {
	var hits atomic.Int32
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer gw.Close()
	client, err := gateway.New(gw.URL, time.Second, time.Second, 4096, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	records := []model.Record{}
	for _, spec := range []struct{ provider, name string }{{"tikhub", "tikhub_feed"}, {"demo", "demo_weather"}} {
		name := spec.name
		record := model.Record{APIID: spec.name, Name: spec.name, Description: "Weather and feeds", ProviderID: spec.provider, ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/" + spec.name, PublicPath: "/v1/agent-apis/" + spec.provider + "/" + spec.name, UnknownParameters: "reject", ConfigPresent: true}
		records = append(records, record)
	}
	rawModels := `{"data":[
	  {"id":"deepseek-v4-flash","input_modalities":["chat"],"output_modalities":["chat"],"is_ready":true},
	  {"id":"Hy-Image-3.5-Preview","input_modalities":["chat","image"],"output_modalities":["image"],"supported_features":["image_generation"],"is_ready":true},
	  {"id":"tts-1","input_modalities":["chat"],"output_modalities":["audio"],"is_ready":true}
	]}`
	models, err := catalog.ParseModelCatalog([]byte(rawModels))
	if err != nil {
		t.Fatal(err)
	}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		return map[string]map[string]any{}, nil
	})
	manager.SetModelLoader(func(context.Context) ([]catalog.LLMModel, error) { return models, nil })
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, ModelGateway: client}))
	defer server.Close()
	page, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(page.Body)
	page.Body.Close()
	if page.StatusCode != 200 || !strings.HasPrefix(page.Header.Get("Content-Type"), "text/plain") || !bytes.Contains(body, []byte("search_apis")) {
		t.Fatal("browser GET / did not return the connection guide", page.StatusCode, page.Header.Get("Content-Type"), len(body))
	}
	rpc := func(path, method string, params any) (map[string]any, int) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		if json.NewDecoder(res.Body).Decode(&out) != nil {
			t.Fatal(path, method, res.StatusCode)
		}
		return out, res.StatusCode
	}
	initParams := map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "0"}}
	rootInit, status := rpc("/", "initialize", initParams)
	liteInit, _ := rpc("/mcp-lite", "initialize", initParams)
	if status != 200 || rootInit["error"] != nil {
		t.Fatal(status, rootInit)
	}
	rootResult := rootInit["result"].(map[string]any)
	liteResult := liteInit["result"].(map[string]any)
	instructions, _ := rootResult["instructions"].(string)
	for _, phrase := range []string{"openany_overview", "search_apis", "get_api", "run_api", "category", "modality", "confirms that price", "do not call the paid gateway"} {
		if !strings.Contains(instructions, phrase) {
			t.Fatalf("lite instructions missing %q: %s", phrase, instructions)
		}
	}
	if instructions != liteResult["instructions"] || rootResult["serverInfo"].(map[string]any)["name"] != "OpenAny Lite" {
		t.Fatal("root and /mcp-lite initialize diverged", rootResult, liteResult)
	}
	fullInit, _ := rpc("/mcp", "initialize", initParams)
	full := fullInit["result"].(map[string]any)
	fullInfo := full["serverInfo"].(map[string]any)
	fullInstructions, _ := full["instructions"].(string)
	if fullInfo["name"] != "OpenAny Full Catalog" || !strings.Contains(fullInstructions, "https://mcp.openany.si") || !strings.Contains(fullInstructions, "does not include AI models") || fullInstructions == instructions {
		t.Fatal(full)
	}
	scoped, _ := rpc("/mcp/demo", "initialize", initParams)
	if scoped["result"].(map[string]any)["serverInfo"].(map[string]any)["name"] != "OpenAny Full Catalog" {
		t.Fatal(scoped)
	}
	listed, _ := rpc("/", "tools/list", map[string]any{})
	tools := listed["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 4 {
		t.Fatal(len(tools))
	}
	call, _ := rpc("/", "tools/call", map[string]any{"name": "openany_overview", "arguments": map[string]any{}})
	same, _ := rpc("/mcp-lite", "tools/call", map[string]any{"name": "openany_overview", "arguments": map[string]any{}})
	got := call["result"].(map[string]any)["structuredContent"].(map[string]any)
	again := same["result"].(map[string]any)["structuredContent"].(map[string]any)
	if !reflect.DeepEqual(got, again) {
		t.Fatal("root and lite overview diverged", got, again)
	}
	if got["api_count"] != float64(2) || got["model_count"] != float64(3) || got["models_available"] != true {
		t.Fatal(got)
	}
	categories := got["apis_by_category"].([]any)
	if len(categories) != 8 {
		t.Fatal(len(categories))
	}
	categoryCounts := map[string]float64{}
	var categorySum float64
	for _, item := range categories {
		row := item.(map[string]any)
		categoryCounts[row["category"].(string)] = row["count"].(float64)
		categorySum += row["count"].(float64)
	}
	if categoryCounts["social/content"] != 1 || categoryCounts["other"] != 1 || categorySum != 2 {
		t.Fatal(categoryCounts, categorySum)
	}
	providers := got["apis_by_provider"].([]any)
	if len(providers) != 2 || providers[0].(map[string]any)["provider"] != "demo" && providers[0].(map[string]any)["provider"] != "tikhub" {
		t.Fatal(providers)
	}
	modalities := got["models_by_modality"].([]any)
	found := map[string][]any{}
	var modelSum float64
	for _, item := range modalities {
		row := item.(map[string]any)
		found[row["modality"].(string)] = row["models"].([]any)
		modelSum += row["count"].(float64)
	}
	if modelSum != 3 || len(found["chat/reasoning"]) != 1 || found["chat/reasoning"][0] != "deepseek-v4-flash" || len(found["image"]) != 1 || found["image"][0] != "Hy-Image-3.5-Preview" || len(found["audio"]) != 1 {
		t.Fatal(found, modelSum)
	}
	rejected, _ := rpc("/", "tools/call", map[string]any{"name": "openany_overview", "arguments": map[string]any{"query": "chat"}})
	if rejected["result"].(map[string]any)["isError"] != true {
		t.Fatal("overview accepted arguments", rejected)
	}
	direct, _ := rpc("/mcp", "tools/list", map[string]any{})
	directTools := direct["result"].(map[string]any)["tools"].([]any)
	if len(directTools) != 2 {
		t.Fatal("full catalog changed", len(directTools))
	}
	for _, tool := range directTools {
		if tool.(map[string]any)["name"] == "openany_overview" {
			t.Fatal("overview leaked onto /mcp")
		}
	}
	if hits.Load() != 0 {
		t.Fatal("overview called the gateway", hits.Load())
	}
	bad, _ := http.NewRequest("POST", server.URL+"/", bytes.NewReader([]byte(`{}`)))
	bad.Header.Set("Origin", "https://untrusted.invalid")
	res, err := http.DefaultClient.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("root bypassed origin check", res.StatusCode)
	}
	disabled := httptest.NewServer(New(manager, client, Options{}))
	defer disabled.Close()
	for _, path := range []string{"/", "/mcp-lite"} {
		res, err := http.Get(disabled.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatal(path, res.StatusCode)
		}
	}
	still, _ := http.Get(disabled.URL + "/mcp")
	still.Body.Close()
	if still.StatusCode == 404 {
		t.Fatal("disabling lite removed /mcp")
	}
}
