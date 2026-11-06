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
		return v["structuredContent"].(map[string]any)
	}
	found := call("search_apis", map[string]any{"query": "帮我查询企业", "category": "company-data"})
	if len(found["results"].([]any)) != 1 {
		t.Fatal(found)
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
