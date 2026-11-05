package gateway

import (
	"context"
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOptionalArrayWithoutPolicyFailsBeforeGatewayDispatch(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits.Add(1)
		if req.URL.RawQuery != "" || req.Header.Get("Authorization") != "Bearer current-caller" {
			t.Error("unexpected dispatch", req.URL.RawQuery, req.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c, _ := New(server.URL, time.Second, time.Second, 1024, 4096)
	defer c.Close()
	r := model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list", RequestContentType: "none", Inputs: []model.Parameter{{Name: "filters", Location: "query", OmitNull: true}}}
	for _, args := range []map[string]any{{"filters": []any{}}, {"filters": []any{"a"}}} {
		res := c.Call(context.Background(), r, args, nil)
		err := res.Value["error"].(map[string]any)
		if !res.IsError || err["code"] != "openany_mcp_array_policy_missing" || err["stage"] != "validate" || err["execution_state"] != "not_started" || !strings.Contains(err["message"].(string), "filters") || hits.Load() != 0 {
			t.Fatal("unsupported array was dispatched or error was hidden", res, hits.Load())
		}
	}
	for _, args := range []map[string]any{{}, {"filters": nil}} {
		if res := c.Call(context.Background(), r, args, http.Header{"Authorization": {"Bearer current-caller"}}); res.IsError {
			t.Fatal("omitted or null optional array was blocked", res)
		}
	}
	if hits.Load() != 2 {
		t.Fatal("requests were lost or repeated", hits.Load())
	}
}

func TestCallPreservesCredentialsAndPublicErrors(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.Header.Get("Authorization")]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "gateway-123")
		w.Write([]byte(`   {"type":"openany_error","code":"openany_authentication_invalid","message":"invalid"}`))
	}))
	defer server.Close()
	c, err := New(server.URL, time.Second, time.Second, 1024, 4096)
	if err != nil {
		t.Fatal(err)
	}
	r := model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list", RequestContentType: "none"}
	var wg sync.WaitGroup
	for _, key := range []string{"Bearer A", "Bearer B"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			res := c.Call(context.Background(), r, map[string]any{}, http.Header{"Authorization": {key}})
			if !res.IsError || res.Value["request_id"] != "gateway-123" {
				t.Error(res)
			}
		}(key)
	}
	wg.Wait()
	if seen["Bearer A"] != 1 || seen["Bearer B"] != 1 || len(seen) != 2 {
		t.Fatal(seen)
	}
}
func TestRedirectIsNotFollowed(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; http.Redirect(w, r, "/again", 302) }))
	defer s.Close()
	c, _ := New(s.URL, time.Second, time.Second, 1024, 4096)
	res := c.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list"}, nil, nil)
	if hits != 1 || !res.IsError {
		t.Fatalf("hits=%d %+v", hits, res)
	}
}
func TestResultLimitIncludesEscapedTextCopy(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"value": strings.Repeat("\\", 200)})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer server.Close()
	c, _ := New(server.URL, time.Second, time.Second, 4096, 2200)
	result := c.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list"}, nil, nil)
	if !result.IsError {
		t.Fatal("escaped MCP text copy exceeded the result limit")
	}
}
func TestRejectsNonJSONBeforeWaitingForStreamEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	c, _ := New(server.URL, 50*time.Millisecond, time.Second, 1024, 4096)
	r := c.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list"}, nil, nil)
	err := r.Value["error"].(map[string]any)
	if err["code"] != "openany_mcp_unsupported_response" {
		t.Fatalf("waited for an unsupported stream: %+v", r)
	}
}
func TestTransportFailureDoesNotReplayAndIsUnknown(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer server.Close()
	c, _ := New(server.URL, time.Second, time.Second, 1024, 4096)
	res := c.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list"}, nil, nil)
	if calls.Load() != 1 || !res.IsError || res.Value["error"].(map[string]any)["execution_state"] != "unknown" {
		t.Fatalf("replayed or misclassified: %d %+v", calls.Load(), res)
	}
}

func TestUnsupportedResponseNeverDispatches(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c, _ := New(server.URL, time.Second, time.Second, 1024, 4096)
	result := c.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/stream", UnsupportedResponse: true}, nil, nil)
	if calls.Load() != 0 || !result.IsError || result.Value["error"].(map[string]any)["code"] != "openany_mcp_unsupported_response_mode" {
		t.Fatal(calls.Load(), result)
	}
}
