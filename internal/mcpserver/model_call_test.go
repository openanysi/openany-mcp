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
)

func TestLiteModelSearchGetAndRun(t *testing.T) {
	var mu sync.Mutex
	var seen []map[string]string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, map[string]string{"path": r.URL.Path, "auth": r.Header.Get("Authorization"), "body": string(body)})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer upstream.Close()
	client, err := gateway.New(upstream.URL, time.Second, time.Second, 1<<20, 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	models, err := catalog.ParseModelCatalog([]byte(`{"data":[{"id":"deepseek-v4-flash","name":"deepseek-v4-flash","input_modalities":["chat"],"output_modalities":["chat"],"context_length":1000000,"pricing":{"prompt":"0.000000225","completion":"0.000000675","image":"0","request":"0","duration":"0"},"is_ready":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	name := "demo_translate"
	record := model.Record{APIID: "translate", Name: "Translate text", Description: "Translation API", ProviderID: "demo", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/translate", PublicPath: "/v1/agent-apis/demo/translate", UnknownParameters: "reject", ConfigPresent: true}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
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
	call := func(tool string, args any, auth string) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		req, _ := http.NewRequest("POST", server.URL+"/mcp-lite", bytes.NewReader(raw))
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
		if json.NewDecoder(res.Body).Decode(&out) != nil {
			t.Fatal(res.StatusCode)
		}
		result, _ := out["result"].(map[string]any)
		if result == nil {
			t.Fatal(out)
		}
		return result
	}
	listRaw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}})
	req, _ := http.NewRequest("POST", server.URL+"/mcp-lite", bytes.NewReader(listRaw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var listed map[string]any
	json.NewDecoder(res.Body).Decode(&listed)
	res.Body.Close()
	tools := listed["result"].(map[string]any)["tools"].([]any)
	joined := ""
	names := map[string]bool{}
	for _, tool := range tools {
		item := tool.(map[string]any)
		names[item["name"].(string)] = true
		joined += item["description"].(string)
	}
	if len(tools) != 4 || !names["openany_overview"] || !strings.Contains(joined, "models and APIs") {
		t.Fatal(tools)
	}
	search := call("search_apis", map[string]any{"query": "deepseek"}, "")
	content := search["structuredContent"].(map[string]any)
	results := content["results"].([]any)
	if content["models_available"] != true || len(results) == 0 || results[0].(map[string]any)["kind"] != "model" {
		t.Fatal(content)
	}
	apiOnly := call("search_apis", map[string]any{"query": "translation", "kind": "api"}, "")
	for _, item := range apiOnly["structuredContent"].(map[string]any)["results"].([]any) {
		if item.(map[string]any)["kind"] != "api" {
			t.Fatal(item)
		}
	}
	detail := call("get_api", map[string]any{"api": "deepseek-v4-flash"}, "")["structuredContent"].(map[string]any)
	if detail["input_schema"] == nil || detail["api_version"] == nil || detail["pricing_brief"] == "" {
		t.Fatal(detail)
	}
	ran := call("run_api", map[string]any{"api": "deepseek-v4-flash", "api_version": detail["api_version"], "input": map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Hello"}}}}, "Bearer caller-model")
	if ran["isError"] == true {
		t.Fatal(ran)
	}
	mu.Lock()
	if len(seen) != 1 || seen[0]["path"] != "/v1/chat/completions" || seen[0]["auth"] != "Bearer caller-model" || !strings.Contains(seen[0]["body"], `"model":"deepseek-v4-flash"`) || !strings.Contains(seen[0]["body"], `"content":"Hello"`) {
		t.Fatal(seen)
	}
	mu.Unlock()
	rejected := call("run_api", map[string]any{"api": "deepseek-v4-flash", "api_version": detail["api_version"], "input": map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Hello"}}, "stream": true}}, "Bearer caller-model")
	if rejected["isError"] != true {
		t.Fatal(rejected)
	}
	mu.Lock()
	if len(seen) != 1 {
		t.Fatal("rejected stream was dispatched", seen)
	}
	mu.Unlock()
}
