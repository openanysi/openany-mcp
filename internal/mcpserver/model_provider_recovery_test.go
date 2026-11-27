package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

// Default fixtures are exported from the Gateway service tests, with only time
// and trace ID normalized. The integration run can consume fresh exports instead.
func TestModelProviderRoutingRecoveryRPC(t *testing.T) {
	for _, label := range []string{"unknown-provider", "route-mode", "unknown-provider-route-mode", "unavailable-model-similar"} {
		t.Run(label, func(t *testing.T) {
			dir := os.Getenv("OPENANY_MODEL_RECOVERY_FIXTURES_DIR")
			if dir == "" {
				dir = "../gateway/testdata/recovery-candidates/v2/model-routing"
			}
			raw, err := os.ReadFile(filepath.Join(dir, "gateway-"+label+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var fixture map[string]any
			if err = json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			expected := fixture["details"].(map[string]any)["alternatives"].([]any)[0].(map[string]any)
			var sourceCalls, targetCalls atomic.Int32
			provider := "missing-provider"
			if label == "route-mode" {
				provider = "neolink"
			}
			withMode := label != "unknown-provider"
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				route, _ := body["route"].(map[string]any)
				if withMode && body["route_mode"] != "balanced" {
					t.Error("route_mode was lost before reaching Gateway")
				}
				if !withMode && body["route_mode"] != nil {
					t.Error("route_mode was invented")
				}
				w.Header().Set("Content-Type", "application/json")
				if body["model"] == "public-model" && route["provider"] == provider {
					sourceCalls.Add(1)
					w.WriteHeader(400)
					_, _ = w.Write(raw)
					return
				}
				if body["model"] == expected["model_id"] && route["provider"] == expected["provider_id"] {
					targetCalls.Add(1)
					_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"local continuation OK"}}],"openany_cost":0}`))
					return
				}
				t.Errorf("unexpected dispatch: %#v", body)
				w.WriteHeader(500)
			}))
			defer upstream.Close()
			client, err := gateway.New(upstream.URL, time.Second, time.Second, 32768, 131072)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.EnableAlternatives(true)
			manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
			manager.SetModelLoader(func(context.Context) ([]catalog.LLMModel, error) {
				return []catalog.LLMModel{
					{ID: "public-model", Name: "public-model", InputModalities: []string{"text"}, OutputModalities: []string{"text"}},
					{ID: "similar-chat-model", Name: "similar-chat-model", InputModalities: []string{"text"}, OutputModalities: []string{"text"}},
				}, nil
			})
			if err = manager.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err = manager.RefreshPricing(context.Background()); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, AlternativesEnabled: true, ModelGateway: client}))
			defer server.Close()
			rpc := func(tool string, args map[string]any) map[string]any {
				return recoveryRPC(t, server.URL, "/mcp-lite", tool, args)
			}
			detail := rpc("get_api", map[string]any{"api": "public-model"})["structuredContent"].(map[string]any)
			input := map[string]any{"model": "public-model", "messages": []any{map[string]any{"role": "user", "content": "Hello!"}}, "route": map[string]any{"provider": provider}, "max_tokens": 16, "stream": false}
			if withMode {
				input["route_mode"] = "balanced"
			}
			if label == "unknown-provider-route-mode" {
				delete(input, "max_tokens")
				delete(input, "stream")
			}
			result := rpc("run_api", map[string]any{"api": "public-model", "api_version": detail["api_version"], "input": input})
			value := result["structuredContent"].(map[string]any)
			if result["isError"] != true || value["http_status"] != float64(400) {
				t.Fatalf("original error lost: %#v", result)
			}
			e := value["error"].(map[string]any)
			if e["code"] != "openany_route_unavailable" || e["alternatives_version"] != float64(2) {
				t.Fatalf("candidates lost: %#v", e)
			}
			recovery := e["recovery"].(map[string]any)
			if recovery["execution_state"] != "not_started" || recovery["charge_state"] != "not_charged" {
				t.Fatal("unsafe execution or charge state", recovery)
			}
			c := e["alternatives"].([]any)[0].(map[string]any)
			if c["model_id"] != expected["model_id"] || c["provider_id"] != expected["provider_id"] || c["relation"] != expected["relation"] {
				t.Fatal("candidate identity changed", c)
			}
			compatibility := c["compatibility"].(map[string]any)
			if len(compatibility["missing_evidence"].([]any)) != 0 {
				t.Fatal("extra output-limit label introduced")
			}
			if label == "unavailable-model-similar" && (compatibility["level"] != "similar" || len(compatibility["differences"].([]any)) == 0) {
				t.Fatal("model change was not disclosed")
			}
			if sourceCalls.Load() != 1 || targetCalls.Load() != 0 {
				t.Fatal("recommendation executed a candidate")
			}
			continuation := c["continuation"].(map[string]any)["mcp"].(map[string]any)
			inspect := continuation["inspect"].(map[string]any)
			fresh := rpc("get_api", inspect["arguments"].(map[string]any))["structuredContent"].(map[string]any)
			if fresh["api_version"] == nil || continuation["execute_tool"] != "run_api" || continuation["refresh_api_version"] != true {
				t.Fatal("missing refresh continuation", continuation)
			}
			if sourceCalls.Load() != 1 || targetCalls.Load() != 0 {
				t.Fatal("inspection executed a candidate")
			}
			next := map[string]any{}
			for k, v := range input {
				next[k] = v
			}
			for k, v := range c["execution"].(map[string]any)["request_overrides"].(map[string]any) {
				next[k] = v
			}
			stale := rpc("run_api", map[string]any{"api": c["model_id"], "api_version": strings.Repeat("0", 64), "input": next})
			if stale["isError"] != true || targetCalls.Load() != 0 {
				t.Fatal("stale contract dispatched")
			}
			continued := rpc("run_api", map[string]any{"api": c["model_id"], "api_version": fresh["api_version"], "input": next})
			if continued["isError"] == true || targetCalls.Load() != 1 || sourceCalls.Load() != 1 {
				t.Fatalf("explicit continuation failed: %#v", continued)
			}
			if dir := os.Getenv("MCP_R2_EVIDENCE_DIR"); dir != "" {
				b, _ := json.MarshalIndent(map[string]any{"case": label, "source_calls": sourceCalls.Load(), "test_driven_target_calls": targetCalls.Load(), "recommendation_target_calls": 0, "inspection_target_calls": 0, "stale_version_target_calls": 0, "initial_error": result, "explicit_local_continuation": continued}, "", "  ")
				if err := os.WriteFile(filepath.Join(dir, "mcp-provider-routing-"+label+".json"), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
