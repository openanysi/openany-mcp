package config_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/openany/openany-mcp/internal/account"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/config"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

func TestD48ConfiguredOriginsReachOnlyTheirDistinctLoopbackServices(t *testing.T) {
	var accountHits, gatewayHits, modelsHits, unexpected atomic.Int32
	const fakeAuthorization = "Bearer local-origin-test-only"
	check := func(w http.ResponseWriter, r *http.Request, path, authorization string) bool {
		t.Helper()
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != path || r.Header.Get("Authorization") != authorization {
			unexpected.Add(1)
			t.Errorf("cross-service or invalid request: method=%s path=%s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return false
		}
		return true
	}
	accountServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountHits.Add(1)
		if !check(w, r, "/api/v1/agent-api-account/onboarding", fakeAuthorization) {
			return
		}
		if r.URL.RawQuery != "lang=zh" {
			t.Error("account language query changed")
		}
		fmt.Fprint(w, `{"data":{"version":1,"authentication":"authenticated","businessRequestStarted":false,"balance":{"status":"available","availableMicroUsd":"123456"},"apiKey":{"status":"available","remainingMicroUsd":null},"signupCredit":{"status":"unknown"},"referral":{"status":"restricted"}}}`)
	}))
	defer accountServer.Close()
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gatewayHits.Add(1)
		if !check(w, r, "/v1/agent-apis/local/read", fakeAuthorization) {
			return
		}
		if r.URL.RawQuery != "" {
			t.Error("gateway request unexpectedly gained query parameters")
		}
		w.Header().Set("X-Request-ID", "origin-local-failure")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"type":"openany_error","code":"openany_upstream_service_error","openany_cost":0,"details":{"recovery":{"version":1,"reason":"upstream_failure","execution_state":"failed","charge_state":"not_charged","primary_action":"check_original_request","next_step_target":"https://openany.si/dashboard?page=usage"}}}`)
	}))
	defer gatewayServer.Close()
	modelsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modelsHits.Add(1)
		if !check(w, r, "/v1/models", "") {
			return
		}
		if r.URL.RawQuery != "" {
			t.Error("public model catalog unexpectedly gained query parameters")
		}
		fmt.Fprint(w, `{"data":[{"id":"origin-local-model","name":"Origin local fixture","input_modalities":["text"],"output_modalities":["text"]}]}`)
	}))
	defer modelsServer.Close()

	t.Setenv("POSTGRES_DATABASE_URL", "postgres://127.0.0.1/origin_fixture_not_opened")
	t.Setenv("MCP_ACCOUNT_BASE_URL", accountServer.URL+"/")
	t.Setenv("OPENANY_GATEWAY_BASE_URL", gatewayServer.URL)
	t.Setenv("OPENANY_MODELS_BASE_URL", modelsServer.URL+"/v1")
	t.Setenv("MCP_ACCOUNT_ENABLED", "true")
	t.Setenv("MCP_BENEFITS_ENABLED", "false")
	t.Setenv("MCP_ALTERNATIVES_ENABLED", "true")
	t.Setenv("MCP_LITE_ENABLED", "true")
	t.Setenv("MCP_DATABASE_QUERY_TIMEOUT", "1s")
	t.Setenv("MCP_GATEWAY_TIMEOUT", "2s")
	t.Setenv("MCP_GATEWAY_CONNECT_TIMEOUT", "250ms")
	t.Setenv("MCP_SHUTDOWN_TIMEOUT", "3s")
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Refuse to construct a client if configuration selected a default or
	// non-loopback endpoint. The assertions below also require real HTTP hits.
	origins := map[string]string{"account": c.AccountOrigin, "gateway": c.GatewayURL, "models": c.ModelsOrigin}
	expected := map[string]string{"account": accountServer.URL, "gateway": gatewayServer.URL, "models": modelsServer.URL}
	seen := map[string]bool{}
	for service, origin := range origins {
		u, parseErr := url.Parse(origin)
		if parseErr != nil || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() || origin != expected[service] || seen[origin] {
			t.Fatalf("configuration crossed services or selected a default origin: %s", service)
		}
		seen[origin] = true
	}

	accountClient, err := account.NewClient(c.AccountOrigin)
	if err != nil {
		t.Fatal(err)
	}
	defer accountClient.Close()
	headers := http.Header{"Authorization": {fakeAuthorization}}
	summary, err := accountClient.Summary(t.Context(), "zh", headers)
	if err != nil || summary == nil || summary.Balance.AvailableMicroUSD != "123456" || summary.BusinessRequestStarted == nil || *summary.BusinessRequestStarted {
		t.Fatalf("configured account read did not return its local marker: %v", err)
	}
	models, err := catalog.HTTPModelLoader(c.ModelsOrigin, c.QueryTimeout)(t.Context())
	if err != nil || len(models) != 1 || models[0].ID != "origin-local-model" {
		t.Fatalf("configured models read did not return its local marker: %v", err)
	}
	client, err := gateway.New(c.GatewayURL, c.GatewayTimeout, c.ConnectTimeout, c.MaxResponse, c.MaxResult)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.EnableAlternatives(c.AlternativesEnabled)
	result := client.Call(t.Context(), model.Request{Method: http.MethodGet, PublicPath: "/v1/agent-apis/local/read", RequestContentType: "none"}, nil, headers)
	e, ok := result.Value["error"].(map[string]any)
	if !result.IsError || !ok || e["code"] != "openany_upstream_service_error" || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || result.Value["request_id"] != "origin-local-failure" || result.Value["http_status"] != 503 {
		t.Fatalf("configured gateway call did not retain its local failure: %#v", result.Value)
	}
	if accountHits.Load() != 1 || gatewayHits.Load() != 1 || modelsHits.Load() != 1 || unexpected.Load() != 0 {
		t.Fatalf("unexpected routing or repeated business dispatch: account=%d gateway=%d models=%d unexpected=%d", accountHits.Load(), gatewayHits.Load(), modelsHits.Load(), unexpected.Load())
	}
	if dir := os.Getenv("MCP_ORIGIN_EVIDENCE_DIR"); dir != "" {
		run := os.Getenv("MCP_ORIGIN_EVIDENCE_RUN")
		if run != "focused" && run != "all" && run != "race" {
			t.Fatal("invalid origin evidence run label")
		}
		b, err := json.MarshalIndent(map[string]any{"test": "TestD48ConfiguredOriginsReachOnlyTheirDistinctLoopbackServices", "scope": "config.Load and real client constructors/methods; three independent local Mock HTTP services; no MCP process or real provider", "configured_origins": origins, "account_read_requests": accountHits.Load(), "models_catalog_read_requests": modelsHits.Load(), "original_gateway_requests": gatewayHits.Load(), "cross_service_requests": unexpected.Load(), "extra_business_dispatches": gatewayHits.Load() - 1, "gateway_http_status": result.Value["http_status"], "execution_state": e["execution_state"], "charge_state": e["charge_state"], "supplier_calls": 0}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "mcp-origin-three-servers-"+run+".json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
