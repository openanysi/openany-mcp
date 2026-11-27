package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const discoveryFixture = `{"version":1,"model_id":"gpt-5.2","source_state":"unavailable","read_only":true,"related_models":[{"model_id":"chat-live","provider_id":"live-provider","relation":"similar_model","protocol":"openai-chat-completions","pricing":{"status":"known","scope":"public_catalog","currency":"USD","unit":"token","prompt":"0.000001","completion":"0.000002","requires_quote":false}}]}`

func TestModelDiscoveryRejectsInvalidOrUnsafeMetadata(t *testing.T) {
	for _, test := range []struct{ name, old, new string }{
		{"wrong-version", `"version":1`, `"version":2`},
		{"wrong-source", `"model_id":"gpt-5.2"`, `"model_id":"other"`},
		{"not-read-only", `"read_only":true`, `"read_only":false`},
		{"source-available", `"source_state":"unavailable"`, `"source_state":"available"`},
		{"unknown-source", `"source_state":"unavailable"`, `"source_state":"unknown"`},
		{"bad-state", `"source_state":"unavailable"`, `"source_state":"garbage"`},
		{"same-source", `"model_id":"chat-live"`, `"model_id":"gpt-5.2"`},
		{"bad-provider", `"provider_id":"live-provider"`, `"provider_id":"https://example.com"`},
		{"image-protocol", `"protocol":"openai-chat-completions"`, `"protocol":"images"`},
		{"unknown-price", `"status":"known"`, `"status":"unknown"`},
		{"negative-price", `"prompt":"0.000001"`, `"prompt":"-1"`},
		{"requires-quote", `"requires_quote":false`, `"requires_quote":true`},
		{"private-scope", `"scope":"public_catalog"`, `"scope":"internal"`},
		{"currency", `"currency":"USD"`, `"currency":"CNY"`},
		{"execution-field", `"protocol":"openai-chat-completions"`, `"protocol":"openai-chat-completions","execution":{"method":"POST"}`},
		{"duplicate-key", `"read_only":true`, `"read_only":false,"read_only":true`},
		{"trailing-json", `}]}`, `}] } {}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(strings.Replace(discoveryFixture, test.old, test.new, 1)))
			}))
			defer server.Close()
			client, _ := New(server.URL, time.Second, time.Second, 32768, 131072)
			defer client.Close()
			if _, err := client.ModelRecovery(context.Background(), "gpt-5.2"); err == nil {
				t.Fatal("unsafe metadata accepted")
			}
			if calls.Load() != 1 {
				t.Fatal("retried metadata read")
			}
		})
	}
}

func TestModelDiscoveryBoundedKeylessReadAndEncodedModelID(t *testing.T) {
	id := "vendor/model&input=private"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/models/recovery" || len(r.URL.Query()) != 1 || r.URL.Query().Get("model") != id || r.ContentLength != 0 || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("unsafe metadata request", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ModelDiscovery{Version: 1, ModelID: id, SourceState: "unknown", ReadOnly: true, RelatedModels: []RelatedModel{}})
	}))
	defer server.Close()
	client, _ := New(server.URL, time.Second, time.Second, 32768, 131072)
	defer client.Close()
	if _, err := client.ModelRecovery(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}

func TestModelDiscoveryDependencyErrorsDoNotRedirectOrRetry(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, test := range []string{"not-found", "server-error", "redirect", "too-large", "timeout", "invalid-json", "wrong-content-type", "too-many", "duplicate-model"} {
		t.Run(test, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch test {
				case "not-found":
					w.WriteHeader(404)
				case "server-error":
					w.WriteHeader(503)
				case "redirect":
					http.Redirect(w, r, target.URL, 302)
				case "too-large":
					w.Write([]byte(strings.Repeat(" ", 16*1024+1)))
				case "timeout":
					time.Sleep(60 * time.Millisecond)
					w.Write([]byte(discoveryFixture))
				case "invalid-json":
					w.Write([]byte(`{"version":`))
				case "wrong-content-type":
					w.Header().Set("Content-Type", "text/html")
					w.Write([]byte(discoveryFixture))
				case "too-many", "duplicate-model":
					var d ModelDiscovery
					json.Unmarshal([]byte(discoveryFixture), &d)
					count := 2
					if test == "too-many" {
						count = 4
					}
					for len(d.RelatedModels) < count {
						d.RelatedModels = append(d.RelatedModels, d.RelatedModels[0])
					}
					json.NewEncoder(w).Encode(d)
				}
			}))
			defer server.Close()
			timeout := time.Second
			if test == "timeout" {
				timeout = 10 * time.Millisecond
			}
			client, _ := New(server.URL, timeout, time.Second, 32768, 131072)
			defer client.Close()
			if _, err := client.ModelRecovery(context.Background(), "gpt-5.2"); err == nil {
				t.Fatal("dependency error accepted")
			}
			if calls.Load() != 1 {
				t.Fatal("read retried")
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("followed redirect")
	}
}

func TestModelDiscoveryInvalidIDNeverLeavesClient(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	client, _ := New(server.URL, time.Second, time.Second, 32768, 131072)
	defer client.Close()
	for _, id := range []string{"", " padded", strings.Repeat("x", 65), "line\nbreak"} {
		if _, err := client.ModelRecovery(context.Background(), id); err == nil {
			t.Fatal("invalid id accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request sent")
	}
}
