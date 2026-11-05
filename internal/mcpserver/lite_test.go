package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestLitePublicDocsAvailableBeforeCatalog(t *testing.T) {
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false)
	server := httptest.NewServer(New(manager, nil, Options{LiteEnabled: true}))
	defer server.Close()
	res, err := http.Get(server.URL + "/docs/mcp")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/plain") || len(body) < 1000 {
		t.Fatal("public docs missing while catalog starts", res.StatusCode, len(body))
	}
	res, err = http.Post(server.URL+"/docs/mcp", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 405 {
		t.Fatal("documentation endpoint accepts mutations", res.StatusCode)
	}
}

func TestLiteWorkflowPreservesDirectCallsAndRejectsUnsafeDispatch(t *testing.T) {
	var mu sync.Mutex
	var seen []map[string]string
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, map[string]string{"method": r.Method, "path": r.URL.Path, "query": r.URL.RawQuery, "body": string(body), "auth": r.Header.Get("Authorization")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer gw.Close()
	client, _ := gateway.New(gw.URL, time.Second, time.Second, 4096, 8192)
	defer client.Close()
	var records []model.Record
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("demo_weather_%02d", i)
		pointer := ""
		r := model.Record{APIID: name, Name: "Weather", Description: "Weather forecast", ProviderID: "demo", ToolName: &name, Status: "active", Method: "POST", RequestContentType: "application/json", PathTemplate: fmt.Sprintf("/weather/%d", i), ConfigPresent: true, UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Type: "object", Location: "body", Required: true, Pointer: &pointer}, {Name: "cursor", Type: "string", Location: "query", Nullable: true}}}
		r.PublicPath = "/v1/agent-apis/demo" + r.PathTemplate
		records = append(records, r)
	}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := New(manager, client, Options{LiteEnabled: true})
	server := httptest.NewServer(h)
	defer server.Close()
	rpc := func(path, method string, params any, auth string) (map[string]any, int) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		req.Header.Set("Authorization", auth)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		var out map[string]any
		if res.StatusCode == 200 && json.Unmarshal(data, &out) != nil {
			t.Fatal(string(data))
		}
		return out, len(data)
	}
	list, size := rpc("/mcp-lite", "tools/list", map[string]any{}, "")
	result, ok := list["result"].(map[string]any)
	if !ok {
		t.Fatal("lightweight endpoint not available", list)
	}
	if len(result["tools"].([]any)) != 3 || size > 10*1024 || result["nextCursor"] != nil {
		t.Fatal("unbounded lightweight tool list", size, list)
	}
	for _, tool := range result["tools"].([]any) {
		name := tool.(map[string]any)["name"]
		if name != "search_apis" && name != "get_api" && name != "run_api" {
			t.Fatal(name)
		}
	}
	call := func(name string, args any, auth string) map[string]any {
		t.Helper()
		res, _ := rpc("/mcp-lite", "tools/call", map[string]any{"name": name, "arguments": args}, auth)
		r, ok := res["result"].(map[string]any)
		if !ok {
			t.Fatal("tool RPC failed", res)
		}
		return r
	}
	search := call("search_apis", map[string]any{"query": "weather"}, "")
	if len(search["structuredContent"].(map[string]any)["results"].([]any)) != 5 {
		t.Fatal("search was not bounded", search)
	}
	for _, number := range []string{"1", "1.0", "1e0"} {
		limited := call("search_apis", map[string]any{"query": "weather", "limit": json.Number(number)}, "")
		if len(limited["structuredContent"].(map[string]any)["results"].([]any)) != 1 {
			t.Errorf("limit %s ignored", number)
		}
	}
	detail := call("get_api", map[string]any{"api": "demo_weather_00"}, "")["structuredContent"].(map[string]any)
	if detail["input_schema"] == nil || detail["api_version"] == nil {
		t.Fatal("missing API contract", detail)
	}
	args := map[string]any{"body": map[string]any{"n": json.Number("9007199254740993")}, "cursor": nil}
	lite := call("run_api", map[string]any{"api": "demo_weather_00", "api_version": detail["api_version"], "input": args}, "Bearer caller-A")
	if lite["isError"] == true {
		t.Fatal(lite)
	}
	direct, _ := rpc("/mcp/demo", "tools/call", map[string]any{"name": "demo_weather_00", "arguments": args}, "Bearer caller-A")
	if direct["result"].(map[string]any)["isError"] == true {
		t.Fatal(direct)
	}
	mu.Lock()
	if len(seen) != 2 || !reflect.DeepEqual(seen[0], seen[1]) || seen[0]["body"] != `{"n":9007199254740993}` || seen[0]["query"] != "" || seen[0]["auth"] != "Bearer caller-A" {
		t.Error("direct/lite dispatch changed", seen)
	}
	mu.Unlock()
	oldVersion := detail["api_version"]
	records[1].Revision++
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	otherRefresh := call("get_api", map[string]any{"api": "demo_weather_00"}, "")["structuredContent"].(map[string]any)
	if otherRefresh["api_version"] != oldVersion {
		t.Fatal("another API revision invalidated selected API")
	}
	records[0].Revision++
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	stale := call("run_api", map[string]any{"api": "demo_weather_00", "api_version": oldVersion, "input": args}, "Bearer caller-B")
	if stale["isError"] != true {
		t.Fatal("revision-only update accepted old version")
	}
	detail = call("get_api", map[string]any{"api": "demo_weather_00"}, "")["structuredContent"].(map[string]any)
	for _, bad := range []map[string]any{
		{"api": "demo_weather_00", "api_version": detail["api_version"], "input": map[string]any{}},
		{"api": "demo_weather_00", "api_version": "0000000000000000000000000000000000000000000000000000000000000000", "input": args},
		{"api": "not_published", "api_version": detail["api_version"], "input": args},
		{"api": "demo_weather_00", "api_version": detail["api_version"], "input": args, "url": "https://example.invalid"},
	} {
		if res := call("run_api", bad, "Bearer caller-B"); res["isError"] != true {
			t.Fatal("invalid call accepted", res)
		}
	}
	mu.Lock()
	if len(seen) != 2 {
		t.Error("invalid input reached gateway", seen)
	}
	mu.Unlock()
	if res := call("run_api", map[string]any{"api": "demo_weather_00", "api_version": detail["api_version"], "input": args}, "Bearer caller-B"); res["isError"] == true {
		t.Fatal(res)
	}
	mu.Lock()
	if seen[2]["auth"] != "Bearer caller-B" {
		t.Error("credential leaked across calls", seen)
	}
	mu.Unlock()
	// Both views must use the same admission budget before a billable dispatch.
	for i := 0; i < cap(h.slots); i++ {
		h.slots <- struct{}{}
	}
	busy := call("run_api", map[string]any{"api": "demo_weather_00", "api_version": detail["api_version"], "input": args}, "Bearer caller-B")
	busyDirect, _ := rpc("/mcp", "tools/call", map[string]any{"name": "demo_weather_00", "arguments": args}, "Bearer caller-B")
	if busy["structuredContent"].(map[string]any)["error"].(map[string]any)["code"] != "openany_mcp_busy" || busyDirect["result"].(map[string]any)["isError"] != true {
		t.Fatal("views have different concurrency budgets", busy, busyDirect)
	}
	for i := 0; i < cap(h.slots); i++ {
		<-h.slots
	}
	mu.Lock()
	if len(seen) != 3 {
		t.Error("busy call dispatched", seen)
	}
	mu.Unlock()
	for _, kind := range []string{"host", "origin"} {
		req, _ := http.NewRequest("POST", server.URL+"/mcp-lite", bytes.NewReader([]byte(`{}`)))
		if kind == "host" {
			req.Host = "untrusted.invalid"
		} else {
			req.Header.Set("Origin", "https://untrusted.invalid")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatal("lite bypassed boundary", kind, res.StatusCode)
		}
	}
	disabled := httptest.NewServer(New(manager, client, Options{}))
	defer disabled.Close()
	res, err := http.Get(disabled.URL + "/mcp-lite")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal("lite enabled without flag", res.StatusCode)
	}
}
