package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/account"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

func onboardingRPC(t *testing.T, h *Handler, path, method string, params any, headers http.Header) (int, map[string]any, http.Header) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, bytes.NewReader(raw))
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	result := make(map[string]any)
	if w.Code == http.StatusOK && json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatalf("invalid response for %s: %s", method, w.Body)
	}
	return w.Code, result, w.Header()
}

func accountToolCall(t *testing.T, h *Handler, name string, args map[string]any, headers http.Header) (map[string]any, map[string]any) {
	t.Helper()
	status, response, responseHeaders := onboardingRPC(t, h, "/mcp-lite", "tools/call", map[string]any{"name": name, "arguments": args}, headers)
	if status != http.StatusOK {
		t.Fatalf("tool call %s status %d", name, status)
	}
	if h.accountEnabled() && !strings.Contains(responseHeaders.Get("Cache-Control"), "no-store") {
		t.Errorf("account connection permits caching: %s", responseHeaders.Get("Cache-Control"))
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing tool result for %s: %v", name, response)
	}
	value, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("missing structured content: %v", result)
	}
	return result, value
}

func TestAccountToolAccessWithoutCatalogAndNoBusinessDispatch(t *testing.T) {
	var accountHits, businessHits atomic.Int32
	business := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		businessHits.Add(1)
	}))
	defer business.Close()
	gw, err := gateway.New(business.URL, time.Second, time.Second, 4096, 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer gw.Close()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/agent-api-account/onboarding" || r.URL.Query().Get("lang") != "ja" {
			t.Errorf("unexpected private account request: %s %s", r.Method, r.URL)
		}
		accountHits.Add(1)
		auth := r.Header.Get("Authorization")
		available := ""
		switch {
		case auth == "Bearer example-1":
			available = "25000"
		case r.Header.Get("X-Api-Key") == "example-2":
			available = "95000"
		default:
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"code":"UNAUTHORIZED","requestId":"request-1","message":"upstream reply"}`)
			return
		}
		fmt.Fprintf(w, `{"data":{"version":1,"authentication":"authenticated","businessRequestStarted":false,"balance":{"status":"available","availableMicroUsd":%q,"accountId":"not-public"},"signupCredit":{"status":"unknown"},"apiKey":{"status":"available","remainingMicroUsd":null},"referral":{"status":"restricted","referralLink":"private","earnedMicroUsd":"10","email":"private"}}}`, available)
	}))
	defer backend.Close()
	client, err := account.NewClient(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false)
	h := New(manager, gw, Options{LiteEnabled: true, AccountClient: client, ModelGateway: gw})
	status, initialized, _ := onboardingRPC(t, h, "/", "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t", "version": "0"}}, nil)
	if status != http.StatusOK || initialized["result"] == nil {
		t.Fatal("Lite initialization needs catalog", status, initialized)
	}
	_, listed, _ := onboardingRPC(t, h, "/", "tools/list", map[string]any{}, nil)
	tools := listed["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 5 {
		t.Fatal("Lite tool count changed", len(tools))
	}
	for _, value := range tools {
		tool := value.(map[string]any)
		if tool["name"] == "get_account" {
			annotations := tool["annotations"].(map[string]any)
			properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
			if annotations["readOnlyHint"] != true || len(properties) != 1 || properties["lang"] == nil {
				t.Fatal("account tool has unsafe schema or annotations", tool)
			}
		}
	}
	_, missing := accountToolCall(t, h, "get_account", map[string]any{}, nil)
	if missing["authentication"] != "missing" || missing["businessRequestStarted"] != false || missing["primaryAction"] != "register_or_configure" {
		t.Fatal("missing account guidance", missing)
	}
	_, invalidArgs := accountToolCall(t, h, "get_account", map[string]any{"key": "secret"}, nil)
	if invalidArgs["error"].(map[string]any)["code"] != "openany_mcp_invalid_input" {
		t.Fatal("tool accepted a key argument", invalidArgs)
	}
	_, placeholder := accountToolCall(t, h, "get_account", map[string]any{}, http.Header{"Authorization": {"Bearer ${OPENANY_API_KEY}"}})
	if placeholder["error"].(map[string]any)["code"] != gateway.PlaceholderCredentialCode || accountHits.Load() != 0 {
		t.Fatal("placeholder forwarded upstream", placeholder)
	}
	_, unavailable := accountToolCall(t, h, "openany_overview", map[string]any{}, nil)
	if unavailable["error"].(map[string]any)["code"] != "openany_mcp_catalog_unavailable" {
		t.Fatal("catalog returned a misleading result", unavailable)
	}
	for _, identity := range []struct {
		name, header, key, balance string
	}{{"one", "Authorization", "example-1", "25000"}, {"two", "X-Api-Key", "example-2", "95000"}, {"one again", "Authorization", "example-1", "25000"}} {
		headers := make(http.Header)
		headers.Set(identity.header, identity.key)
		result, value := accountToolCall(t, h, "get_account", map[string]any{"lang": "ja"}, headers)
		balance := value["balance"].(map[string]any)
		referral := value["referral"].(map[string]any)
		key := value["apiKey"].(map[string]any)
		remaining, present := key["remainingMicroUsd"]
		if result["isError"] == true || value["authentication"] != "authenticated" || balance["availableMicroUsd"] != identity.balance ||
			balance["accountId"] != nil || referral["referralLink"] != nil || referral["earnedMicroUsd"] != nil || referral["email"] != nil ||
			referral["status"] != "restricted" || !present || remaining != nil {
			t.Errorf("incorrect private state for %s: %v", identity.name, value)
		}
	}
	result, invalid := accountToolCall(t, h, "get_account", map[string]any{"lang": "ja"}, http.Header{"Authorization": {"wrong"}})
	if result["isError"] != true || invalid["authentication"] != "invalid" || invalid["request_id"] != "request-1" ||
		invalid["http_status"] != float64(401) || invalid["primaryAction"] != "replace_credentials" || invalid["error"].(map[string]any)["code"] != "UNAUTHORIZED" {
		t.Fatal("invalid key lost its failure state", invalid)
	}
	ready := httptest.NewRecorder()
	h.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "http://localhost/readyz", nil))
	full, _, _ := onboardingRPC(t, h, "/mcp", "tools/list", map[string]any{}, nil)
	if ready.Code != http.StatusServiceUnavailable || full != http.StatusServiceUnavailable || accountHits.Load() != 4 || businessHits.Load() != 0 {
		t.Fatalf("readiness or request counts changed: ready=%d full=%d account=%d business=%d", ready.Code, full, accountHits.Load(), businessHits.Load())
	}
}

func TestOnboardingFlagsLeaveAccountToolUnpublished(t *testing.T) {
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false)
	h := New(manager, nil, Options{LiteEnabled: true, BenefitsEnabled: true})
	_, result, _ := onboardingRPC(t, h, "/", "tools/list", map[string]any{}, nil)
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "get_account") || strings.Contains(string(raw), "$0.10") {
		t.Fatal("published disabled tool or an unverified amount", result)
	}
}
