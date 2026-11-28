package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func inspectionModel(t *testing.T) map[string]any {
	t.Helper()
	v := candidateV2Fixture(t, "model")
	_, r, c := v2Parts(v)
	v["code"] = "openany_route_unavailable"
	r["execution_state"], r["charge_state"] = "not_started", "not_charged"
	r["primary_action"], r["replay_safety"] = "inspect_alternatives", "safe"
	c["compatibility"].(map[string]any)["level"] = "needs_inspection"
	c["compatibility"].(map[string]any)["basis"] = "catalog_inferred"
	c["compatibility"].(map[string]any)["missing_evidence"] = []any{"input_schema"}
	delete(c, "execution")
	delete(c, "method")
	delete(c, "public_path")
	return v
}

func TestSparseModelInspectionPreservesKnownAndQuotePrices(t *testing.T) {
	for _, quote := range []bool{false, true} {
		v := inspectionModel(t)
		_, _, c := v2Parts(v)
		if quote {
			c["pricing"] = map[string]any{"status": "requires_quote", "scope": "public_catalog", "requires_quote": true}
			delete(c, "currency")
			delete(c, "billing_unit")
		}
		e := publicFailure(v, 400, "sparse", true).Value["error"].(map[string]any)
		a, ok := e["alternatives"].([]any)
		if !ok || len(a) != 1 || a[0].(map[string]any)["execution"] != nil || e["recovery"].(map[string]any)["primary_action"] != "inspect_alternatives" {
			t.Fatalf("read-only model lost or promoted (quote=%t): %#v", quote, e)
		}
	}
}

func TestSparseModelInspectionRejectsExecutionAndUnsafeFacts(t *testing.T) {
	for _, change := range []string{"execution", "method", "path", "charge", "state", "identity"} {
		v := inspectionModel(t)
		_, r, c := v2Parts(v)
		switch change {
		case "execution":
			c["execution"] = map[string]any{}
		case "method":
			c["method"] = "POST"
		case "path":
			c["public_path"] = "/v1/chat/completions"
		case "charge":
			r["charge_state"] = "unknown"
		case "state":
			r["execution_state"] = "unknown"
		case "identity":
			c["model_id"] = "different-model"
		}
		if e := publicFailure(v, 400, "sparse", true).Value["error"].(map[string]any); e["alternatives"] != nil {
			t.Fatalf("accepted %s: %#v", change, e)
		}
	}
}

func TestRoutingRecoveryWithoutCandidatesUsesCorrectInput(t *testing.T) {
	v := map[string]any{"type": "openany_error", "code": "openany_route_unavailable", "details": map[string]any{"recovery": map[string]any{
		"version": json.Number("1"), "reason": "routing_options_unavailable", "execution_state": "not_started", "charge_state": "not_charged", "primary_action": "correct_input", "next_step_target": "https://openany.si/docs",
	}}}
	e := publicFailure(v, 400, "routing", true).Value["error"].(map[string]any)
	if e["recovery"] == nil || e["recovery"].(map[string]any)["primary_action"] != "correct_input" || e["execution_state"] != "not_started" {
		t.Fatalf("routing guidance lost: %#v", e)
	}
	if !strings.Contains(e["message"].(string), "route_mode") || !strings.Contains(e["message"].(string), "Provider") {
		t.Fatal("routing-specific guidance missing", e)
	}
}

func TestSparseModelDisabledOnlyHidesCandidatesNotKnownRoutingGuidance(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		v := inspectionModel(t)
		_, r, _ := v2Parts(v)
		if uncertain {
			r["execution_state"] = "unknown"
		}
		e := publicFailure(v, 400, "disabled", false).Value["error"].(map[string]any)
		if e["alternatives"] != nil {
			t.Fatal("disabled candidates exposed", e)
		}
		want := "correct_input"
		if uncertain {
			want = "check_original_request"
		}
		if e["recovery"].(map[string]any)["primary_action"] != want {
			t.Fatal("incorrect fallback", e)
		}
	}
}
