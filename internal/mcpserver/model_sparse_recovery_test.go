package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func TestSparseModelRecoveryAcrossHTTPAndMCPEndpoints(t *testing.T) {
	for _, tc := range []struct{ label, model string }{
		{"v4-provider", "deepseek-v4-flash"}, {"v4-mode", "deepseek-v4-flash"}, {"v4-combined", "deepseek-v4-flash"},
		{"v41-provider", "DeepSeek-V4.1-Flash"}, {"v41-mode", "DeepSeek-V4.1-Flash"}, {"v41-combined", "DeepSeek-V4.1-Flash"},
		{"v4-similar", "deepseek-v4-flash"}, {"v41-similar", "DeepSeek-V4.1-Flash"},
		{"empty", "DeepSeek-V4.1-Flash"}, {"disabled", "DeepSeek-V4.1-Flash"},
	} {
		for _, entrance := range []string{"root", "lite"} {
			t.Run(tc.label+"/"+entrance, func(t *testing.T) {
				dir := os.Getenv("OPENANY_SPARSE_MODEL_FIXTURES_DIR")
				if dir == "" {
					dir = "../gateway/testdata/recovery-candidates/v2/sparse-models"
				}
				raw, err := os.ReadFile(filepath.Join(dir, "sparse-"+tc.label+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var source map[string]any
				if err := json.Unmarshal(raw, &source); err != nil {
					t.Fatal(err)
				}
				var hits atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" {
						t.Error("unexpected gateway path", r.Method, r.URL.Path)
					}
					var request map[string]any
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
					}
					if request["model"] != tc.model {
						t.Error("model changed before routing", request)
					}
					if strings.Contains(tc.label, "mode") || strings.Contains(tc.label, "combined") || strings.Contains(tc.label, "similar") || tc.label == "empty" || tc.label == "disabled" {
						if request["route_mode"] != "balanced" {
							t.Error("route_mode was lost")
						}
					}
					if !strings.HasSuffix(tc.label, "-mode") {
						if route, ok := request["route"].(map[string]any); !ok || route["provider"] != "Neolink1" {
							t.Error("provider was lost", request)
						}
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(400)
					_, _ = w.Write(raw)
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
						{ID: tc.model, Name: tc.model, InputModalities: []string{"text"}, OutputModalities: []string{"text"}},
						{ID: "local-small-chat", Name: "local-small-chat", InputModalities: []string{"text"}, OutputModalities: []string{"text"}},
					}, nil
				})
				if err := manager.Refresh(t.Context()); err != nil {
					t.Fatal(err)
				}
				if err := manager.RefreshPricing(t.Context()); err != nil {
					t.Fatal(err)
				}
				server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, AlternativesEnabled: true, ModelGateway: client}))
				defer server.Close()
				path := "/"
				if entrance == "lite" {
					path = "/mcp-lite"
				}
				rpc := func(tool string, args map[string]any) map[string]any {
					return recoveryRPC(t, server.URL, path, tool, args)
				}
				detail := rpc("get_api", map[string]any{"api": tc.model})["structuredContent"].(map[string]any)
				input := map[string]any{"model": tc.model, "messages": []any{map[string]any{"role": "user", "content": "Hello!"}}}
				if !strings.HasSuffix(tc.label, "-mode") {
					input["route"] = map[string]any{"provider": "Neolink1"}
				}
				if !strings.HasSuffix(tc.label, "-provider") {
					input["route_mode"] = "balanced"
				}
				result := rpc("run_api", map[string]any{"api": tc.model, "api_version": detail["api_version"], "input": input})
				value := result["structuredContent"].(map[string]any)
				e := value["error"].(map[string]any)
				if result["isError"] != true || value["http_status"] != float64(400) || e["code"] != "openany_route_unavailable" || e["execution_state"] != "not_started" || e["charge_state"] != "not_charged" {
					t.Fatal("original facts lost", result)
				}
				if tc.label == "empty" || tc.label == "disabled" {
					r := e["recovery"].(map[string]any)
					if e["alternatives"] != nil || r["reason"] != "routing_options_unavailable" || r["primary_action"] != "correct_input" {
						t.Fatal("routing guidance lost", e)
					}
					if !strings.Contains(e["message"].(string), "route_mode") {
						t.Fatal("routing-specific message lost", e)
					}
				} else {
					candidates, ok := e["alternatives"].([]any)
					if !ok || len(candidates) == 0 || e["alternatives_version"] != float64(2) {
						t.Fatal("candidate lost", e)
					}
					for _, raw := range candidates {
						c := raw.(map[string]any)
						k := c["compatibility"].(map[string]any)
						if k["level"] != "needs_inspection" || k["basis"] != "catalog_inferred" || c["execution"] != nil {
							t.Fatal("inspection promoted", c)
						}
						continuation := c["continuation"].(map[string]any)["mcp"].(map[string]any)
						if continuation["execute_tool"] != nil || continuation["refresh_api_version"] != nil {
							t.Fatal("unverified execution continuation", continuation)
						}
						inspect := continuation["inspect"].(map[string]any)
						message := e["message"].(string)
						for _, part := range []string{c["model_id"].(string), c["provider_id"].(string), "get_api", "read-only"} {
							if !strings.Contains(message, part) {
								t.Fatalf("message-only guidance missing %q: %s", part, message)
							}
						}
						if strings.Contains(tc.label, "similar") && !strings.Contains(message, "different model") {
							t.Fatal(message)
						}
						if strings.Contains(message, "run_api") {
							t.Fatal("read-only message advertises execution", message)
						}
						encodedArgs, _ := json.Marshal(inspect["arguments"])
						callText := "get_api(" + string(encodedArgs) + ")"
						start := strings.Index(message, callText)
						if start < 0 {
							t.Fatalf("missing exact message-only next step %s: %s", callText, message)
						}
						var messageArgs map[string]any
						if err := json.Unmarshal([]byte(message[start+len("get_api("):start+len(callText)-1]), &messageArgs); err != nil {
							t.Fatal(err)
						}
						if inspect["tool"] != "get_api" {
							t.Fatal("missing inspection", inspect)
						}
						fresh := rpc("get_api", messageArgs)
						if fresh["isError"] == true || fresh["structuredContent"].(map[string]any)["api_version"] == nil {
							t.Fatal("inspection target unavailable", fresh)
						}
					}
					if len(candidates) != len(source["details"].(map[string]any)["alternatives"].([]any)) {
						t.Fatal("candidate dropped in MCP", e)
					}
				}
				if hits.Load() != 1 {
					t.Fatal("candidate or get_api dispatched again", hits.Load())
				}
				if out := os.Getenv("RECOVERY_EVIDENCE_DIR"); out != "" {
					hash := sha256.Sum256(raw)
					record := map[string]any{"fixture_sha256": hex.EncodeToString(hash[:]), "model": tc.model, "case": tc.label, "entrance": path, "gateway_calls": hits.Load(), "supplier_calls": 0, "candidate_execution_calls": 0, "mcp_result": result}
					body, _ := json.MarshalIndent(record, "", "  ")
					if err := os.WriteFile(filepath.Join(out, "mcp-sparse-"+tc.label+"-"+entrance+".json"), body, 0600); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
