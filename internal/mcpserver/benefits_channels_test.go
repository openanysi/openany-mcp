package mcpserver

import (
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

	"github.com/openany/openany-mcp/internal/account"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

const mixedChannelDescription = "Eligible Agent newcomers automatically receive trial credit: $0.25 for Agent wallet registration; $0.10 for CLI email registration."

func channelBenefitFixtures(t *testing.T) map[string]map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../account/testdata/onboarding-channel-amounts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]struct{ Data map[string]any }
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]map[string]any)
	for name, value := range fixtures {
		result[name] = value.Data
	}
	return result
}

func TestTrialDescriptionUsesChannelAmounts(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		change        func(map[string]any)
		want          string
	}{
		{"legacy_equal", "equal", nil, "Eligible Agent newcomers automatically receive $0.10 in credit."},
		{"mixed", "mixed", nil, mixedChannelDescription},
		{"wallet_disabled", "wallet_disabled", nil, "Eligible Agent newcomers automatically receive $0.10 in credit."},
		{"disabled", "disabled", nil, ""},
		{"explicit_equal", "equal", func(c map[string]any) {
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "100000", "cli_email": "100000"}
		}, "Eligible Agent newcomers automatically receive $0.10 in credit."},
		{"email_first", "mixed", func(c map[string]any) {
			c["eligibleChannels"] = []string{"cli_email", "agent_wallet"}
			c["amountMicroUsd"] = "100000"
		}, "Eligible Agent newcomers automatically receive trial credit: $0.10 for CLI email registration; $0.25 for Agent wallet registration."},
		{"micro_amounts", "mixed", func(c map[string]any) {
			c["amountMicroUsd"] = "1"
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "1", "cli_email": "25"}
		}, "Eligible Agent newcomers automatically receive trial credit: $0.000001 for Agent wallet registration; $0.000025 for CLI email registration."},
		{"missing_channel", "mixed", func(c map[string]any) { c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "250000"} }, ""},
		{"invalid_channel_amount", "mixed", func(c map[string]any) {
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "250000", "cli_email": "-1"}
		}, ""},
		{"unknown_channel", "mixed", func(c map[string]any) {
			c["eligibleChannels"] = []string{"agent_wallet", "unknown-untrusted-channel"}
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "250000", "unknown-untrusted-channel": "100000"}
		}, "Eligible Agent newcomers automatically receive channel-specific trial credit. See the current trial-credit terms."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := channelBenefitFixtures(t)[tc.fixture]
			if tc.change != nil {
				tc.change(p["signupCredit"].(map[string]any))
			}
			raw, _ := json.Marshal(p)
			var policy account.Policy
			if err := json.Unmarshal(raw, &policy); err != nil {
				t.Fatal(err)
			}
			if got := trialDescription(&policy); got != tc.want {
				t.Fatalf("description = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChannelBenefitsAcrossMCPEntrances(t *testing.T) {
	fixtures := channelBenefitFixtures(t)
	for _, name := range []string{"equal", "mixed", "wallet_disabled", "disabled"} {
		t.Run(name, func(t *testing.T) {
			var businessHits, accountHits atomic.Int32
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Each response owns its language view; PolicyCache refreshes four languages concurrently.
				raw, _ := json.Marshal(fixtures[name])
				var policy map[string]any
				_ = json.Unmarshal(raw, &policy)
				policy["lang"] = r.URL.Query().Get("lang")
				var data any = policy
				switch r.URL.Path {
				case "/api/v1/agent-onboarding":
					if r.Header.Get("Authorization") != "" {
						t.Error("public policy received a credential")
					}
				case "/api/v1/agent-api-account/onboarding":
					accountHits.Add(1)
					if r.Header.Get("Authorization") != "Bearer local-test-key" {
						t.Error("unexpected account credential")
					}
					data = map[string]any{"version": 1, "authentication": "authenticated", "businessRequestStarted": false, "primaryAction": "review_account", "publicBenefits": policy,
						"balance": map[string]string{"status": "available", "availableMicroUsd": "70000"}, "signupCredit": map[string]string{"status": "credited", "grantedAmountMicroUsd": "100000"}, "referral": map[string]string{"status": "restricted"}}
				default:
					businessHits.Add(1)
					t.Error("unexpected business dispatch", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer backend.Close()
			client, err := account.NewClient(backend.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			cache := account.NewPolicyCache(client)
			cache.Refresh(t.Context())
			toolName := "demo_weather"
			manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) {
				return []model.Record{{APIID: toolName, Name: "Weather", Description: "Weather forecast", ProviderID: "demo", ToolName: &toolName, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/weather", PublicPath: "/v1/agent-apis/demo/weather", UnknownParameters: "reject", ConfigPresent: true}}, nil
			}, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
			if err := manager.Refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			h := New(manager, nil, Options{LiteEnabled: true, AccountClient: client, BenefitsEnabled: true, PolicyCache: cache})
			beforeHash := manager.Current().Hash
			var beforeVersion any
			observed := map[string]any{}
			wantTrial := "Eligible Agent newcomers automatically receive $0.10 in credit."
			if name == "mixed" {
				wantTrial = mixedChannelDescription
			} else if name == "disabled" {
				wantTrial = ""
			}
			for _, path := range []string{"/", "/mcp-lite", "/mcp"} {
				status, reply, _ := onboardingRPC(t, h, path, "tools/list", map[string]any{}, nil)
				if status != http.StatusOK || reply["error"] != nil {
					t.Fatal("tools/list failed", path, status, reply)
				}
				tools := reply["result"].(map[string]any)["tools"].([]any)
				for _, item := range tools {
					tool := item.(map[string]any)
					if tool["name"] == "run_api" {
						if strings.Contains(tool["description"].(string), "$") {
							t.Fatal("benefit added to execution tool")
						}
						continue
					}
					got := tool["description"].(string)
					if wantTrial != "" && !strings.Contains(got, wantTrial) || wantTrial == "" && strings.Contains(got, "$") {
						t.Fatalf("incorrect offer in %s/%s: %s", path, tool["name"], got)
					}
				}
				observed[path+" tools/list"] = reply["result"]
			}
			for _, path := range []string{"/", "/mcp-lite"} {
				for _, lang := range []string{"en", "zh", "ja", "ar"} {
					for _, authenticated := range []bool{false, true} {
						var headers http.Header
						if authenticated {
							headers = http.Header{"Authorization": {"Bearer local-test-key"}}
						}
						status, reply, _ := onboardingRPC(t, h, path, "tools/call", map[string]any{"name": "get_account", "arguments": map[string]any{"lang": lang}}, headers)
						if status != http.StatusOK || reply["error"] != nil {
							t.Fatal("get_account failed", reply)
						}
						result := reply["result"].(map[string]any)
						value := result["structuredContent"].(map[string]any)
						public := value["publicBenefits"].(map[string]any)
						if public["lang"] != lang || !reflect.DeepEqual(public["signupCredit"], fixtures[name]["signupCredit"]) || value["businessRequestStarted"] != false {
							t.Fatal("get_account lost channel rules or state", value)
						}
						if authenticated && (value["balance"].(map[string]any)["availableMicroUsd"] != "70000" || value["signupCredit"].(map[string]any)["grantedAmountMicroUsd"] != "100000") {
							t.Fatal("offer replaced actual account money", value)
						}
						var textValue map[string]any
						if err := json.Unmarshal([]byte(result["content"].([]any)[0].(map[string]any)["text"].(string)), &textValue); err != nil || !reflect.DeepEqual(textValue, value) {
							t.Fatal("text and structured account results differ", err)
						}
						observed[path+" get_account "+lang+" authenticated="+map[bool]string{false: "false", true: "true"}[authenticated]] = value
					}
				}
			}
			for _, call := range []struct {
				name string
				args map[string]any
			}{
				{"openany_overview", map[string]any{}}, {"search_apis", map[string]any{"query": "weather"}}, {"get_api", map[string]any{"api": toolName}},
			} {
				_, value := accountToolCall(t, h, call.name, call.args, nil)
				public := value["benefits"].(map[string]any)["publicPolicy"].(map[string]any)
				if !reflect.DeepEqual(public["signupCredit"], fixtures[name]["signupCredit"]) {
					t.Fatal("discovery entrance lost channel rules", call.name, value)
				}
				if call.name == "get_api" {
					beforeVersion = value["api_version"]
				}
				observed[call.name] = value
			}
			cache.Refresh(t.Context())
			_, detail := accountToolCall(t, h, "get_api", map[string]any{"api": toolName}, nil)
			if beforeVersion == nil || detail["api_version"] != beforeVersion || manager.Current().Hash != beforeHash || businessHits.Load() != 0 || accountHits.Load() != 8 {
				t.Fatal("benefits affected catalog, API version or business dispatch", businessHits.Load(), accountHits.Load())
			}
			if dir := os.Getenv("CHANNEL_BENEFITS_EVIDENCE_DIR"); dir != "" {
				raw, err := json.MarshalIndent(map[string]any{"case": name, "results": observed, "businessDispatches": businessHits.Load(), "privateAccountReads": accountHits.Load()}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "mcp-entrances-"+name+".json"), append(raw, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
