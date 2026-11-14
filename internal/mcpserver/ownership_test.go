package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestLiteOwnershipResource(t *testing.T) {
	const wantURI = "agent-tools://ownership"
	const wantBody = `{"agentToolsVerify":"atc_OfYhv55TTxeGWsZNHcK7wCuui3GlJSPJ"}`
	name := "demo_weather"
	record := model.Record{APIID: name, Name: "Weather", Description: "Forecast", ProviderID: "demo", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/weather", PublicPath: "/v1/agent-apis/demo/weather", UnknownParameters: "reject", ConfigPresent: true}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		return map[string]map[string]any{}, nil
	})
	if err := manager.Refresh(context.Background()); err != nil || manager.RefreshPricing(context.Background()) != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, nil, Options{LiteEnabled: true}))
	defer server.Close()
	rpc := func(path, method string, params any) (map[string]any, int) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		if req.Header.Get("Authorization") != "" || req.Header.Get("X-Api-Key") != "" {
			t.Fatal("ownership check sent credentials")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		var out map[string]any
		if json.Unmarshal(body, &out) != nil {
			t.Fatal(method, path, res.StatusCode, string(body))
		}
		return out, res.StatusCode
	}
	initParams := map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t", "version": "0"}}
	for _, path := range []string{"/", "/mcp-lite"} {
		initOut, status := rpc(path, "initialize", initParams)
		if status != 200 || initOut["error"] != nil {
			t.Fatal(path, status, initOut)
		}
		result := initOut["result"].(map[string]any)
		info := result["serverInfo"].(map[string]any)
		caps := result["capabilities"].(map[string]any)
		if info["name"] != "OpenAny" || info["title"] != "OpenAny" || caps["resources"] == nil || caps["tools"] == nil {
			t.Fatal(path, result)
		}
		listed, status := rpc(path, "resources/list", map[string]any{})
		if status != 200 || listed["error"] != nil {
			t.Fatal(path, status, listed)
		}
		resources := listed["result"].(map[string]any)["resources"].([]any)
		found := false
		for _, item := range resources {
			resource := item.(map[string]any)
			if resource["uri"] != wantURI {
				continue
			}
			found = true
			if resource["mimeType"] != "application/json" || resource["name"] != "ownership" {
				t.Fatal(path, resource)
			}
		}
		if !found {
			t.Fatal(path, "ownership resource missing", listed)
		}
		read, status := rpc(path, "resources/read", map[string]any{"uri": wantURI})
		if status != 200 || read["error"] != nil {
			t.Fatal(path, status, read)
		}
		contents := read["result"].(map[string]any)["contents"].([]any)
		if len(contents) != 1 {
			t.Fatal(path, contents)
		}
		content := contents[0].(map[string]any)
		if content["uri"] != wantURI || content["mimeType"] != "application/json" || content["text"] != wantBody {
			t.Fatal(path, content)
		}
	}
	fullInit, _ := rpc("/mcp", "initialize", initParams)
	fullCaps := fullInit["result"].(map[string]any)["capabilities"].(map[string]any)
	if fullCaps["resources"] != nil {
		t.Fatal("full catalog advertised resources", fullCaps)
	}
	fullList, _ := rpc("/mcp", "resources/list", map[string]any{})
	if result, ok := fullList["result"].(map[string]any); ok {
		if resources, ok := result["resources"].([]any); ok {
			for _, item := range resources {
				if item.(map[string]any)["uri"] == wantURI {
					t.Fatal("full catalog listed the ownership resource")
				}
			}
		}
	}
	tools, _ := rpc("/", "tools/list", map[string]any{})
	names := map[string]bool{}
	for _, item := range tools["result"].(map[string]any)["tools"].([]any) {
		names[item.(map[string]any)["name"].(string)] = true
	}
	if len(names) != 4 || !names["openany_overview"] || !names["search_apis"] || !names["get_api"] || !names["run_api"] {
		t.Fatal(names)
	}
}
