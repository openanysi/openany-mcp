package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/model"
)

func TestAuthorizationRawKeyBecomesBearerAndPlaceholdersStayLocal(t *testing.T) {
	var mu sync.Mutex
	var seenAuth, seenKey []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		seenKey = append(seenKey, r.Header.Get("X-Api-Key"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c, err := New(server.URL, time.Second, time.Second, 4096, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	request := model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list", RequestContentType: "none"}
	call := func(headers http.Header) Result {
		t.Helper()
		return c.Call(context.Background(), request, map[string]any{}, headers)
	}
	if res := call(http.Header{"Authorization": {"raw-key-value"}}); res.IsError {
		t.Fatal(res)
	}
	if res := call(http.Header{"Authorization": {"bearer live-key"}}); res.IsError {
		t.Fatal(res)
	}
	if res := call(http.Header{"Authorization": {"Bearer live-key"}}); res.IsError {
		t.Fatal(res)
	}
	if res := call(http.Header{"X-Api-Key": {"raw-key-value"}, "Xi-Api-Key": {"other-key"}}); res.IsError {
		t.Fatal(res)
	}
	mu.Lock()
	if len(seenAuth) != 4 || seenAuth[0] != "Bearer raw-key-value" || seenAuth[1] != "bearer live-key" || seenAuth[2] != "Bearer live-key" || seenAuth[3] != "" {
		t.Fatal(seenAuth)
	}
	if seenKey[3] != "raw-key-value" {
		t.Fatal(seenKey)
	}
	mu.Unlock()
	for _, value := range []string{
		"Bearer ${OPENANY_API_KEY}",
		"bearer ${env:OPENANY_API_KEY}",
		"Bearer $OPENANY_API_KEY",
		"${OPENANY_API_KEY}",
		"$OPENANY_API_KEY",
		"<key>",
		"<your-key>",
		"YOUR_API_KEY",
		"Bearer YOUR_OPENANY_KEY",
	} {
		before := len(seenAuth)
		res := call(http.Header{"Authorization": {value}})
		errObj, _ := res.Value["error"].(map[string]any)
		if !res.IsError || errObj["code"] != PlaceholderCredentialCode || errObj["execution_state"] != "not_started" || res.Value["key_url"] != KeyPageURL {
			t.Fatal(value, res)
		}
		encoded := errObj["message"].(string) + errObj["next_action"].(string)
		if strings.Contains(encoded, value) || strings.Contains(encoded, credentialMaterial("Authorization", value)) {
			t.Fatal("placeholder echoed", value)
		}
		mu.Lock()
		if len(seenAuth) != before {
			t.Fatal("placeholder reached the gateway", value, seenAuth)
		}
		mu.Unlock()
	}
	before := len(seenAuth)
	res := call(http.Header{"X-Api-Key": {"${OPENANY_API_KEY}"}})
	if !res.IsError || res.Value["error"].(map[string]any)["code"] != PlaceholderCredentialCode {
		t.Fatal(res)
	}
	mu.Lock()
	if len(seenAuth) != before {
		t.Fatal("x-api-key placeholder was forwarded")
	}
	mu.Unlock()
}
