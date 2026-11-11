package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
)

func TestRunAPIMissingAndRejectedKeys(t *testing.T) {
	var hits atomic.Int32
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "gw-auth")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"openany_error","code":"openany_authentication_invalid","message":"invalid"}`))
	}))
	defer gw.Close()
	client, err := gateway.New(gw.URL, time.Second, time.Second, 4096, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	name := "demo_weather"
	record := model.Record{APIID: name, Name: "Weather", Description: "Forecast", ProviderID: "demo", ToolName: &name, Status: "active", Method: "GET", RequestContentType: "none", PathTemplate: "/weather", PublicPath: "/v1/agent-apis/demo/weather", UnknownParameters: "reject", ConfigPresent: true}
	manager := catalog.NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		return map[string]map[string]any{}, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(manager, client, Options{LiteEnabled: true, ModelGateway: client}))
	defer server.Close()
	detail := callTool(t, server.URL, "/mcp-lite", "get_api", map[string]any{"api": name}, "")
	version := detail["api_version"]
	args := map[string]any{"api": name, "api_version": version, "input": map[string]any{}}
	before := hits.Load()
	missing := callTool(t, server.URL, "/", "run_api", args, "")
	if missing == nil {
		t.Fatal("missing result")
	}
	if hits.Load() != before {
		t.Fatal("missing key reached the gateway", hits.Load())
	}
	errObj := missing["error"].(map[string]any)
	if errObj["code"] != "openany_mcp_credentials_missing" || errObj["execution_state"] != "not_started" || missing["key_url"] != keyPageURL {
		t.Fatal(missing)
	}
	text := errObj["message"].(string) + errObj["next_action"].(string)
	for _, phrase := range []string{keyPageURL, "Authorization: Bearer <key>", "Do not ask the user to paste the key into the chat"} {
		if !bytes.Contains([]byte(text), []byte(phrase)) {
			t.Fatal(phrase, text)
		}
	}
	if _, ok := missing["elicitation"]; ok {
		t.Fatal("url elicitation was invented", missing["elicitation"])
	}
	blank := callTool(t, server.URL, "/mcp-lite", "run_api", args, "Bearer")
	if blank["error"].(map[string]any)["code"] != "openany_mcp_credentials_missing" || hits.Load() != before {
		t.Fatal(blank, hits.Load())
	}
	direct := callDirect(t, server.URL, "/mcp", name, map[string]any{}, "")
	if direct["error"].(map[string]any)["code"] != "openany_mcp_credentials_missing" || hits.Load() != before {
		t.Fatal("full catalog forwarded a keyless call", direct, hits.Load())
	}
	rejected := callTool(t, server.URL, "/", "run_api", args, "Bearer expired-key")
	if hits.Load() != before+1 {
		t.Fatal("rejected key did not reach the gateway once", hits.Load(), before)
	}
	rejectedErr := rejected["error"].(map[string]any)
	if rejectedErr["code"] != "openany_authentication_invalid" || rejected["http_status"] != float64(401) || rejected["request_id"] != "gw-auth" || rejected["key_url"] != keyPageURL {
		t.Fatal(rejected)
	}
	rejectedText := rejectedErr["message"].(string) + rejectedErr["next_action"].(string)
	for _, phrase := range []string{"invalid, expired, or revoked", keyPageURL, "Do not ask the user to paste a key into the chat"} {
		if !bytes.Contains([]byte(rejectedText), []byte(phrase)) {
			t.Fatal(phrase, rejectedText)
		}
	}
	withHeader := callRaw(t, server.URL, "/mcp-lite", "run_api", args, map[string]string{"X-Api-Key": "present-key"})
	if withHeader["error"].(map[string]any)["code"] != "openany_authentication_invalid" || hits.Load() != before+2 {
		t.Fatal("x-api-key was not treated as a credential", withHeader, hits.Load())
	}
	urlClient := callRaw(t, server.URL, "/", "run_api", args, nil, map[string]any{
		"io.modelcontextprotocol/clientCapabilities": map[string]any{"elicitation": map[string]any{"url": map[string]any{}}},
	})
	if urlClient["error"].(map[string]any)["code"] != "openany_mcp_credentials_missing" || hits.Load() != before+2 {
		t.Fatal(urlClient, hits.Load())
	}
	elicitation := urlClient["elicitation"].(map[string]any)
	if elicitation["mode"] != "url" || elicitation["available"] != false || elicitation["reason"] == "" {
		t.Fatal(elicitation)
	}
	formClient := callRaw(t, server.URL, "/", "run_api", args, nil, map[string]any{
		"io.modelcontextprotocol/clientCapabilities": map[string]any{"elicitation": map[string]any{"form": map[string]any{}}},
	})
	if _, ok := formClient["elicitation"]; ok || formClient["error"].(map[string]any)["code"] != "openany_mcp_credentials_missing" {
		t.Fatal("form elicitation was used for a secret", formClient)
	}
}

func callTool(t *testing.T, base, path, name string, args map[string]any, auth string) map[string]any {
	t.Helper()
	headers := map[string]string{}
	if auth != "" {
		headers["Authorization"] = auth
	}
	return callRaw(t, base, path, name, args, headers)
}

func callDirect(t *testing.T, base, path, name string, args map[string]any, auth string) map[string]any {
	t.Helper()
	headers := map[string]string{}
	if auth != "" {
		headers["Authorization"] = auth
	}
	return callRaw(t, base, path, name, args, headers)
}

func callRaw(t *testing.T, base, path, name string, args map[string]any, headers map[string]string, meta ...map[string]any) map[string]any {
	t.Helper()
	params := map[string]any{"name": name, "arguments": args}
	if len(meta) > 0 {
		params["_meta"] = meta[0]
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params})
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var out map[string]any
	if json.Unmarshal(body, &out) != nil || out["error"] != nil {
		t.Fatalf("%s %s", res.Status, body)
	}
	result, _ := out["result"].(map[string]any)
	content, _ := result["structuredContent"].(map[string]any)
	if content == nil {
		t.Fatal(body)
	}
	return content
}
