package mcpserver

import (
	"context"
	"encoding/json"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This fixture is the actual response exported by the Gateway HTTP route test.
func unpublishedModelFixture(t *testing.T) []byte {
	t.Helper()
	file := "../gateway/testdata/model-discovery/unpublished.json"
	if dir := os.Getenv("MCP_R2_EVIDENCE_DIR"); dir != "" {
		file = filepath.Join(dir, "gateway-unpublished-model.json")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestUnpublishedModelOffersReadOnlyRelatedModels(t *testing.T) {
	raw := unpublishedModelFixture(t)
	var reads, business atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models/recovery" || r.Method != "GET" {
			business.Add(1)
			w.WriteHeader(500)
			return
		}
		reads.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("catalog read forwarded caller credential")
		}
		if r.URL.Query().Get("model") != "gpt-5.2" {
			t.Error("wrong source")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}))
	defer upstream.Close()
	client, err := gateway.New(upstream.URL, time.Second, time.Second, 32768, 131072)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	manager.SetModelLoader(func(context.Context) ([]catalog.LLMModel, error) {
		return []catalog.LLMModel{{ID: "chat-live", Name: "Live chat", InputModalities: []string{"text"}, OutputModalities: []string{"chat"}}}, nil
	})
	if err = manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, AlternativesEnabled: true, ModelGateway: client}))
	defer server.Close()
	for _, tool := range []string{"get_api", "run_api"} {
		args := map[string]any{"api": "gpt-5.2"}
		if tool == "run_api" {
			args["api_version"] = strings.Repeat("0", 64)
			args["input"] = map[string]any{"messages": []any{map[string]any{"role": "user", "content": "must never reach business gateway"}}}
		}
		result := recoveryRPC(t, server.URL, "/mcp-lite", tool, args)
		e := result["structuredContent"].(map[string]any)["error"].(map[string]any)
		if result["isError"] != true || e["code"] != "openany_mcp_api_unavailable" {
			t.Fatal("original unavailable error lost", result)
		}
		candidates, ok := e["related_models"].([]any)
		if !ok || len(candidates) != 1 {
			t.Fatalf("no related models: %#v", e)
		}
		c := candidates[0].(map[string]any)
		if e["execution_state"] != "not_started" || e["charge_state"] != "not_charged" || !strings.Contains(e["next_action"].(string), "get_api") {
			t.Fatal(e)
		}
		if dir := os.Getenv("MCP_R2_EVIDENCE_DIR"); dir != "" {
			b, _ := json.MarshalIndent(result, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, "mcp-unpublished-"+tool+".json"), b, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if c["model_id"] != "chat-live" || c["provider_id"] != "live-provider" {
			t.Fatal(c)
		}
		if c["execution"] != nil || c["api_version"] != nil {
			t.Fatal("discovery granted execution", c)
		}
		next := c["continuation"].(map[string]any)["mcp"].(map[string]any)
		if next["execute_tool"] != nil || next["inspect"].(map[string]any)["tool"] != "get_api" {
			t.Fatal("unsafe continuation", next)
		}
		detail := recoveryRPC(t, server.URL, "/mcp-lite", "get_api", map[string]any{"api": "chat-live"})["structuredContent"].(map[string]any)
		if detail["api_version"] == nil || business.Load() != 0 {
			t.Fatal("inspection caused business call")
		}
	}
	if reads.Load() != 2 || business.Load() != 0 {
		t.Fatalf("reads=%d business=%d", reads.Load(), business.Load())
	}
}

func TestUnpublishedModelFailsClosedWithoutExecuting(t *testing.T) {
	for _, scenario := range []string{"disabled", "stale-models", "unavailable-catalog", "missing-model-origin", "unknown-source", "source-available", "unpublished-target", "image-target", "old-gateway", "invalid-json", "timeout", "no-candidates"} {
		t.Run(scenario, func(t *testing.T) {
			raw := unpublishedModelFixture(t)
			var reads, business atomic.Int32
			var payload map[string]any
			json.Unmarshal(raw, &payload)
			switch scenario {
			case "unknown-source":
				payload["source_state"] = "unknown"
				payload["related_models"] = []any{}
			case "source-available":
				payload["source_state"] = "available"
				payload["related_models"] = []any{}
			case "no-candidates":
				payload["related_models"] = []any{}
			}
			raw, _ = json.Marshal(payload)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/v1/models/recovery" {
					business.Add(1)
					w.WriteHeader(500)
					return
				}
				reads.Add(1)
				if scenario == "old-gateway" {
					w.WriteHeader(404)
					return
				}
				if scenario == "timeout" {
					time.Sleep(50 * time.Millisecond)
				}
				w.Header().Set("Content-Type", "application/json")
				if scenario == "invalid-json" {
					w.Write([]byte(`{`))
					return
				}
				w.Write(raw)
			}))
			defer upstream.Close()
			timeout := time.Second
			if scenario == "timeout" {
				timeout = 10 * time.Millisecond
			}
			client, _ := gateway.New(upstream.URL, timeout, time.Second, 32768, 131072)
			defer client.Close()
			manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
			manager.SetModelLoader(func(context.Context) ([]catalog.LLMModel, error) {
				id := "chat-live"
				if scenario == "unpublished-target" {
					id = "different-live"
				}
				output := []string{"chat"}
				if scenario == "image-target" {
					output = []string{"image"}
				}
				return []catalog.LLMModel{{ID: id, InputModalities: []string{"text"}, OutputModalities: output}}, nil
			})
			if err := manager.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := manager.RefreshPricing(context.Background()); err != nil {
				t.Fatal(err)
			}
			if scenario == "stale-models" {
				manager.Current().Discovery.ModelsStale = true
			}
			if scenario == "unavailable-catalog" {
				manager.Current().Discovery.ModelsAvailable = false
			}
			options := Options{LiteEnabled: true, AlternativesEnabled: scenario != "disabled", ModelGateway: client}
			if scenario == "missing-model-origin" {
				options.ModelGateway = nil
			}
			server := httptest.NewServer(New(manager, client, options))
			defer server.Close()
			for _, tool := range []string{"get_api", "run_api"} {
				args := map[string]any{"api": "gpt-5.2"}
				if tool == "run_api" {
					args["api_version"] = strings.Repeat("0", 64)
					args["input"] = map[string]any{"messages": []any{}}
				}
				result := recoveryRPC(t, server.URL, "/mcp-lite", tool, args)
				e := result["structuredContent"].(map[string]any)["error"].(map[string]any)
				if result["isError"] != true || e["code"] != "openany_mcp_api_unavailable" || e["execution_state"] != "not_started" || e["related_models"] != nil {
					t.Fatal("unsafe unavailable handling", e)
				}
			}
			if business.Load() != 0 {
				t.Fatal("business execution occurred")
			}
			expected := int32(2)
			switch scenario {
			case "disabled", "stale-models", "unavailable-catalog", "missing-model-origin":
				expected = 0
			}
			if reads.Load() != expected {
				t.Fatalf("reads=%d expected=%d", reads.Load(), expected)
			}
		})
	}
}
