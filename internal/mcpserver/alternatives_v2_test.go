package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

func recoveryRPC(t *testing.T, server, path, tool string, arguments map[string]any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": arguments}})
	req, _ := http.NewRequest("POST", server+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	req.Header.Set("Authorization", "Bearer local-r2-test")
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
		t.Fatalf("missing result: %#v", response)
	}
	value := result["structuredContent"].(map[string]any)
	var textValue map[string]any
	content := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if err = json.Unmarshal([]byte(content), &textValue); err != nil || !reflect.DeepEqual(textValue, value) {
		t.Fatal("text/structuredContent mismatch")
	}
	return result
}

func TestD31D34D36V2FullLiteControlledContinuationRPC(t *testing.T) {
	for _, grade := range []string{"direct", "requires_adaptation", "needs_inspection", "unknown_version", "bad_candidate"} {
		t.Run(grade, func(t *testing.T) {
			fixture := grade
			invalid := grade == "unknown_version" || grade == "bad_candidate"
			if invalid {
				fixture = "direct"
			}
			raw, err := os.ReadFile("../gateway/testdata/recovery-candidates/v2/" + fixture + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if grade == "unknown_version" {
				raw = bytes.Replace(raw, []byte(`"alternatives_version": 2`), []byte(`"alternatives_version": 99`), 1)
			}
			if grade == "bad_candidate" {
				raw = bytes.Replace(raw, []byte(`"public_path": "/v1/agent-apis/target/suggestions"`), []byte(`"public_path": "https://evil.example/run"`), 1)
			}
			var sourceCalls, targetCalls atomic.Int32
			parameter := "query"
			if grade != "direct" {
				parameter = "keyword"
			}
			const businessInput = "中文 A&B+query"
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-ID", "r2-mock-request")
				switch r.URL.Path {
				case "/v1/agent-apis/source/read":
					sourceCalls.Add(1)
					w.WriteHeader(503)
					_, _ = w.Write(raw)
				case "/v1/agent-apis/target/suggestions":
					targetCalls.Add(1)
					if r.URL.Query().Get(parameter) != businessInput || len(r.URL.Query()) != 1 {
						t.Errorf("wrong adapted input: %s", r.URL.RawQuery)
					}
					_, _ = w.Write([]byte(`{"ok":true}`))
				default:
					t.Errorf("unexpected dispatch %s", r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer upstream.Close()
			client, err := gateway.New(upstream.URL, time.Second, time.Second, 32768, 131072)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.EnableAlternatives(true)
			source, target := "source_read", "target_read"
			records := []model.Record{
				{APIID: "00000000-0000-4000-8000-000000000001", Revision: 1, ProviderID: "source", Name: "source", ToolName: &source, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/read", PublicPath: "/v1/agent-apis/source/read", ConfigPresent: true, UnknownParameters: "reject", Inputs: []model.Input{{Name: "query", Type: "string", Location: "query", Required: true}}},
				{APIID: "00000000-0000-4000-8000-000000000002", Revision: 1, ProviderID: "target", Name: "target", ToolName: &target, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/suggestions", PublicPath: "/v1/agent-apis/target/suggestions", ConfigPresent: true, UnknownParameters: "reject", Inputs: []model.Input{{Name: parameter, Type: "string", Location: "query", Required: true}}},
			}
			manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
			if err := manager.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, AlternativesEnabled: true}))
			defer server.Close()
			rpc := func(path, tool string, args map[string]any) map[string]any {
				return recoveryRPC(t, server.URL, path, tool, args)
			}
			detail := rpc("/mcp-lite", "get_api", map[string]any{"api": source})["structuredContent"].(map[string]any)
			var firstError map[string]any
			for _, path := range []string{"/mcp-lite", "/mcp/source", "/mcp"} {
				beforeSource, beforeTarget := sourceCalls.Load(), targetCalls.Load()
				var result map[string]any
				if path == "/mcp-lite" {
					result = rpc(path, "run_api", map[string]any{"api": source, "api_version": detail["api_version"], "input": map[string]any{"query": businessInput}})
				} else {
					result = rpc(path, source, map[string]any{"query": businessInput})
				}
				e := result["structuredContent"].(map[string]any)["error"].(map[string]any)
				if firstError == nil {
					firstError = e
				} else if !reflect.DeepEqual(firstError, e) {
					t.Fatalf("Full/Lite projected different candidates: %#v / %#v", firstError, e)
				}
				if invalid {
					if e["alternatives"] != nil || e["alternatives_meta"] != nil || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || e["recovery"].(map[string]any)["primary_action"] != "check_original_request" || sourceCalls.Load() != beforeSource+1 || targetCalls.Load() != beforeTarget {
						t.Fatalf("invalid candidates lost core or dispatched: %#v", e)
					}
					continue
				}
				c := e["alternatives"].([]any)[0].(map[string]any)
				continuation := c["continuation"].(map[string]any)["mcp"].(map[string]any)
				if continuation["endpoint"] != "/mcp-lite" || continuation["connect_before_use"] != true {
					t.Fatalf("Full continuation not explicitly connectable: %#v", continuation)
				}
				inspect := continuation["inspect"].(map[string]any)
				if inspect["tool"] != "get_api" || inspect["arguments"].(map[string]any)["api"] != target {
					t.Fatal("wrong inspect mapping")
				}
				fresh := rpc("/mcp-lite", "get_api", inspect["arguments"].(map[string]any))["structuredContent"].(map[string]any)
				if sourceCalls.Load() != beforeSource+1 || targetCalls.Load() != beforeTarget {
					t.Fatal("recommendation or inspect dispatched an extra call")
				}
				if fresh["api_version"] == nil {
					t.Fatal("no refreshed version")
				}
				if grade == "needs_inspection" {
					if continuation["execute_tool"] != nil || continuation["arguments"] != nil || continuation["refresh_api_version"] != nil || c["execution"] != nil {
						t.Fatalf("inspection contains execution template: %#v", c)
					}
				} else {
					if continuation["execute_tool"] != "run_api" || continuation["refresh_api_version"] != true {
						t.Fatal("missing existing high-grade continuation")
					}
					// A stale version and unadapted fields cannot dispatch a new call.
					stale := rpc("/mcp-lite", "run_api", map[string]any{"api": target, "api_version": detail["api_version"], "input": map[string]any{parameter: businessInput}})
					if stale["isError"] != true || targetCalls.Load() != beforeTarget {
						t.Fatal("stale version dispatched")
					}
					if grade == "requires_adaptation" {
						bad := rpc("/mcp-lite", "run_api", map[string]any{"api": target, "api_version": fresh["api_version"], "input": map[string]any{"query": businessInput}})
						if bad["isError"] != true || targetCalls.Load() != beforeTarget {
							t.Fatal("unadapted input dispatched")
						}
					}
					continued := rpc("/mcp-lite", "run_api", map[string]any{"api": target, "api_version": fresh["api_version"], "input": map[string]any{parameter: businessInput}})
					if continued["isError"] == true || targetCalls.Load() != beforeTarget+1 {
						t.Fatalf("explicit new request failed: %#v", continued)
					}
				}
				if dir := os.Getenv("MCP_R2_EVIDENCE_DIR"); dir != "" {
					label := map[string]string{"/mcp-lite": "lite", "/mcp/source": "scope", "/mcp": "full"}[path]
					b, _ := json.MarshalIndent(map[string]any{"grade": grade, "endpoint": path, "original_dispatches": sourceCalls.Load() - beforeSource, "test_driven_new_dispatches": targetCalls.Load() - beforeTarget, "recommendation_dispatches": 0, "inspection_dispatches": 0, "result": result}, "", "  ")
					if err := os.WriteFile(filepath.Join(dir, "mcp-rpc-v2-"+grade+"-"+label+".json"), b, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if invalid {
				return
			}
			records[1].Revision = 2
			if err := manager.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			result := rpc("/mcp/source", source, map[string]any{"query": businessInput})
			c := result["structuredContent"].(map[string]any)["error"].(map[string]any)["alternatives"].([]any)[0].(map[string]any)
			if c["continuation"] != nil || c["definition_path"] == nil {
				t.Fatal("revision mismatch must keep only HTTP details")
			}
			if grade == "needs_inspection" && targetCalls.Load() != 0 {
				t.Fatal("inspection dispatched target")
			}
		})
	}
}

func TestD37V2ModelContinuationRequiresCurrentNonStreamingChat(t *testing.T) {
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
	h := &Handler{options: Options{AlternativesEnabled: true, LiteEnabled: true}}
	for _, tc := range []struct {
		protocol, path string
		stream, want   bool
	}{
		{"openai-chat-completions", "/v1/chat/completions", false, true}, {"openai-chat-completions", "/v1/chat/completions", true, false}, {"openai-responses", "/v1/responses", false, false}, {"anthropic-messages", "/v1/messages", false, false}, {"openai-images", "/v1/images/generations", false, false}, {"openai-video", "/v1/videos", false, false}, {"openai-audio", "/v1/audio/speech", false, false},
	} {
		t.Run(tc.protocol+"/stream="+map[bool]string{true: "true", false: "false"}[tc.stream], func(t *testing.T) {
			raw, err := os.ReadFile("../gateway/testdata/recovery-candidates/v2/model.json")
			if err != nil {
				t.Fatal(err)
			}
			var v map[string]any
			d := json.NewDecoder(strings.NewReader(string(raw)))
			d.UseNumber()
			if err := d.Decode(&v); err != nil {
				t.Fatal(err)
			}
			c := v["details"].(map[string]any)["alternatives"].([]any)[0].(map[string]any)
			execution := c["execution"].(map[string]any)
			execution["protocol"], execution["path"], c["public_path"] = tc.protocol, tc.path, tc.path
			execution["request_overrides"].(map[string]any)["stream"] = tc.stream
			if tc.protocol == "openai-images" || tc.protocol == "openai-video" || tc.protocol == "openai-audio" {
				unit := "second"
				if tc.protocol == "openai-images" {
					unit = "image"
				}
				c["billing_unit"] = unit
				c["pricing"] = map[string]any{"status": "known", "scope": "public_catalog", "currency": "USD", "unit": unit, "amount": "0.01", "requires_quote": false}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				_ = json.NewEncoder(w).Encode(v)
			}))
			defer server.Close()
			client, _ := gateway.New(server.URL, time.Second, time.Second, 32768, 131072)
			defer client.Close()
			client.EnableAlternatives(true)
			r := client.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/test"}, nil, nil)
			r = h.withAlternatives(r, manager.Current().Discovery)
			candidate := r.Value["error"].(map[string]any)["alternatives"].([]any)[0].(map[string]any)
			if (candidate["continuation"] != nil) != tc.want {
				t.Fatalf("unsupported model execution advertised: %#v", candidate)
			}
			if tc.want {
				candidate["model_id"] = "not-in-catalog"
				delete(candidate, "continuation")
				h.withAlternatives(r, manager.Current().Discovery)
				if candidate["continuation"] != nil {
					t.Fatal("unavailable model advertised")
				}
			}
		})
	}
}

func TestD35V2ContinuationDoesNotExceedCandidateBound(t *testing.T) {
	name := "target_read"
	record := model.Record{APIID: "00000000-0000-4000-8000-000000000002", Revision: 1, ProviderID: "target", Name: "target", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/suggestions", PublicPath: "/v1/agent-apis/target/suggestions", ConfigPresent: true, UnknownParameters: "reject"}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../gateway/testdata/recovery-candidates/v2/direct.json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	c := v["details"].(map[string]any)["alternatives"].([]any)[0].(map[string]any)
	k := c["compatibility"].(map[string]any)
	differences := []any{}
	for i := 0; i < 8; i++ {
		differences = append(differences, map[string]any{"code": "contract_note", "message": strings.Repeat("x", 256)})
	}
	k["differences"] = differences
	k["input_mapping"] = []any{map[string]any{"from": "query:" + strings.Repeat("a", 220), "to": "query:" + strings.Repeat("b", 220)}}
	encoded, _ := json.Marshal(c)
	if len(encoded) > 4096 || len(encoded) < 3800 {
		t.Fatalf("fixture must be valid and near 4KiB: %d", len(encoded))
	}
	h := &Handler{options: Options{AlternativesEnabled: true, LiteEnabled: true}}
	r := gateway.Result{Value: map[string]any{"error": map[string]any{"alternatives_version": json.Number("2"), "alternatives": []any{c}}}}
	h.withAlternatives(r, manager.Current().Discovery)
	encoded, _ = json.Marshal(c)
	if len(encoded) > 4096 || c["definition_path"] == nil {
		t.Fatalf("projection exceeded bound or lost HTTP details: bytes=%d", len(encoded))
	}
}
