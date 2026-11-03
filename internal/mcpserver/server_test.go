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
	"testing"
	"time"
)

func TestHTTPDiscoveryCallScopeAndHeaderIsolation(t *testing.T) {
	seen := ""
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"n":9007199254740993}`))
	}))
	defer gw.Close()
	client, _ := gateway.New(gw.URL, time.Second, time.Second, 1024, 4096)
	name := "demo_list"
	r := model.Record{APIID: "00000000-0000-4000-8000-000000000001", Name: "List", ToolName: &name, ProviderID: "demo", Method: "GET", PathTemplate: "/list", PublicPath: "/v1/agent-apis/demo/list", Status: "active", ConfigPresent: true, RequestContentType: "none", UnknownParameters: "reject"}
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false)
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
	call := rpc("/mcp/demo", "tools/call", map[string]any{"name": "demo_list", "arguments": map[string]any{}}, "Bearer caller")
	if call["error"] != nil || seen != "Bearer caller" {
		t.Fatalf("%v auth=%s", call, seen)
	}
	response, _ := http.Get(server.URL + "/readyz")
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("not ready")
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
