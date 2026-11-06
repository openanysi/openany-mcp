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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

func TestLiteUsabilityWorkflowDoesNotChangeGatewayRequests(t *testing.T) {
	var mu sync.Mutex
	seen := []string{}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.URL.String()+"|"+string(body)+"|"+r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "request-1")
		w.Write([]byte(`{"rows":[{"id":9007199254740993,"name":"A","other":true},{"id":2,"name":"B"}],"total":99}`))
	}))
	defer gw.Close()
	client, _ := gateway.New(gw.URL, time.Second, time.Second, 4096, 8192)
	defer client.Close()
	name, root := "demo_company", ""
	record := model.Record{APIID: "api-1", Revision: 1, Name: "Company", Description: "Company lookup", ProviderID: "demo", ToolName: &name, Status: "active", Method: "POST", RequestContentType: "application/json", PathTemplate: "/company", PublicPath: "/v1/agent-apis/demo/company", ConfigPresent: true, UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Pointer: &root, Required: true, Schema: json.RawMessage(`{"type":"object","required":["company"],"properties":{"company":{"type":"string"},"count":{"type":"integer"}}}`)}}}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return nil, nil }, func(context.Context) (map[string]catalog.APIMetadata, error) {
		return map[string]catalog.APIMetadata{"api-1": {Revision: 1, Categories: []string{"company-data"}, NameTranslations: map[string]string{"zh-CN": "企业查询"}}}, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true}))
	defer server.Close()
	call := func(name string, args any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		r, _ := http.NewRequest("POST", server.URL+"/mcp-lite", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-11-25")
		r.Header.Set("Authorization", "Bearer test-caller")
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope map[string]any
		if json.NewDecoder(res.Body).Decode(&envelope) != nil {
			t.Fatal("bad MCP response")
		}
		v, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatal(envelope)
		}
		content := v["structuredContent"].(map[string]any)
		if content["error"] != nil && v["isError"] != true {
			t.Fatal("tool error not marked for the MCP client", v)
		}
		return content
	}
	found := call("search_apis", map[string]any{"query": "帮我查询企业", "category": "company-data"})
	if len(found["results"].([]any)) != 1 {
		t.Fatal(found)
	}
	for _, bad := range []map[string]any{
		{"query": "企业", "cursor": "invalid-cursor"},
		{},
		{"query": "企业", "limit": 100},
	} {
		failed := call("search_apis", bad)
		e := failed["error"].(map[string]any)
		action := e["next_action"].(string)
		if e["execution_state"] != "not_started" || strings.Contains(action, "get_api.input_schema") {
			t.Fatal("search error pointed to business schema", failed)
		}
		if e["code"] == "openany_mcp_invalid_search" && (!strings.Contains(action, "search_apis") || !strings.Contains(action, "cursor")) {
			t.Fatal("search correction step missing", failed)
		}
		if e["details_scope"] == "arguments" && (!strings.Contains(action, "inputSchema") || !strings.Contains(action, "details")) {
			t.Fatal("tool argument correction step missing", failed)
		}
		corrected := call("search_apis", map[string]any{"query": "企业", "limit": 5})
		if len(corrected["results"].([]any)) != 1 {
			t.Fatal("corrected search did not recover", corrected)
		}
	}
	detail := call("get_api", map[string]any{"api": name})
	if detail["examples"] == nil || detail["price_summary"] == nil {
		t.Fatal(detail)
	}
	version := detail["api_version"]
	invalid := call("run_api", map[string]any{"api": name, "api_version": version, "input": map[string]any{"body": map[string]any{"count": "secret-input"}}})
	errObj := invalid["error"].(map[string]any)
	raw, _ := json.Marshal(invalid)
	if errObj["next_action"] == nil || len(errObj["details"].([]any)) != 2 || strings.Contains(string(raw), "secret-input") {
		t.Fatal(invalid)
	}
	input := map[string]any{"body": map[string]any{"company": "sample"}}
	for _, bad := range []any{map[string]any{"mode": "compact", "items_path": "rows"}, map[string]any{"mode": "raw", "max_items": 1}, map[string]any{"mode": "compact", "max_items": 0}} {
		res := call("run_api", map[string]any{"api": name, "api_version": version, "input": input, "output": bad})
		if res["error"].(map[string]any)["execution_state"] != "not_started" {
			t.Fatal(res)
		}
	}
	mu.Lock()
	n := len(seen)
	mu.Unlock()
	if n != 0 {
		t.Fatal("invalid input/output reached gateway", n)
	}
	rawResult := call("run_api", map[string]any{"api": name, "api_version": version, "input": input})
	if rawResult["data"] == nil || rawResult["summary"] == nil {
		t.Fatal(rawResult)
	}
	compact := call("run_api", map[string]any{"api": name, "api_version": version, "input": input, "output": map[string]any{"mode": "compact", "items_path": "/rows", "max_items": 1, "fields": []string{"id", "name"}}})
	if len(compact["data"].(map[string]any)["rows"].([]any)) != 1 {
		t.Fatal(compact)
	}
	summary := call("run_api", map[string]any{"api": name, "api_version": version, "input": input, "output": map[string]any{"mode": "summary"}})
	if _, exists := summary["data"]; exists {
		t.Fatal(summary)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 || !reflect.DeepEqual(seen, []string{seen[0], seen[0], seen[0]}) {
		t.Fatal("presentation changed gateway request", seen)
	}
}
func TestRecoveryNeverEncouragesUnknownExecutionRetry(t *testing.T) {
	result := withRecovery(gateway.Failure("openany_mcp_gateway_transport", "failed", "gateway", "unknown", nil, nil))
	action := result.Value["error"].(map[string]any)["next_action"].(string)
	if !strings.Contains(action, "Do not retry") {
		t.Fatal(action)
	}
}

