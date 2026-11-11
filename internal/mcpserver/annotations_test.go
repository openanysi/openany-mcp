package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestToolListAdvertisesTitlesAndAnnotations(t *testing.T) {
	name := "demo_list"
	record := model.Record{APIID: name, Name: "List", Description: "List items", ProviderID: "demo", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/list", PublicPath: "/v1/agent-apis/demo/list", UnknownParameters: "reject", ConfigPresent: true}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		return map[string]map[string]any{}, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, nil, Options{LiteEnabled: true}))
	defer server.Close()
	list := func(path string) []any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}})
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
		if json.NewDecoder(res.Body).Decode(&out) != nil || res.StatusCode != 200 {
			t.Fatal(path, res.StatusCode, out)
		}
		return out["result"].(map[string]any)["tools"].([]any)
	}
	want := map[string]map[string]any{
		"openany_overview": {"title": "OpenAny overview", "readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		"search_apis":      {"title": "Search APIs and models", "readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		"get_api":          {"title": "Get API or model details", "readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		"run_api":          {"title": "Run an API or model call", "readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": true},
	}
	checkLite := func(path string) {
		t.Helper()
		tools := list(path)
		if len(tools) != 4 {
			t.Fatal(path, len(tools))
		}
		seen := map[string]bool{}
		for _, item := range tools {
			tool := item.(map[string]any)
			name := tool["name"].(string)
			expected := want[name]
			if expected == nil {
				t.Fatal(path, name)
			}
			seen[name] = true
			annotations := tool["annotations"].(map[string]any)
			if tool["title"] != expected["title"] || !reflect.DeepEqual(annotations, expected) {
				t.Fatalf("%s %s title=%v annotations=%v", path, name, tool["title"], annotations)
			}
		}
		if len(seen) != 4 {
			t.Fatal(path, seen)
		}
	}
	checkLite("/")
	checkLite("/mcp-lite")
	for _, path := range []string{"/mcp", "/mcp/demo"} {
		tools := list(path)
		if len(tools) != 1 {
			t.Fatal(path, len(tools))
		}
		tool := tools[0].(map[string]any)
		annotations := tool["annotations"].(map[string]any)
		expected := map[string]any{"title": "List", "readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": true}
		if tool["name"] != "demo_list" || tool["title"] != "List" || !reflect.DeepEqual(annotations, expected) {
			t.Fatalf("%s %#v", path, tool)
		}
	}
	doc, err := os.ReadFile("../../docs/listing/tool-annotations.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	if !bytes.Contains(doc, []byte("The open marketplace for agents")) {
		t.Fatal("annotation doc dropped the tagline")
	}
	for _, phrase := range []string{
		"OpenAny overview",
		"Search APIs and models",
		"Get API or model details",
		"Run an API or model call",
		"Returns live catalog counts from a public snapshot; no arguments; no paid gateway or third-party call.",
		"Keyword search over OpenAny's own catalog metadata; returns summaries only; no paid call or write.",
		"Reads one catalog entry's schema and public price; discovery only; no gateway call.",
		"Executes a selected API or chat-completions model call through OpenAny; can incur charges and have upstream side effects; does not itself delete resources, so not destructive, but not read-only and reaches external providers.",
	} {
		if !bytes.Contains(doc, []byte(phrase)) {
			t.Fatal("missing", phrase, text)
		}
	}
}
