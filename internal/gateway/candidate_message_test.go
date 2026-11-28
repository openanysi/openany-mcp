package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCandidateMessageUsesValidatedFieldsNotUpstreamMessage(t *testing.T) {
	for _, grade := range []string{"model", "verified_model", "direct", "requires_adaptation", "needs_inspection", "mixed"} {
		t.Run(grade, func(t *testing.T) {
			fixture := grade
			if fixture == "verified_model" {
				fixture = "model"
			}
			v := candidateV2Fixture(t, fixture)
			if grade == "model" {
				v = inspectionModel(t)
			}
			v["message"] = "UNTRUSTED_UPSTREAM_INSTRUCTION"
			e := publicFailure(v, 503, "message-test", true).Value["error"].(map[string]any)
			text := e["message"].(string)
			for _, raw := range e["alternatives"].([]any) {
				c := raw.(map[string]any)
				label := c["name"].(string)
				if c["kind"] == "model" {
					label = c["model_id"].(string)
				}
				if !strings.Contains(text, label) || !strings.Contains(text, c["provider_id"].(string)) || !strings.Contains(text, "GET "+c["definition_path"].(string)) {
					t.Fatalf("missing candidate or next step: %s", text)
				}
			}
			if strings.Contains(text, "UNTRUSTED_UPSTREAM_INSTRUCTION") {
				t.Fatal(text)
			}
		})
	}
}

func TestCandidateMessageListsOnlyPublishedCandidatesAfterFiltering(t *testing.T) {
	v := candidateV2Fixture(t, "mixed")
	_, _, rejected := v2Parts(v)
	rejected["provider_id"] = "hidden-provider"
	rejected["execution"].(map[string]any)["path"] = "https://evil.example/run"
	e := publicFailure(v, 503, "filtered", true).Value["error"].(map[string]any)
	text := e["message"].(string)
	if strings.Contains(text, "hidden-provider") || strings.Contains(text, "evil.example") {
		t.Fatal(text)
	}
	for _, raw := range e["alternatives"].([]any) {
		c := raw.(map[string]any)
		if !strings.Contains(text, c["provider_id"].(string)) || !strings.Contains(text, c["definition_path"].(string)) {
			t.Fatal(text)
		}
	}
}

func TestCandidateMessageDisclosesDifferentModelAndQuotesNames(t *testing.T) {
	v := inspectionModel(t)
	_, _, c := v2Parts(v)
	c["relation"] = "similar_model"
	c["compatibility"].(map[string]any)["differences"] = []any{map[string]any{"code": "model_changed", "message": "Different model."}}
	e := publicFailure(v, 400, "similar", true).Value["error"].(map[string]any)
	text := e["message"].(string)
	if !strings.Contains(text, "different model") || !strings.Contains(text, "(read-only)") || strings.Contains(text, "run_api") {
		t.Fatal(text)
	}
	v = candidateV2Fixture(t, "direct")
	_, _, c = v2Parts(v)
	c["name"] = "Company \"Register\" lookup"
	encoded, _ := json.Marshal(c["name"])
	e = publicFailure(v, 503, "quoted", true).Value["error"].(map[string]any)
	if !strings.Contains(e["message"].(string), string(encoded)) {
		t.Fatal(e)
	}
}

func TestCandidateMessageCannotLeakDisabledRejectedOrUncertainCandidates(t *testing.T) {
	for _, scenario := range []string{"disabled", "identity", "charge", "state"} {
		t.Run(scenario, func(t *testing.T) {
			v := inspectionModel(t)
			_, r, c := v2Parts(v)
			c["provider_id"] = "hidden-provider"
			switch scenario {
			case "identity":
				c["model_id"] = "invalid-hidden-model"
			case "charge":
				r["charge_state"] = "unknown"
			case "state":
				r["execution_state"] = "unknown"
			}
			e := publicFailure(v, 400, "message-test", scenario != "disabled").Value["error"].(map[string]any)
			if e["alternatives"] != nil || strings.Contains(e["message"].(string), "hidden-provider") {
				t.Fatal(e)
			}
		})
	}
}