func TestMCPTextWorkflowAcrossAllEntrances(t *testing.T) {
	var hits atomic.Int32
	text := "# 今日新闻\n- 第一条\n- 第二条"
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != "GET" || r.URL.String() != "/v1/agent-apis/demo/news" || r.Header.Get("Authorization") != "Bearer text-caller" || r.Header.Get("Accept") != "application/json" {
			t.Error("dispatch contract changed")
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("X-Request-ID", "text-workflow")
		w.Write([]byte(text))
	}))
	defer gw.Close()
	client, _ := gateway.New(gw.URL, time.Second, time.Second, 4096, 8192)
	defer client.Close()
	name := "demo_news"
	record := model.Record{APIID: "news", Name: "Latest news", Description: "News as Markdown", ProviderID: "demo", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/news", PublicPath: "/v1/agent-apis/demo/news", ConfigPresent: true, UnknownParameters: "reject"}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return nil, nil })
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(New(manager, client, Options{LiteEnabled: true}))
	defer s.Close()
	rpc := func(path, method string, params any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		r, _ := http.NewRequest("POST", s.URL+path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-11-25")
		r.Header.Set("Authorization", "Bearer text-caller")
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var envelope map[string]any
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || envelope["error"] != nil {
			t.Fatal("MCP RPC failed", err, envelope)
		}
		return envelope["result"].(map[string]any)
	}
	list := rpc("/mcp-lite", "tools/list", map[string]any{})
	for _, value := range list["tools"].([]any) {
		tool := value.(map[string]any)
		if tool["name"] == "run_api" && !strings.Contains(tool["description"].(string), "Markdown") {
			t.Fatal("text capability missing from tool guidance")
		}
	}
	detail := rpc("/mcp-lite", "tools/call", map[string]any{"name": "get_api", "arguments": map[string]any{"api": name}})["structuredContent"].(map[string]any)
	if detail["examples"].(map[string]any)["items"].([]any)[0].(map[string]any)["kind"] != "empty_request" {
		t.Fatal("empty request guidance missing", detail)
	}
	args := map[string]any{"api": name, "api_version": detail["api_version"], "input": map[string]any{}}
	for _, path := range []string{"/mcp-lite", "/mcp", "/mcp/demo"} {
		params := map[string]any{"name": "run_api", "arguments": args}
		if path != "/mcp-lite" {
			params = map[string]any{"name": name, "arguments": map[string]any{}}
		}
		result := rpc(path, "tools/call", params)
		content := result["structuredContent"].(map[string]any)
		if result["isError"] == true || content["data"] != text || content["request_id"] != "text-workflow" {
			t.Fatal("text lost through MCP", path, result)
		}
		var textCopy map[string]any
		if err := json.Unmarshal([]byte(result["content"].([]any)[0].(map[string]any)["text"].(string)), &textCopy); err != nil || !reflect.DeepEqual(textCopy, content) {
			t.Fatal("text-only MCP clients received different data", result)
		}
	}
	args["output"] = map[string]any{"mode": "summary"}
	summary := rpc("/mcp-lite", "tools/call", map[string]any{"name": "run_api", "arguments": args})["structuredContent"].(map[string]any)
	if _, ok := summary["data"]; ok || summary["summary"].(map[string]any)["type"] != "string" {
		t.Fatal("text summary failed", summary)
	}
	args["output"] = map[string]any{"mode": "compact", "fields": []string{"title"}}
	compact := rpc("/mcp-lite", "tools/call", map[string]any{"name": "run_api", "arguments": args})["structuredContent"].(map[string]any)
	if compact["data"] != text || compact["error"].(map[string]any)["execution_state"] != "completed" || hits.Load() != 5 {
		t.Fatal("text projection discarded data or replayed request", compact, hits.Load())
	}
}
