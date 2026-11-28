package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/account"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func mergedGuideManager(t *testing.T) *catalog.Manager {
	t.Helper()
	name := "guide_weather"
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) {
		return []model.Record{{APIID: name, ToolName: &name, Name: "Weather", Description: "Weather forecast", ProviderID: "guide", Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/weather", PublicPath: "/v1/agent-apis/guide/weather", UnknownParameters: "reject", ConfigPresent: true}}, nil
	}, model.Overrides{Version: 1}, time.Minute, false)
	if err := manager.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestMergedGuideToolCountsMatchRuntime(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		count   int
	}{{"default_account_enabled", true, 5}, {"explicit_account_disabled", false, 4}} {
		for _, path := range []string{"/", "/mcp-lite"} {
			t.Run(tc.name+path, func(t *testing.T) {
				opts := Options{LiteEnabled: true}
				if tc.enabled {
					client, err := account.NewClient("http://127.0.0.1:1")
					if err != nil {
						t.Fatal(err)
					}
					defer client.Close()
					opts.AccountClient = client
				}
				// No gateway is configured. Listing tools and an anonymous account
				// lookup must not dispatch a business request or query an account.
				h := New(mergedGuideManager(t), nil, opts)
				status, response, _ := onboardingRPC(t, h, path, "tools/list", map[string]any{}, nil)
				if status != http.StatusOK || response["error"] != nil {
					t.Fatal("tool listing failed", status, response)
				}
				tools := response["result"].(map[string]any)["tools"].([]any)
				names := make(map[string]bool)
				for _, tool := range tools {
					names[tool.(map[string]any)["name"].(string)] = true
				}
				if len(tools) != tc.count || names["get_account"] != tc.enabled {
					t.Fatalf("guide/runtime mismatch: %v", names)
				}
				for _, required := range []string{"openany_overview", "search_apis", "get_api", "run_api"} {
					if !names[required] {
						t.Fatal("existing tool lost", required)
					}
				}
				if tc.enabled {
					_, reply, _ := onboardingRPC(t, h, path, "tools/call", map[string]any{"name": "get_account", "arguments": map[string]any{}}, nil)
					value := reply["result"].(map[string]any)["structuredContent"].(map[string]any)
					if value["authentication"] != "missing" || value["businessRequestStarted"] != false {
						t.Fatal("documented keyless account behavior changed", value)
					}
				}
			})
		}
	}
}

func TestMergedGuideServesMainPresentationAndDevFeatures(t *testing.T) {
	h := New(mergedGuideManager(t), nil, Options{LiteEnabled: true})
	for _, path := range []string{"/", "/docs/mcp"} {
		for _, lang := range []string{"en", "zh"} {
			for _, media := range []string{"text/plain", "text/html"} {
				t.Run(path+lang+media, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodGet, "http://localhost"+path+"?lang="+lang, nil)
					req.Header.Set("Accept", media)
					response := httptest.NewRecorder()
					h.ServeHTTP(response, req)
					body := response.Body.String()
					if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), media) {
						t.Fatal("guide response changed", response.Code, response.Header())
					}
					for _, want := range []string{"get_account", "amountMicroUsdByChannel", "MCP_ACCOUNT_ENABLED=false", "MCP_ALTERNATIVES_ENABLED", "error.recovery", "error.alternatives", "direct", "requires_adaptation", "needs_inspection", "error.related_models", "api_version", "request_id", "OAuth", "Streamable HTTP", "Chat Completions", "DataForSEO", "Open-Meteo"} {
						if !strings.Contains(body, want) {
							t.Fatal("merged guide lost feature or boundary", want)
						}
					}
					five := "expose five tools"
					if lang == "zh" {
						five = "提供五个工具"
					}
					if !strings.Contains(body, five) {
						t.Fatal("default five-tool statement missing")
					}
					for _, stale := range []string{"<<<<<<<", "=======", ">>>>>>>", "4,400+", "168+", "Confirm the four tools appear", "确认四个工具可见", "<!-- operations-docs -->"} {
						if strings.Contains(body, stale) {
							t.Fatal("merge marker or obsolete statement exposed", stale)
						}
					}
					if media == "text/html" && (!strings.Contains(body, "data-copy-endpoint") || strings.Count(body, "<details open>") != 1 || strings.Count(body, "<details><summary>") != 1 || !strings.Contains(body, `class="table-scroll"`)) {
						t.Fatal("main page presentation lost")
					}
				})
			}
		}
	}
}
