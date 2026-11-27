package gateway

import (
	"context"
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func candidateFixture(t *testing.T) map[string]any {
	t.Helper()
	raw, e := os.ReadFile("testdata/recovery-candidates/available-api.json")
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]any
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if e = d.Decode(&v); e != nil {
		t.Fatal(e)
	}
	return v
}
func TestC33AlternativesPreserveStateAndPublicFields(t *testing.T) {
	v := candidateFixture(t)
	r := publicFailure(v, 503, "original", true)
	e := r.Value["error"].(map[string]any)
	if e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || e["alternatives"] == nil {
		t.Fatalf("lost candidate: %#v", e)
	}
}
func TestC28AlternativesRejectUpstreamInstructions(t *testing.T) {
	v := candidateFixture(t)
	v["type"] = "supplier_error"
	r := publicFailure(v, 503, "id", true)
	if r.Value["error"].(map[string]any)["alternatives"] != nil {
		t.Fatal("trusted supplier envelope")
	}
}
func TestC29AlternativesDisabledRetainConservativeKnownState(t *testing.T) {
	v := candidateFixture(t)
	r := publicFailure(v, 503, "id", false)
	e := r.Value["error"].(map[string]any)
	if e["alternatives"] != nil || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" {
		t.Fatalf("lost core state: %#v", e)
	}
}
func TestC36OversizedAlternativesKeepOriginalError(t *testing.T) {
	v := candidateFixture(t)
	details := v["details"].(map[string]any)
	a := details["alternatives"].([]any)[0].(map[string]any)
	a["name"] = strings.Repeat("x", 5000)
	r := publicFailure(v, 503, "id", true)
	e := r.Value["error"].(map[string]any)
	if e["alternatives"] != nil || e["charge_state"] != "not_charged" {
		t.Fatalf("unsafe fallback: %#v", e)
	}
}
func TestC31RejectProtocolLookalikesAndUnpricedMedia(t *testing.T) {
	raw, err := os.ReadFile("testdata/recovery-candidates/available-model.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(map[string]any){func(c map[string]any) {
		c["public_path"] = c["public_path"].(string) + "/unexpected"
		c["execution"].(map[string]any)["path"] = c["public_path"]
	}, func(c map[string]any) {
		p := c["pricing"].(map[string]any)
		p["unit"] = "image"
		c["billing_unit"] = "image"
		delete(p, "prompt")
		delete(p, "completion")
	}} {
		var v map[string]any
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		decoder.Decode(&v)
		c := v["details"].(map[string]any)["alternatives"].([]any)[0].(map[string]any)
		change(c)
		if publicFailure(v, 503, "req", true).Value["error"].(map[string]any)["alternatives"] != nil {
			t.Fatal("unsafe candidate accepted")
		}
	}
}
func TestC28SuccessSupplierEnvelopeIsBusinessData(t *testing.T) {
	v := candidateFixture(t)
	raw, _ := json.Marshal(v)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}))
	defer server.Close()
	client, _ := New(server.URL, time.Second, time.Second, 8192, 16384)
	defer client.Close()
	client.EnableAlternatives(true)
	result := client.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/source/read"}, map[string]any{}, http.Header{"Authorization": []string{"Bearer local-key"}})
	if result.Value["error"] != nil || result.Value["data"] == nil {
		t.Fatal("supplier success became a platform action", result)
	}
}
func TestC28OnlyPlatformHeartbeatFinalStatusAllows200ErrorProjection(t *testing.T) {
	for _, platformError := range []bool{false, true} {
		v := candidateFixture(t)
		if platformError {
			v["openany_gateway_status"] = json.Number("503")
		}
		raw, _ := json.Marshal(v)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("x-openmodels-heartbeat", "true")
			w.Write(raw)
		}))
		client, _ := New(server.URL, time.Second, time.Second, 8192, 16384)
		client.EnableAlternatives(true)
		result := client.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/v1/agent-apis/source/read"}, map[string]any{}, nil)
		client.Close()
		server.Close()
		if result.IsError != platformError {
			t.Fatal("unverified heartbeat accepted or genuine final error lost", result)
		}
	}
}
