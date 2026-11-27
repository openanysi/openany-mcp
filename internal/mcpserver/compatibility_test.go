package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/config"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestHunterAndAlchemyFullLiteRPCSendReviewedWireContracts(t *testing.T) {
	raw, err := os.ReadFile("../catalog/testdata/compatibility/hunter-alchemy.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []model.Record
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	overrides, err := config.ReadOverrides("../../config/catalog-overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Authorization") != "Bearer local-compatibility-test" {
			t.Error("caller credential missing or changed")
		}
		switch r.URL.Path {
		case "/v1/agent-apis/hunter/v2/domain-finder":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.URL.RawQuery != "" {
				t.Error("Hunter method/content type/query changed")
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.PostForm.Get("company") != "中国 A&B+Corp" || r.PostForm.Get("limit") != "2" || r.PostForm.Get("perfect_match") != "false" || len(r.PostForm) != 3 {
				t.Errorf("wrong form fields: %v", r.PostForm)
			}
		case "/v1/agent-apis/alchemy/prices/v1/tokens/by-symbol":
			if r.Method != "GET" || r.URL.RawQuery != "symbols=ETH%2CBTC" || len(r.URL.Query()["symbols"]) != 1 || len(body) != 0 {
				t.Errorf("wrong Alchemy CSV request: %s %q", r.Method, r.URL.RawQuery)
			}
		default:
			t.Error("unexpected request path")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	client, err := gateway.New(upstream.URL, time.Second, time.Second, 8192, 16384)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, overrides, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if current := manager.Current(); len(current.Tools) != 2 || current.Quarantined != 0 {
		t.Fatalf("compatibility listings unavailable: %+v", current.Issues)
	}
	server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true}))
	defer server.Close()
	rpc := func(path, tool string, input map[string]any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": input}})
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		req.Header.Set("Authorization", "Bearer local-compatibility-test")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var response map[string]any
		if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		result, ok := response["result"].(map[string]any)
		if !ok {
			t.Fatal(response)
		}
		return result
	}
	results := []map[string]any{}
	for _, record := range records {
		var input map[string]any
		if record.ProviderID == "hunter" {
			input = map[string]any{"company": "中国 A&B+Corp", "limit": 2, "perfect_match": false}
		} else {
			input = map[string]any{"symbols": []string{"ETH", "BTC"}}
		}
		detail := rpc("/mcp-lite", "get_api", map[string]any{"api": *record.ToolName})["structuredContent"].(map[string]any)
		for _, path := range []string{"/mcp-lite", "/mcp/" + record.ProviderID} {
			before := calls.Load()
			var result map[string]any
			if path == "/mcp-lite" {
				result = rpc(path, "run_api", map[string]any{"api": *record.ToolName, "api_version": detail["api_version"], "input": input})
			} else {
				result = rpc(path, *record.ToolName, input)
			}
			if result["isError"] == true || calls.Load() != before+1 {
				t.Fatalf("business request missing or repeated: %v", result)
			}
			results = append(results, map[string]any{"endpoint": path, "api": *record.ToolName, "one_dispatch": true, "result": result})
		}
		invalidInputs := []map[string]any{{}}
		if record.ProviderID == "alchemy" {
			invalidInputs[0]["symbols"] = []any{}
		} else {
			invalidInputs = append(invalidInputs, map[string]any{"company": map[string]any{"nested": "not a flat form scalar"}})
		}
		for _, bad := range invalidInputs {
			for _, path := range []string{"/mcp-lite", "/mcp/" + record.ProviderID} {
				before := calls.Load()
				var invalid map[string]any
				if path == "/mcp-lite" {
					invalid = rpc(path, "run_api", map[string]any{"api": *record.ToolName, "api_version": detail["api_version"], "input": bad})
				} else {
					invalid = rpc(path, *record.ToolName, bad)
				}
				if invalid["isError"] != true || calls.Load() != before {
					t.Fatal("invalid input was dispatched", invalid)
				}
			}
		}
	}
	if calls.Load() != 4 {
		t.Fatal("unexpected total execution count", calls.Load())
	}
	if dir := os.Getenv("RECOVERY_EVIDENCE_DIR"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"local_mock_gateway": true, "mock_gateway_dispatches": 4, "invalid_requests_dispatched": 0, "supplier_calls": 0, "results": results}, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "compatibility-rpc-wire.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
