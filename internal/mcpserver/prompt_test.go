package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestLiteStartPrompt(t *testing.T) {
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
	rpc := func(path, method string, params any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		var out map[string]any
		if json.Unmarshal(body, &out) != nil || out["error"] != nil {
			t.Fatal(method, path, string(body))
		}
		return out["result"].(map[string]any)
	}
	initResult := rpc("/", "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t", "version": "0"}})
	promptsCap := initResult["capabilities"].(map[string]any)["prompts"]
	instructions := initResult["instructions"].(string)
	if promptsCap == nil || !strings.Contains(instructions, "vague") || !strings.Contains(instructions, "openany_overview") {
		t.Fatal(initResult)
	}
	listed := rpc("/mcp-lite", "prompts/list", map[string]any{})
	prompts := listed["prompts"].([]any)
	if len(prompts) != 1 || prompts[0].(map[string]any)["name"] != "openany-start" {
		t.Fatal(listed)
	}
	if prompts[0].(map[string]any)["title"] != "开始使用 OpenAny / Get started with OpenAny" {
		t.Fatal(prompts[0])
	}
	got := rpc("/", "prompts/get", map[string]any{"name": "openany-start"})
	message := got["messages"].([]any)[0].(map[string]any)
	text := message["content"].(map[string]any)["text"].(string)
	if message["role"] != "user" || !strings.Contains(text, "openany_overview") || strings.Contains(text, "OpenAny 能帮我做什么") {
		t.Fatal(got)
	}
	zh := rpc("/mcp-lite", "prompts/get", map[string]any{"name": "openany-start", "arguments": map[string]any{"lang": "zh"}})
	zhText := zh["messages"].([]any)[0].(map[string]any)["content"].(map[string]any)["text"].(string)
	if !strings.Contains(zhText, "OpenAny 能帮我做什么") || !strings.Contains(zhText, "openany_overview") {
		t.Fatal(zhText)
	}
	full := rpc("/mcp", "prompts/list", map[string]any{})
	if prompts, ok := full["prompts"].([]any); ok {
		for _, item := range prompts {
			if item.(map[string]any)["name"] == "openany-start" {
				t.Fatal("full catalog advertised the lite prompt")
			}
		}
	}
}
