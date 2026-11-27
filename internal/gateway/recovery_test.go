package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/model"
)

func recoveryFixture() map[string]any {
	return map[string]any{"version": json.Number("1"), "reason": "insufficient_balance", "execution_state": "not_started", "charge_state": "not_charged", "primary_action": "top_up", "next_step_target": "https://openany.si/dashboard?page=credits", "recovery_id": "req-1"}
}

func TestRecoveryPreservesStateAndDropsUnknownFields(t *testing.T) {
	recovery := recoveryFixture()
	recovery["private_key"] = "must-not-leak"
	obj := map[string]any{"type": "openany_error", "code": "openany_insufficient_balance", "message": "untrusted raw message", "openany_cost": json.Number("0"), "details": map[string]any{"recovery": recovery, "private": "not public"}}
	r := publicFailure(obj, 402, "req-1")
	e := r.Value["error"].(map[string]any)
	if !r.IsError || e["execution_state"] != "not_started" || r.Value["openany_cost"] != json.Number("0") {
		t.Fatalf("lost state: %#v", r)
	}
	safe := e["recovery"].(map[string]any)
	if safe["private_key"] != nil || e["details"] != nil || e["message"] == "untrusted raw message" {
		t.Fatalf("unsafe fields: %#v", e)
	}
}

func TestRecoveryRejectsUnknownVersionAndUntrustedLinks(t *testing.T) {
	for _, change := range []func(map[string]any){func(r map[string]any) { r["version"] = json.Number("2") }, func(r map[string]any) { r["next_step_target"] = "https://evil.example/steal" }, func(r map[string]any) { r["execution_state"] = "invented" }} {
		rec := recoveryFixture()
		change(rec)
		r := publicFailure(map[string]any{"code": "openany_insufficient_balance", "details": map[string]any{"recovery": rec}}, 402, "req")
		e := r.Value["error"].(map[string]any)
		if e["recovery"] != nil || e["execution_state"] != "reported_failure" {
			t.Fatalf("accepted invalid recovery: %#v", r)
		}
	}
}

func TestRecoveryZeroChargeDoesNotImplyUnstarted(t *testing.T) {
	r := publicFailure(map[string]any{"code": "openany_upstream_timeout", "openany_cost": json.Number("0")}, 504, "req")
	if r.Value["error"].(map[string]any)["execution_state"] != "reported_failure" {
		t.Fatal("charge changed execution state")
	}
}

func TestRecoveryRejectsUpstreamSpoofAndMismatchedAction(t *testing.T) {
	for _, code := range []string{"openany_upstream_timeout", "openany_spend_limit_exceeded", "openany_unrecognized_error"} {
		r := publicFailure(map[string]any{"code": code, "details": map[string]any{"recovery": recoveryFixture()}}, 502, "req")
		if e := r.Value["error"].(map[string]any); e["recovery"] != nil || e["execution_state"] != "reported_failure" {
			t.Fatalf("trusted inconsistent %s: %#v", code, e)
		}
	}
	for _, code := range []string{"openany_upstream_timeout", "openany_insufficient_balance"} {
		rec := recoveryFixture()
		rec["execution_state"] = "unknown"
		r := publicFailure(map[string]any{"code": code, "details": map[string]any{"recovery": rec}}, 502, "req")
		if r.Value["error"].(map[string]any)["recovery"] != nil {
			t.Fatal("uncertain request offered a purchase action")
		}
	}
}

func TestRecoveryHTTPRoundTripPreservesStateAndIdentity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-roundtrip")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "openany_error", "code": "openany_insufficient_balance", "openany_cost": 0, "details": map[string]any{"recovery": recoveryFixture()}})
	}))
	defer s.Close()
	c, err := New(s.URL, time.Second, time.Second, 4096, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	r := c.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/test"}, nil, nil)
	e := r.Value["error"].(map[string]any)
	if r.Value["http_status"] != 402 || r.Value["request_id"] != "req-roundtrip" || r.Value["openany_cost"] != json.Number("0") || e["execution_state"] != "not_started" || e["charge_state"] != "not_charged" || e["recovery"].(map[string]any)["primary_action"] != "top_up" {
		t.Fatalf("wire recovery changed: %#v", r.Value)
	}
}

func TestRecoveryOutputLimitKeepsChargeAndOriginalFailure(t *testing.T) {
	c, _ := New("https://openany.si", time.Second, time.Second, 4096, 8192)
	defer c.Close()
	r := publicFailure(map[string]any{"code": "openany_insufficient_balance", "openany_cost": json.Number("0"), "details": map[string]any{"recovery": recoveryFixture()}}, 402, "req-limit")
	r.Value["presentation"] = strings.Repeat("x", 20000)
	r = c.LimitResult(r)
	e := r.Value["error"].(map[string]any)
	if e["code"] != "openany_mcp_result_too_large" || e["original_code"] != "openany_insufficient_balance" || e["charge_state"] != "not_charged" || e["recovery"] == nil || r.Value["openany_cost"] != json.Number("0") || r.Value["request_id"] != "req-limit" {
		t.Fatalf("output limit discarded recovery or charge: %#v", r.Value)
	}
}

func TestRecoverySharedContractExamples(t *testing.T) {
	b, err := os.ReadFile("../account/testdata/onboarding-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples struct {
		Recoveries []struct {
			Code       string         `json:"code"`
			HTTPStatus int            `json:"httpStatus"`
			Recovery   map[string]any `json:"recovery"`
		} `json:"recoveries"`
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	if err = d.Decode(&examples); err != nil {
		t.Fatal(err)
	}
	for _, row := range examples.Recoveries {
		r := publicFailure(map[string]any{"code": row.Code, "details": map[string]any{"recovery": row.Recovery}}, row.HTTPStatus, "example-request")
		e := r.Value["error"].(map[string]any)
		if e["recovery"] == nil || e["execution_state"] != row.Recovery["execution_state"] || e["charge_state"] != row.Recovery["charge_state"] {
			t.Fatalf("contract rejected %s: %#v", row.Code, e)
		}
	}
}
