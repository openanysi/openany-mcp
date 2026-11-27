package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/account"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestBenefitPresentationUpdatesWithoutChangingCatalogVersionsOrCursors(t *testing.T) {
	var state atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-onboarding" || r.Header.Get("Authorization") != "" {
			t.Error("non-public request from benefit presentation")
		}
		phase := state.Load()
		credit, creditStatus, channels := "100000", "enabled", `["agent_wallet"]`
		switch phase {
		case 1:
			credit = "150000"
		case 2:
			credit, creditStatus, channels = "0", "disabled", `[]`
		case 3:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, `{"data":{"version":1,"status":"available","policyRevision":"revision-%d","lang":%q,"links":{"account":"https://openany.si/dashboard?page=api-keys","publicPolicy":"https://openany.si/api/v1/agent-onboarding"},"signupCredit":{"automatic":true,"status":%q,"currency":"USD","eligibleChannels":%s,"amountMicroUsd":%q},"referralReward":{"automatic":true,"recipient":"inviter","calculation":"marginal_tiers"},"email":"hidden"}}`,
			phase, r.URL.Query().Get("lang"), creditStatus, channels, credit)
	}))
	defer backend.Close()
	client, err := account.NewClient(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cache := account.NewPolicyCache(client)
	cache.Refresh(t.Context())
	const api = "demo_weather_00"
	var records []model.Record
	for i := range 51 {
		name := fmt.Sprintf("demo_weather_%02d", i)
		records = append(records, model.Record{APIID: name, Name: "Weather", Description: "Weather forecast", ProviderID: "demo", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/weather/" + name, PublicPath: "/v1/agent-apis/demo/weather/" + name, UnknownParameters: "reject", ConfigPresent: true})
	}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false,
		func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil })
	if err := manager.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	h := New(manager, nil, Options{LiteEnabled: true, AccountClient: client, BenefitsEnabled: true, PolicyCache: cache})
	getDescriptions := func(path string, args map[string]any) (map[string]string, any) {
		t.Helper()
		status, response, _ := onboardingRPC(t, h, path, "tools/list", args, nil)
		if status != http.StatusOK || response["error"] != nil {
			t.Fatalf("could not list %s: status %d %v", path, status, response)
		}
		list := response["result"].(map[string]any)
		descriptions := make(map[string]string)
		for _, raw := range list["tools"].([]any) {
			tool := raw.(map[string]any)
			descriptions[tool["name"].(string)] = tool["description"].(string)
		}
		return descriptions, list["nextCursor"]
	}
	lite, _ := getDescriptions("/mcp-lite", map[string]any{})
	for _, name := range []string{"openany_overview", "search_apis", "get_api", "get_account"} {
		if !strings.Contains(lite[name], "$0.10") {
			t.Errorf("missing current offer in %s", name)
		}
	}
	if strings.Contains(lite["run_api"], "$0.10") {
		t.Fatal("execution description changed")
	}
	full, fullCursor := getDescriptions("/mcp", map[string]any{})
	if fullCursor == nil || !strings.Contains(full[api], "$0.10") || strings.Contains(full[api], "get_account") || full["get_account"] != "" {
		t.Fatal("Full Catalog scope or benefits missing", full[api], fullCursor)
	}
	_, overview := accountToolCall(t, h, "openany_overview", map[string]any{}, nil)
	if overview["api_count"] != float64(51) || overview["model_count"] != float64(0) || overview["example_questions"] == nil ||
		overview["benefits"].(map[string]any)["checkAccount"].(map[string]any)["tool"] != "get_account" {
		t.Fatal("overview lost live catalog or account action", overview)
	}
	_, anon := accountToolCall(t, h, "get_account", map[string]any{"lang": "ar"}, nil)
	if policy := anon["publicBenefits"].(map[string]any); policy["lang"] != "ar" || policy["email"] != nil {
		t.Fatal("anonymous account did not return localized public-only policy", anon)
	}
	searchArgs := map[string]any{"query": "weather", "limit": 1}
	_, search := accountToolCall(t, h, "search_apis", searchArgs, nil)
	searchCursor := search["next_cursor"]
	if searchCursor == nil || search["benefits"] == nil {
		t.Fatal("search changed pagination or dropped benefits", search)
	}
	_, before := accountToolCall(t, h, "get_api", map[string]any{"api": api}, nil)
	beforeHash := manager.Current().Hash
	if before["benefits"] == nil || before["api_version"] == nil {
		t.Fatal("missing detail benefits or version", before)
	}

	state.Store(1)
	cache.Refresh(t.Context())
	lite, _ = getDescriptions("/mcp-lite", map[string]any{})
	full, _ = getDescriptions("/mcp", map[string]any{"cursor": fullCursor})
	if !strings.Contains(lite["get_api"], "$0.15") || strings.Contains(lite["get_api"], "$0.10") || !strings.Contains(full["demo_weather_50"], "$0.15") {
		t.Fatal("tools list failed to present refreshed revision", lite["get_api"], full)
	}
	searchArgs["cursor"] = searchCursor
	_, continued := accountToolCall(t, h, "search_apis", searchArgs, nil)
	if continued["error"] != nil || len(continued["results"].([]any)) != 1 {
		t.Fatal("benefit refresh invalidated search cursor", continued)
	}
	_, after := accountToolCall(t, h, "get_api", map[string]any{"api": api}, nil)
	if after["api_version"] != before["api_version"] || after["description"] != before["description"] || manager.Current().Hash != beforeHash {
		t.Fatal("benefit refresh changed catalog contract")
	}

	state.Store(2)
	cache.Refresh(t.Context())
	lite, _ = getDescriptions("/mcp-lite", map[string]any{})
	if strings.Contains(lite["openany_overview"], "$0.") || !strings.Contains(lite["openany_overview"], "credit the inviter") {
		t.Fatal("disabled trial offer was advertised or referral changed", lite["openany_overview"])
	}
	state.Store(3)
	cache.Refresh(t.Context())
	lite, _ = getDescriptions("/mcp-lite", map[string]any{})
	_, fallback := accountToolCall(t, h, "get_api", map[string]any{"api": api}, nil)
	policy := fallback["benefits"].(map[string]any)["publicPolicy"].(map[string]any)
	if strings.Contains(lite["get_api"], "$0.") || policy["status"] != "unavailable" || policy["signupCredit"] != nil {
		t.Fatal("unavailable public policy retained an offer", fallback)
	}
}

func TestBenefitServerCardRemainsBounded(t *testing.T) {
	h := New(nil, nil, Options{LiteEnabled: true, BenefitsEnabled: true})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/server-card", nil))
	if w.Code != http.StatusOK || w.Header().Get("ETag") == serverCardETag || !strings.Contains(w.Body.String(), serverCardBenefits) ||
		len(serverCardBenefits) > 100 || strings.Contains(w.Body.String(), "$0.10") {
		t.Fatalf("invalid benefits server card %d %s", w.Code, w.Body)
	}
}
