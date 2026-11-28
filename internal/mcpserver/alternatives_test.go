package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestC30C33AlternativesRPCMapsExactRevisionAndPreservesText(t *testing.T) {
	raw, err := os.ReadFile("../gateway/testdata/recovery-candidates/available-api.json")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(raw)
	}))
	defer upstream.Close()
	client, _ := gateway.New(upstream.URL, time.Second, time.Second, 8192, 16384)
	defer client.Close()
	client.EnableAlternatives(true)
	source, target := "source_read", "target_read"
	records := []model.Record{
		{APIID: "00000000-0000-4000-8000-000000000001", Revision: 1, ProviderID: "source", Name: "source", ToolName: &source, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/read", PublicPath: "/v1/agent-apis/source/read", ConfigPresent: true, UnknownParameters: "reject"},
		{APIID: "00000000-0000-4000-8000-000000000002", Revision: 1, ProviderID: "target", Name: "target", ToolName: &target, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/suggestions", PublicPath: "/v1/agent-apis/target/suggestions", ConfigPresent: true, UnknownParameters: "reject"},
	}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, AlternativesEnabled: true}))
	defer server.Close()
	rpc := func(path, tool string, args map[string]any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		req.Header.Set("Authorization", "Bearer local-test-only")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		if json.NewDecoder(res.Body).Decode(&out) != nil {
			t.Fatal("invalid RPC")
		}
		r, ok := out["result"].(map[string]any)
		if !ok {
			t.Fatal(out)
		}
		return r
	}
	detail := rpc("/mcp-lite", "get_api", map[string]any{"api": source})["structuredContent"].(map[string]any)
	for _, path := range []string{"/mcp-lite", "/mcp/source"} {
		before := calls.Load()
		var result map[string]any
		if path == "/mcp-lite" {
			result = rpc(path, "run_api", map[string]any{"api": source, "api_version": detail["api_version"], "input": map[string]any{}})
		} else {
			result = rpc(path, source, map[string]any{})
		}
		value := result["structuredContent"].(map[string]any)
		e := value["error"].(map[string]any)
		a := e["alternatives"].([]any)[0].(map[string]any)
		mcp, ok := a["continuation"].(map[string]any)
		if !ok {
			t.Fatal("missing public mapping", a)
		}
		continuation := mcp["mcp"].(map[string]any)
		if continuation["endpoint"] != "/mcp-lite" {
			t.Fatal("unreachable MCP continuation", continuation)
		}
		inspect := continuation["inspect"].(map[string]any)
		message := e["message"].(string)
		if !strings.Contains(message, a["name"].(string)) || !strings.Contains(message, a["provider_id"].(string)) || !strings.Contains(message, `get_api({"api":"target_read"})`) {
			t.Fatal("API message guidance missing", message)
		}
		refreshed := rpc(continuation["endpoint"].(string), "get_api", inspect["arguments"].(map[string]any))
		if refreshed["structuredContent"].(map[string]any)["api_version"] == nil {
			t.Fatal("continuation cannot inspect target")
		}
		if inspect["arguments"].(map[string]any)["api"] != target || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || calls.Load() != before+1 {
			t.Fatal("state, mapping or call count", value)
		}
		var textValue map[string]any
		content := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if json.Unmarshal([]byte(content), &textValue) != nil || !reflect.DeepEqual(textValue, value) {
			t.Fatal("text differs from structuredContent")
		}
		if dir := os.Getenv("RECOVERY_EVIDENCE_DIR"); dir != "" {
			b, _ := json.MarshalIndent(result, "", "  ")
			os.WriteFile(filepath.Join(dir, "local-mcp"+map[string]string{"/mcp-lite": "-lite", "/mcp/source": "-full"}[path]+"-candidate.json"), b, 0600)
		}
	}
	records[1].Revision = 2
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	result := rpc("/mcp/source", source, map[string]any{})
	e := result["structuredContent"].(map[string]any)["error"].(map[string]any)
	candidate := e["alternatives"].([]any)[0].(map[string]any)
	if candidate["continuation"] != nil || candidate["definition_path"] == nil {
		t.Fatal("stale revision advertised as MCP executable", candidate)
	}
	message := e["message"].(string)
	if strings.Contains(message, "get_api(") || !strings.Contains(message, "GET "+candidate["definition_path"].(string)) {
		t.Fatal("stale mapping leaked into message", message)
	}
}

func TestC31OnlyCurrentNonStreamingChatModelsAdvertiseMCPContinuation(t *testing.T) {
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	manager.SetModelLoader(func(context.Context) ([]catalog.LLMModel, error) {
		return []catalog.LLMModel{{ID: "chat-b", Name: "chat-b", InputModalities: []string{"text"}, OutputModalities: []string{"text"}}}, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{options: Options{AlternativesEnabled: true, LiteEnabled: true}}
	for _, tc := range []struct {
		protocol, path string
		stream, want   bool
	}{{"openai-chat-completions", "/v1/chat/completions", false, true}, {"openai-chat-completions", "/v1/chat/completions", true, false}, {"openai-responses", "/v1/responses", false, false}, {"anthropic-messages", "/v1/messages", false, false}, {"openai-audio", "/v1/audio/transcriptions", false, false}} {
		c := map[string]any{"kind": "model", "model_id": "chat-b", "execution": map[string]any{"protocol": tc.protocol, "path": tc.path, "request_overrides": map[string]any{"stream": tc.stream}}}
		result := gateway.Result{Value: map[string]any{"error": map[string]any{"alternatives": []any{c}}}}
		handler.withAlternatives(result, manager.Current().Discovery)
		if (c["continuation"] != nil) != tc.want {
			t.Fatalf("protocol=%s stream=%v continuation=%v", tc.protocol, tc.stream, c["continuation"])
		}
	}
}
