package gateway

import (
	"reflect"
	"testing"
)

func TestReviewV2ExecutableCandidatesRequireCompleteVerifiedEvidence(t *testing.T) {
	for _, tc := range []struct{ fixture, level string }{
		{"direct", "direct"}, {"requires_adaptation", "requires_adaptation"},
		{"model", "direct"}, {"model", "requires_adaptation"}, {"model", "similar"},
	} {
		t.Run(tc.fixture+"/"+tc.level, func(t *testing.T) {
			makeFixture := func() (map[string]any, map[string]any) {
				v := candidateV2Fixture(t, tc.fixture)
				_, _, c := v2Parts(v)
				k := c["compatibility"].(map[string]any)
				k["level"] = tc.level
				if tc.fixture == "model" && tc.level != "similar" {
					c["relation"] = "same_model"
				}
				return v, k
			}
			v, _ := makeFixture()
			if e := publicFailure(v, 503, "verified", true).Value["error"].(map[string]any); e["alternatives"] == nil {
				t.Fatalf("valid verified candidate rejected: %#v", e)
			}
			for _, evidence := range []string{"catalog_inferred", "read_only", "output_contract", "pricing"} {
				t.Run(evidence, func(t *testing.T) {
					v, k := makeFixture()
					if evidence == "catalog_inferred" {
						k["basis"] = evidence
					} else {
						k["missing_evidence"] = []any{evidence}
					}
					original := map[string]any{"basis": k["basis"], "missing_evidence": k["missing_evidence"]}
					r := publicFailure(v, 503, "incomplete", true)
					e := r.Value["error"].(map[string]any)
					recovery := e["recovery"].(map[string]any)
					if e["alternatives"] != nil || e["alternatives_meta"] != nil || recovery["primary_action"] != "check_original_request" || recovery["next_step_target"] != "https://openany.si/dashboard?page=usage" || recovery["alternatives_status"] != "none" {
						t.Fatalf("incomplete evidence advertised execution or dangling action: %#v", e)
					}
					if e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || r.Value["request_id"] != "incomplete" || e["code"] != v["code"] {
						t.Fatalf("core failure facts changed: %#v", r.Value)
					}
					if !reflect.DeepEqual(original, map[string]any{"basis": k["basis"], "missing_evidence": k["missing_evidence"]}) {
						t.Fatal("parser filled in evidence instead of rejecting candidate")
					}
				})
			}
		})
	}
}

func TestReviewV2InvalidHighEvidenceDowngradesMixedCandidates(t *testing.T) {
	for _, evidence := range []string{"catalog_inferred", "read_only", "output_contract", "pricing"} {
		t.Run(evidence, func(t *testing.T) {
			v := candidateV2Fixture(t, "mixed")
			_, _, c := v2Parts(v)
			k := c["compatibility"].(map[string]any)
			if evidence == "catalog_inferred" {
				k["basis"] = evidence
			} else {
				k["missing_evidence"] = []any{evidence}
			}
			e := publicFailure(v, 503, "mixed", true).Value["error"].(map[string]any)
			a, ok := e["alternatives"].([]any)
			if !ok || len(a) != 1 {
				t.Fatalf("did not retain only the valid inspection candidate: %#v", e)
			}
			candidate := a[0].(map[string]any)
			r := e["recovery"].(map[string]any)
			if candidate["compatibility"].(map[string]any)["level"] != "needs_inspection" || candidate["execution"] != nil || r["primary_action"] != "inspect_related_apis" || r["next_step_target"] != candidate["definition_path"] {
				t.Fatalf("invalid high grade was upgraded or target was not recomputed: %#v", e)
			}
		})
	}
}

func TestReviewV2MappingChecksEveryAuthenticationPathComponent(t *testing.T) {
	for _, tc := range []struct {
		reference string
		want      bool
	}{
		{"body:/auth/id", false}, {"body:credentials/id", false},
		{"body:/payload/auth/id", false}, {"body:/payload/credentials/id", false},
		{"body:/auth/token_address", false}, {"body:/credentials/token_symbol", false},
		{"body:/payload/token_address", true}, {"body:/payload/token_symbol", true},
		{"query:token_address", true}, {"query:token_symbol", true}, {"query:keyword", true},
	} {
		for _, direction := range []string{"from", "to"} {
			t.Run(direction+"/"+tc.reference, func(t *testing.T) {
				v := candidateV2Fixture(t, "requires_adaptation")
				_, _, c := v2Parts(v)
				mapping := map[string]any{"from": "query:keyword", "to": "query:keyword"}
				mapping[direction] = tc.reference
				c["compatibility"].(map[string]any)["input_mapping"] = []any{mapping}
				e := publicFailure(v, 503, "mapping", true).Value["error"].(map[string]any)
				if (e["alternatives"] != nil) != tc.want {
					t.Fatalf("mapping path %s accepted=%v want=%v", tc.reference, e["alternatives"] != nil, tc.want)
				}
			})
		}
	}
}
