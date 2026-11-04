package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestHTTPDiscoveryCallScopeAndHeaderIsolation(t *testing.T) {
	seen := ""
	seenQuery := ""
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		seenQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"n":9007199254740993}`))
	}))
	defer gw.Close()
	client, _ := gateway.New(gw.URL, time.Second, time.Second, 1024, 4096)
	name := "demo_list"
	r := model.Record{APIID: "00000000-0000-4000-8000-000000000001", Name: "List", ToolName: &name, ProviderID: "demo", Method: "GET", PathTemplate: "/list", PublicPath: "/v1/agent-apis/demo/list", Status: "active", ConfigPresent: true, RequestContentType: "none", UnknownParameters: "reject"}
	r.Inputs = []model.Input{{Name: "cursor", Location: "query", Type: "string", Nullable: true}}
	invalidAPI := r
	invalidAPI.APIID, invalidAPI.PathTemplate, invalidAPI.PublicPath = "00000000-0000-4000-8000-000000000002", "/other", "/v1/agent-apis/demo/other"
	badName := "demo_bad"
	invalidAPI.ToolName = &badName
	invalidAPI.Inputs = []model.Input{{Name: "tickers", Location: "query", Type: "array"}}
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r, invalidAPI}, nil }, model.Overrides{Version: 1}, time.Minute, false)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := New(manager, client, Options{MaxBody: 4096, MaxCalls: 2})
	server := httptest.NewServer(h)
	defer server.Close()
	rpc := func(path, method string, params any, auth string) map[string]any {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		req.Header.Set("Authorization", auth)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var out map[string]any
		if json.Unmarshal(raw, &out) != nil {
			t.Fatalf("%d %s", res.StatusCode, raw)
		}
		return out
	}
	list := rpc("/mcp/demo", "tools/list", map[string]any{}, "")
	if list["error"] != nil {
		t.Fatal(list)
	}
	tools := list["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "demo_list" {
		t.Fatal("quarantined tool leaked into discovery", list)
	}
	call := rpc("/mcp/demo", "tools/call", map[string]any{"name": "demo_list", "arguments": map[string]any{"cursor": nil}}, "Bearer caller")
	if call["error"] != nil || seen != "Bearer caller" || seenQuery != "" {
		t.Fatalf("%v auth=%s query=%s", call, seen, seenQuery)
	}
	response, _ := http.Get(server.URL + "/readyz")
	var status map[string]any
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || status["published"] != float64(1) || status["quarantined"] != float64(1) {
		t.Fatal("incorrect partial catalog readiness", response.StatusCode, status)
	}
	if _, exposed := status["issues"]; exposed {
		t.Fatal("public readiness exposed API configuration details")
	}
	bad, _ := http.NewRequest("POST", server.URL+"/mcp", bytes.NewReader([]byte(`{}`)))
	bad.Header.Set("Origin", "https://untrusted.invalid")
	response, _ = http.DefaultClient.Do(bad)
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("bad origin accepted")
	}
}
func TestAllProtocolVersionsAndScope(t *testing.T) {
	name := "demo_list"
	other := "other_list"
	records := []model.Record{}
	for _, p := range []struct{ provider, name, id string }{{"demo", name, "00000000-0000-4000-8000-000000000001"}, {"other", other, "00000000-0000-4000-8000-000000000002"}} {
		n := p.name
		records = append(records, model.Record{APIID: p.id, Name: p.name, ToolName: &n, ProviderID: p.provider, Method: "GET", PathTemplate: "/list", PublicPath: "/v1/agent-apis/" + p.provider + "/list", Status: "active", ConfigPresent: true, RequestContentType: "none", UnknownParameters: "reject"})
	}
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	client, _ := gateway.New("http://127.0.0.1:1", time.Millisecond, time.Millisecond, 1024, 4096)
	server := httptest.NewServer(New(manager, client, Options{MaxCalls: 1}))
	defer server.Close()
	for _, version := range []string{"2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			params := map[string]any{}
			if version == "2026-07-28" {
				params["_meta"] = map[string]any{"io.modelcontextprotocol/protocolVersion": version, "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
			}
			body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": params})
			req, _ := http.NewRequest("POST", server.URL+"/mcp/demo", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", version)
			req.Header.Set("Mcp-Method", "tools/list")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			raw, _ := io.ReadAll(res.Body)
			var response struct {
				Result struct {
					Tools []struct {
						Name string `json:"name"`
					} `json:"tools"`
				} `json:"result"`
				Error any `json:"error"`
			}
			if json.Unmarshal(raw, &response) != nil || response.Error != nil || len(response.Result.Tools) != 1 || response.Result.Tools[0].Name != "demo_list" {
				t.Fatalf("%d %s", res.StatusCode, raw)
			}
		})
	}
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"other_list","arguments":{}}}`)
	req, _ := http.NewRequest("POST", server.URL+"/mcp/demo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	res, _ := http.DefaultClient.Do(req)
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(raw, []byte(`"error"`)) {
		t.Fatalf("cross-scope tool allowed: %s", raw)
	}
}

func TestCachedCallsDoNotDependOnMutableSourceRecords(t *testing.T) {
	type observed struct{ body, query, auth string }
	seen := make(chan observed, 1)
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen <- observed{string(raw), r.URL.RawQuery, r.Header.Get("Authorization")}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer gw.Close()
	client, _ := gateway.New(gw.URL, time.Second, time.Second, 4096, 16384)
	name, pointer := "demo_submit", "/payload"
	r := model.Record{APIID: "00000000-0000-4000-8000-000000000001", Name: "Submit", Description: "Public description\nPrivate ingestion notes", ToolName: &name, ProviderID: "demo", Method: "POST", PathTemplate: "/submit", PublicPath: "/v1/agent-apis/demo/submit", Status: "active", ConfigPresent: true, RequestContentType: "application/json", UnknownParameters: "reject", Inputs: []model.Input{
		{Name: "body", Location: "body", Type: "object", Required: true, Pointer: &pointer, Schema: json.RawMessage(`{"type":"object","required":["n"],"properties":{"n":{"type":"integer"}},"additionalProperties":false}`)},
		{Name: "ticker", Location: "query", Type: "array"},
	}}
	overrides := model.Overrides{Version: 1, APIs: map[string]model.Override{r.APIID: {QueryArrays: map[string]model.ArrayEncoding{"ticker": {Encoding: "repeated", Empty: "empty", Source: "test contract"}}}}}
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, overrides, time.Minute, false)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, client, Options{MaxCalls: 1}))
	defer server.Close()
	rpc := func(method string, params any) map[string]any {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest("POST", server.URL+"/mcp/demo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		req.Header.Set("Authorization", "Bearer current-caller")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var response map[string]any
		if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response["error"] != nil {
			t.Fatal(response)
		}
		return response["result"].(map[string]any)
	}
	before := rpc("tools/list", map[string]any{})
	// A loader's original containers must not become the published runtime bindings.
	pointer = "/changed"
	r.Inputs[0].Name = "discarded_source_name"
	r.Inputs[0].Schema = nil
	overrides.APIs[r.APIID].QueryArrays["ticker"] = model.ArrayEncoding{Encoding: "csv", Empty: "omit", Source: "changed source"}
	if after := rpc("tools/list", map[string]any{}); !reflect.DeepEqual(before, after) {
		t.Fatal("source mutation changed discovery")
	}
	result := rpc("tools/call", map[string]any{"name": name, "arguments": map[string]any{"body": map[string]any{"n": json.Number("9007199254740993")}, "ticker": []string{"A", "B"}}})
	if result["isError"] == true {
		t.Fatal(result)
	}
	got := <-seen
	if got.body != `{"payload":{"n":9007199254740993}}` || got.query != "ticker=A&ticker=B" || got.auth != "Bearer current-caller" {
		t.Fatalf("published request changed with discarded source data: %+v", got)
	}
}
